package storage

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"comptoir/internal/models"
)

func openTest(t *testing.T) *Database {
	t.Helper()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	return db
}

func TestCollection_CycleDeVie(t *testing.T) {
	db := openTest(t)
	c := db.Categories

	cat := models.Category{ID: "cat_1", Name: "Réseau"}
	if err := c.Insert(cat); err != nil {
		t.Fatalf("insertion : %v", err)
	}
	if err := c.Insert(cat); !errors.Is(err, ErrDuplicate) {
		t.Errorf("doublon : erreur = %v, attendu ErrDuplicate", err)
	}
	if got, err := c.Get("cat_1"); err != nil || got.Name != "Réseau" {
		t.Errorf("lecture = %+v, erreur %v", got, err)
	}
	if _, err := c.Get("inconnu"); !errors.Is(err, ErrNotFound) {
		t.Errorf("identifiant inconnu : erreur = %v, attendu ErrNotFound", err)
	}

	cat.Name = "Réseau et câblage"
	if err := c.Update(cat); err != nil {
		t.Fatalf("mise à jour : %v", err)
	}
	if got, _ := c.Get("cat_1"); got.Name != "Réseau et câblage" {
		t.Errorf("nom après mise à jour = %q", got.Name)
	}
	if err := c.Delete("cat_1"); err != nil {
		t.Fatalf("suppression : %v", err)
	}
	if c.Count() != 0 {
		t.Errorf("%d élément(s) restant(s), attendu 0", c.Count())
	}
}

// All renvoie une copie : modifier le résultat ne doit pas altérer le magasin.
func TestCollection_AllRenvoieUneCopie(t *testing.T) {
	db := openTest(t)
	_ = db.Products.Insert(models.Product{ID: "prd_1", Name: "Origine", Quantity: 5})

	list := db.Products.All()
	list[0].Name = "Modifié en dehors"
	list[0].Quantity = 999

	got, _ := db.Products.Get("prd_1")
	if got.Name != "Origine" || got.Quantity != 5 {
		t.Errorf("le magasin a été modifié à distance : %+v", got)
	}
}

func TestTransaction_ToutOuRien(t *testing.T) {
	db := openTest(t)
	for _, id := range []string{"prd_1", "prd_2", "prd_3"} {
		_ = db.Products.Insert(models.Product{ID: id, Quantity: 10})
	}

	echec := errors.New("refus au milieu du lot")
	err := db.Products.Transaction(func(items []models.Product) ([]models.Product, error) {
		items[0].Quantity = 0
		items[1].Quantity = 0
		return nil, echec
	})
	if !errors.Is(err, echec) {
		t.Fatalf("erreur = %v, attendue %v", err, echec)
	}
	for _, p := range db.Products.All() {
		if p.Quantity != 10 {
			t.Errorf("%s : quantité = %d, attendu 10 — une transaction refusée ne laisse aucune trace", p.ID, p.Quantity)
		}
	}

	if err := db.Products.Transaction(func(items []models.Product) ([]models.Product, error) {
		for i := range items {
			items[i].Quantity += 5
		}
		return items, nil
	}); err != nil {
		t.Fatalf("transaction : %v", err)
	}
	for _, p := range db.Products.All() {
		if p.Quantity != 15 {
			t.Errorf("%s : quantité = %d, attendu 15", p.ID, p.Quantity)
		}
	}
}

// L'écriture passe par un fichier temporaire renommé : aucun fichier partiel ne
// doit subsister, même après plusieurs écritures.
func TestWriteJSONAtomic_NeLaissePasDeResidu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "donnees.json")

	for i := 0; i < 5; i++ {
		if err := WriteJSONAtomic(path, map[string]int{"passage": i}); err != nil {
			t.Fatalf("écriture : %v", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("fichier temporaire abandonné : %s", e.Name())
		}
	}
	var got map[string]int
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if got["passage"] != 4 {
		t.Errorf("contenu = %v, attendu la dernière écriture", got)
	}
}

// Un fichier illisible est mis de côté plutôt qu'écrasé : les données restent
// récupérables à la main.
func TestCollection_FichierIllisibleMisDeCote(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "products.json")
	if err := os.WriteFile(path, []byte("{ceci n'est pas du JSON"), 0o640); err != nil {
		t.Fatal(err)
	}

	_, err := OpenCollection[models.Product](dir, "products")
	if err == nil {
		t.Fatal("l'ouverture aurait dû signaler le fichier illisible")
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Errorf("le fichier d'origine n'a pas été conservé sous .corrupt : %v", err)
	}
}

func TestNumerotation(t *testing.T) {
	db := openTest(t)

	premier, err := db.NextInvoiceNumber()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := db.NextInvoiceNumber()
	if premier == second {
		t.Fatalf("deux fois le même numéro : %s", premier)
	}
	if !strings.HasPrefix(second, "FA-") {
		t.Errorf("préfixe inattendu : %s", second)
	}

	// Le numéro non utilisé est rendu.
	db.ReleaseInvoiceNumber(second)
	repris, _ := db.NextInvoiceNumber()
	if repris != second {
		t.Errorf("numéro repris = %s, attendu %s", repris, second)
	}
	// Rendre un numéro qui n'est pas le dernier n'a aucun effet.
	db.ReleaseInvoiceNumber(premier)
	if c := db.Settings().InvoiceCounter; c != 2 {
		t.Errorf("compteur = %d, attendu 2 : seul le dernier numéro est rendable", c)
	}
}

func TestNextMovementRefs(t *testing.T) {
	db := openTest(t)

	refs, err := db.NextMovementRefs(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 {
		t.Fatalf("%d référence(s), attendu 3", len(refs))
	}
	suite, _ := db.NextMovementRefs(2)
	vues := map[string]bool{}
	for _, r := range append(refs, suite...) {
		if vues[r] {
			t.Errorf("référence en double : %s", r)
		}
		if !strings.HasPrefix(r, "MVT-") {
			t.Errorf("format inattendu : %s", r)
		}
		vues[r] = true
	}
	if n, _ := db.NextMovementRefs(0); n != nil {
		t.Error("une demande de zéro référence doit renvoyer une liste vide")
	}
}

func TestSauvegardeEtRestauration(t *testing.T) {
	db := openTest(t)
	_ = db.Products.Insert(models.Product{ID: "prd_1", Name: "Avant sauvegarde", Quantity: 7})

	info, err := db.Backup("test")
	if err != nil {
		t.Fatalf("sauvegarde : %v", err)
	}
	if info.SizeBytes == 0 {
		t.Error("archive vide")
	}

	// Les données évoluent, puis on revient en arrière.
	_ = db.Products.Update(models.Product{ID: "prd_1", Name: "Après sauvegarde", Quantity: 99})
	if err := db.Restore(info.Path); err != nil {
		t.Fatalf("restauration : %v", err)
	}

	// La restauration remplace les fichiers ; un redémarrage relit le disque.
	rouvert, err := Open(db.Dir)
	if err != nil {
		t.Fatalf("réouverture : %v", err)
	}
	got, err := rouvert.Products.Get("prd_1")
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if got.Name != "Avant sauvegarde" || got.Quantity != 7 {
		t.Errorf("état restauré = %+v, attendu l'état d'avant la sauvegarde", got)
	}
}

func TestPruneBackups(t *testing.T) {
	db := openTest(t)
	for i := 0; i < 4; i++ {
		if _, err := db.Backup("test"); err != nil {
			t.Fatalf("sauvegarde %d : %v", i, err)
		}
	}
	list, _ := db.ListBackups()
	if len(list) < 2 {
		t.Skip("l'horodatage à la seconde a fusionné les archives")
	}
	removed, err := db.PruneBackups(1)
	if err != nil {
		t.Fatal(err)
	}
	if removed != len(list)-1 {
		t.Errorf("%d archive(s) supprimée(s), attendu %d", removed, len(list)-1)
	}
	if rest, _ := db.ListBackups(); len(rest) != 1 {
		t.Errorf("%d archive(s) restante(s), attendu 1", len(rest))
	}
}

// Une archive dont une entrée remonte hors du dossier de destination doit être
// refusée (« Zip Slip »).
func TestRestore_RefuseUneArchivePiegee(t *testing.T) {
	db := openTest(t)
	piege := filepath.Join(t.TempDir(), "piege.zip")

	f, err := os.Create(piege)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../../evasion.json")
	_, _ = w.Write([]byte("[]"))
	w2, _ := zw.Create("products.json")
	_, _ = w2.Write([]byte("[]"))
	_ = zw.Close()
	_ = f.Close()

	if err := db.Restore(piege); err == nil {
		t.Fatal("l'archive piégée aurait dû être refusée")
	}
	if _, err := os.Stat(filepath.Join(db.Dir, "..", "evasion.json")); err == nil {
		t.Error("un fichier a été écrit hors du dossier de destination")
	}
}

func TestRestore_RefuseUneArchiveEtrangere(t *testing.T) {
	db := openTest(t)
	etranger := filepath.Join(t.TempDir(), "photos.zip")

	f, _ := os.Create(etranger)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("photo.jpg")
	_, _ = w.Write([]byte("pas des données Comptoir"))
	_ = zw.Close()
	_ = f.Close()

	err := db.Restore(etranger)
	if err == nil {
		t.Fatal("une archive sans products.json aurait dû être refusée")
	}
	if !strings.Contains(err.Error(), "sauvegarde Comptoir") {
		t.Errorf("message peu explicite : %v", err)
	}
}

func TestMigrate_RepareLesCompteurs(t *testing.T) {
	db := openTest(t)
	for i := 0; i < 3; i++ {
		_ = db.Movements.Insert(models.Movement{ID: string(rune('a' + i))})
	}
	// Compteur incohérent, comme après une reprise de fichiers à la main.
	s := db.Settings()
	s.MovementCounter = 0
	s.SessionTimeoutMin = 0
	if err := db.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migration : %v", err)
	}
	after := db.Settings()
	if after.MovementCounter < 3 {
		t.Errorf("compteur de mouvements = %d, attendu au moins 3 : des références seraient redistribuées",
			after.MovementCounter)
	}
	if after.SessionTimeoutMin != 60 {
		t.Errorf("délai d'inactivité = %d, attendu la valeur par défaut 60", after.SessionTimeoutMin)
	}
}

func TestOpen_RefuseUnSchemaPlusRecent(t *testing.T) {
	dir := t.TempDir()
	db := openTest(t)
	_ = db

	metaPath := filepath.Join(dir, "data", "meta.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o750); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"schemaVersion": models.SchemaVersion + 5, "app": "Comptoir"})
	if err := os.WriteFile(metaPath, raw, 0o640); err != nil {
		t.Fatal(err)
	}

	_, err := Open(dir)
	if err == nil {
		t.Fatal("des données plus récentes auraient dû être refusées")
	}
	if !strings.Contains(err.Error(), "version plus récente") {
		t.Errorf("message peu explicite : %v", err)
	}
}
