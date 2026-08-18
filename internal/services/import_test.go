package services

import (
	"strings"
	"testing"

	"comptoir/internal/models"
)

// prod abrège le type des produits dans les prédicats de recherche.
type prod = models.Product

func TestParseSheetMoney(t *testing.T) {
	cases := []struct {
		raw      string
		decimals int
		attendu  int64
		erreur   bool
	}{
		{"1500", 0, 150000, false},
		{"1 500", 0, 150000, false},
		{"1 500 FCFA", 0, 150000, false},
		{"1.500", 0, 150000, false},   // point de milliers, monnaie sans décimale
		{"1500,50", 2, 150050, false}, // virgule décimale
		{"1500.50", 2, 150050, false}, // point décimal
		{"1 234 567", 0, 123456700, false},
		{"", 0, 0, false}, // colonne vide : montant nul, pas une erreur
		{"-250", 0, -25000, false},
		{"gratuit", 0, 0, true},
	}
	for _, c := range cases {
		got, err := parseSheetMoney(c.raw, c.decimals)
		if c.erreur {
			if err == nil {
				t.Errorf("parseSheetMoney(%q) aurait dû échouer", c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSheetMoney(%q) : %v", c.raw, err)
			continue
		}
		if got != c.attendu {
			t.Errorf("parseSheetMoney(%q, %d) = %d, attendu %d", c.raw, c.decimals, got, c.attendu)
		}
	}
}

func TestMapColumns_IntitulesLibres(t *testing.T) {
	header := []string{"Désignation", "RÉFÉRENCE", "Prix de vente HT", "Stock initial", "Colonne inconnue"}
	columns, ignored := mapColumns(header)

	for _, field := range []string{"name", "sku", "sale", "quantity"} {
		if _, ok := columns[field]; !ok {
			t.Errorf("colonne %q non reconnue dans %v", field, header)
		}
	}
	if len(ignored) != 1 || ignored[0] != "Colonne inconnue" {
		t.Errorf("colonnes ignorées = %v, attendu [Colonne inconnue]", ignored)
	}
}

func TestParseSheet_DevineLeSeparateur(t *testing.T) {
	for _, content := range []string{
		"Désignation;Prix\nClavier;5000\n",
		"Désignation,Prix\nClavier,5000\n",
		"Désignation\tPrix\nClavier\t5000\n",
	} {
		records, header, err := parseSheet(content)
		if err != nil {
			t.Fatalf("parseSheet : %v", err)
		}
		if len(header) != 2 {
			t.Errorf("%d colonne(s) détectée(s), attendu 2 — séparateur mal deviné", len(header))
		}
		if len(records) != 1 || records[0][0] != "Clavier" {
			t.Errorf("lignes = %v", records)
		}
	}
}

func TestParseSheet_Refus(t *testing.T) {
	if _, _, err := parseSheet("   "); err == nil {
		t.Error("un fichier vide aurait dû être refusé")
	}
	if _, _, err := parseSheet("Désignation;Prix\n"); err == nil {
		t.Error("un fichier sans article aurait dû être refusé")
	}
}

func TestImportProducts_ApercuPuisApplication(t *testing.T) {
	s := newSuite(t)
	content := strings.Join([]string{
		"Désignation;Référence;Catégorie;Prix de vente;Coût d'achat;Stock;Seuil",
		"Clavier mécanique;CLA-001;Accessoires;25 000;15 000;12;3",
		"Souris sans fil;SOU-001;Accessoires;8 000;4 500;30;5",
		"Écran 24 pouces;ECR-024;Écrans;95 000;70 000;4;1",
		"",
	}, "\n")

	// 1. L'aperçu ne touche à rien.
	preview, err := s.catalog.ImportProducts(content, false)
	if err != nil {
		t.Fatalf("aperçu : %v", err)
	}
	if preview.Applied {
		t.Error("l'aperçu se déclare appliqué")
	}
	if preview.Created != 3 || preview.Updated != 0 || preview.Skipped != 0 {
		t.Errorf("aperçu : %d créations, %d mises à jour, %d écartées — attendu 3/0/0",
			preview.Created, preview.Updated, preview.Skipped)
	}
	if n := s.db.Products.Count(); n != 0 {
		t.Fatalf("%d produit(s) créé(s) par un simple aperçu", n)
	}
	if n := s.db.Categories.Count(); n != 0 {
		t.Fatalf("%d catégorie(s) créée(s) par un simple aperçu", n)
	}
	if len(preview.CategoriesCreated) != 2 {
		t.Errorf("catégories annoncées = %v, attendu 2", preview.CategoriesCreated)
	}

	// 2. L'application produit exactement ce que l'aperçu annonçait.
	report, err := s.catalog.ImportProducts(content, true)
	if err != nil {
		t.Fatalf("application : %v", err)
	}
	if report.Created != preview.Created || report.Skipped != preview.Skipped {
		t.Errorf("l'application diverge de l'aperçu : %+v contre %+v", report, preview)
	}
	if n := s.db.Products.Count(); n != 3 {
		t.Fatalf("%d produit(s), attendu 3", n)
	}
	if n := s.db.Categories.Count(); n != 2 {
		t.Errorf("%d catégorie(s), attendu 2", n)
	}

	clavier, err := s.db.Products.FindOne(func(p prod) bool { return p.SKU == "CLA-001" })
	if err != nil {
		t.Fatal("le clavier n'a pas été importé")
	}
	if clavier.SalePrice != 2500000 || clavier.PurchasePrice != 1500000 {
		t.Errorf("prix = %d / %d, attendu 2500000 / 1500000", clavier.SalePrice, clavier.PurchasePrice)
	}
	if clavier.Quantity != 12 {
		t.Errorf("stock = %d, attendu 12", clavier.Quantity)
	}
	if clavier.MinStock != 3 {
		t.Errorf("seuil = %d, attendu 3", clavier.MinStock)
	}
	// Le stock importé passe par un mouvement : l'historique reste complet.
	movements := s.db.Movements.Find(func(m mv) bool { return m.ProductID == clavier.ID })
	if len(movements) != 1 {
		t.Errorf("%d mouvement(s) pour le stock initial, attendu 1", len(movements))
	}
}

func TestImportProducts_MiseAJourNeTouchePasAuStockNiAuCout(t *testing.T) {
	s := newSuite(t)
	p := s.product("Article existant", 500000, 900000, 20)
	// On lui donne une référence connue du fichier.
	p.SKU = "REF-001"
	if err := s.db.Products.Update(p); err != nil {
		t.Fatal(err)
	}

	content := "Désignation;Référence;Prix de vente;Coût d'achat;Stock\n" +
		"Article renommé;REF-001;1 200;99 999;999\n"

	report, err := s.catalog.ImportProducts(content, true)
	if err != nil {
		t.Fatalf("import : %v", err)
	}
	if report.Updated != 1 || report.Created != 0 {
		t.Fatalf("%d mise(s) à jour, %d création(s) — attendu 1/0", report.Updated, report.Created)
	}

	after := s.reload(p.ID)
	if after.Name != "Article renommé" {
		t.Errorf("nom = %q, la mise à jour n'a pas eu lieu", after.Name)
	}
	if after.SalePrice != 120000 {
		t.Errorf("prix de vente = %d, attendu 120000", after.SalePrice)
	}
	if after.Quantity != 20 {
		t.Errorf("stock = %d, attendu 20 : un import ne modifie pas un stock existant", after.Quantity)
	}
	if after.PurchasePrice != 500000 {
		t.Errorf("coût moyen = %d, attendu 500000 : il découle des réceptions, pas d'un fichier", after.PurchasePrice)
	}
}

func TestImportProducts_LignesInvalides(t *testing.T) {
	s := newSuite(t)
	content := strings.Join([]string{
		"Désignation;Référence;Prix de vente",
		";SANS-NOM;1000",               // désignation absente
		"Prix illisible;PRIX-X;offert", // montant illisible
		"Doublon;DUP-1;1000",
		"Doublon bis;DUP-1;2000", // référence déjà vue
		"Correct;OK-1;3000",
		"",
	}, "\n")

	report, err := s.catalog.ImportProducts(content, true)
	if err != nil {
		t.Fatalf("import : %v", err)
	}
	if report.Created != 2 {
		t.Errorf("%d création(s), attendu 2", report.Created)
	}
	if report.Skipped != 3 {
		t.Errorf("%d ligne(s) écartée(s), attendu 3", report.Skipped)
	}
	// Chaque ligne écartée doit dire pourquoi.
	for _, row := range report.Rows {
		if row.Action == ImportSkip && strings.TrimSpace(row.Message) == "" {
			t.Errorf("ligne %d écartée sans explication", row.Line)
		}
	}
}

func TestImportProducts_SansColonneDesignation(t *testing.T) {
	s := newSuite(t)
	_, err := s.catalog.ImportProducts("Truc;Machin\n1;2\n", false)
	if err == nil {
		t.Fatal("un fichier sans colonne de désignation aurait dû être refusé")
	}
	if !strings.Contains(err.Error(), "désignation") {
		t.Errorf("message peu explicite : %v", err)
	}
}

func TestImportTemplate(t *testing.T) {
	s := newSuite(t)
	f, err := s.catalog.ImportTemplate()
	if err != nil {
		t.Fatalf("modèle : %v", err)
	}
	// Le modèle doit être relisible par l'import lui-même.
	report, err := s.catalog.ImportProducts(string(f.Content), false)
	if err != nil {
		t.Fatalf("le modèle produit n'est pas relisible : %v", err)
	}
	if report.Created != 2 || report.Skipped != 0 {
		t.Errorf("modèle : %d création(s), %d écartée(s) — attendu 2/0", report.Created, report.Skipped)
	}
}
