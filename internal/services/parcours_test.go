package services

import (
	"strings"
	"testing"

	"comptoir/internal/models"
	"comptoir/internal/storage"
)

// TestParcoursComplet rejoue une journée de boutique du premier démarrage à la
// clôture, dans l'ordre où un commerçant la vit. Les tests précédents vérifient
// chaque règle isolément ; celui-ci vérifie qu'elles tiennent ensemble, et que
// les chiffres se recoupent d'un bout à l'autre de la chaîne.
func TestParcoursComplet(t *testing.T) {
	s := newSuite(t)
	session := NewSession(s.db, s.sec)

	// --- 1. Le catalogue est repris depuis le tableur du gérant --------------
	catalogue := strings.Join([]string{
		"Désignation;Référence;Catégorie;Prix de vente;Coût d'achat;Stock;Seuil",
		"Ordinateur portable 14 pouces;PC-14;Ordinateurs portables;450 000;320 000;6;2",
		"Souris sans fil;SOU-BT;Accessoires;9 000;5 000;40;10",
		"Cartouche noire;CAR-N;Consommables;28 000;18 000;15;5",
	}, "\n")
	if _, err := s.catalog.ImportProducts(catalogue, true); err != nil {
		t.Fatalf("import du catalogue : %v", err)
	}
	if n := s.db.Products.Count(); n != 3 {
		t.Fatalf("%d article(s) importé(s), attendu 3", n)
	}
	s.setTax(18)

	pc := mustFind(t, s, "PC-14")
	souris := mustFind(t, s, "SOU-BT")

	// --- 2. Une réception fournisseur, transport compris ---------------------
	fournisseur, err := s.catalog.SaveParty(PartyInput{
		Type: models.PartySupplier, Name: "Sahel Informatique", Phone: "+223 00 00 00 00", Active: true,
	})
	if err != nil {
		t.Fatalf("création du fournisseur : %v", err)
	}
	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		SupplierID: fournisseur.ID, Reference: "FA-2026-889",
		OtherCosts: 6000000, // 60 000 de transport
		Lines: []PurchaseLineInput{
			{ProductID: pc.ID, Quantity: 4, UnitCost: 33000000, TaxRate: rate(0)},
		},
	}); err != nil {
		t.Fatalf("réception : %v", err)
	}

	pcApres := s.reload(pc.ID)
	if pcApres.Quantity != 10 {
		t.Fatalf("stock du portable = %d, attendu 10 (6 + 4)", pcApres.Quantity)
	}
	// Coût de revient de l'entrée : (4 × 330 000 + 60 000) / 4 = 345 000.
	// Coût moyen : (6 × 320 000 + 4 × 345 000) / 10 = 330 000.
	if pcApres.PurchasePrice != 33000000 {
		t.Errorf("coût moyen = %d, attendu 33000000, les frais de transport sont-ils intégrés ?", pcApres.PurchasePrice)
	}

	// --- 3. Une vente au comptoir, réglée en partie -------------------------
	client, err := s.catalog.SaveParty(PartyInput{
		Type: models.PartyCustomer, Name: "Cabinet Diallo", Active: true,
	})
	if err != nil {
		t.Fatalf("création du client : %v", err)
	}
	facture, err := s.sales.CreateInvoice(InvoiceInput{
		CustomerID: client.ID,
		Lines: []InvoiceLineInput{
			{ProductID: pc.ID, Quantity: 2},
			{ProductID: souris.ID, Quantity: 2},
		},
		AmountPaid: 50000000,
		Date:       today(),
	})
	if err != nil {
		t.Fatalf("vente : %v", err)
	}
	if facture.Status != models.StatusPartial {
		t.Errorf("statut = %s, attendu PARTIAL", facture.Status)
	}
	// HT : 2 × 450 000 + 2 × 9 000 = 918 000. TTC : 918 000 × 1,18 = 1 083 240.
	if facture.SubtotalHT != 91800000 {
		t.Errorf("HT = %d, attendu 91800000", facture.SubtotalHT)
	}
	if facture.Total != 108324000 {
		t.Errorf("TTC = %d, attendu 108324000", facture.Total)
	}
	// Marge : 918 000 − (2 × 330 000 + 2 × 5 000) = 248 000.
	if facture.Margin != 24800000 {
		t.Errorf("marge = %d, attendu 24800000", facture.Margin)
	}
	if got := s.reload(pc.ID).Quantity; got != 8 {
		t.Errorf("stock après vente = %d, attendu 8", got)
	}

	// --- 4. Le client solde sa facture --------------------------------------
	solde := facture.Balance
	facture, err = s.sales.RegisterPayment(PaymentInput{InvoiceID: facture.ID, Amount: solde})
	if err != nil {
		t.Fatalf("règlement : %v", err)
	}
	if facture.Status != models.StatusPaid || facture.Balance != 0 {
		t.Fatalf("statut=%s solde=%d, attendu PAID / 0", facture.Status, facture.Balance)
	}

	// --- 5. Un article revient cassé, il est mis au rebut -------------------
	if _, err := s.stock.DeclareDefective(MovementInput{
		ProductID: souris.ID, Quantity: 3, Reason: "molette bloquée",
	}); err != nil {
		t.Fatalf("déclaration de défaut : %v", err)
	}
	if _, err := s.stock.ScrapDefective(MovementInput{
		ProductID: souris.ID, Quantity: 3, Reason: "non réparable",
	}); err != nil {
		t.Fatalf("rebut : %v", err)
	}

	// --- 6. Le loyer du mois -------------------------------------------------
	if _, err := s.expenses.SaveExpense(ExpenseInput{
		Category: "Loyer", Label: "Loyer du local", Amount: 15000000, Date: today(),
	}); err != nil {
		t.Fatalf("charge : %v", err)
	}

	// --- 7. Les comptes de la journée doivent se recouper -------------------
	st, err := s.reports.IncomeStatement("", "")
	if err != nil {
		t.Fatalf("compte de résultat : %v", err)
	}
	if st.RevenueHT != facture.SubtotalHT {
		t.Errorf("CA du compte de résultat = %d, mais la facture porte %d", st.RevenueHT, facture.SubtotalHT)
	}
	if st.GrossMargin != facture.Margin {
		t.Errorf("marge brute = %d, mais la facture porte %d", st.GrossMargin, facture.Margin)
	}
	if st.TotalExpenses != 15000000 {
		t.Errorf("charges = %d, attendu 15000000", st.TotalExpenses)
	}
	// Rebut : 3 souris au coût moyen de 5 000.
	if st.ScrapLoss != 1500000 {
		t.Errorf("pertes sur rebuts = %d, attendu 1500000", st.ScrapLoss)
	}
	attendu := st.GrossMargin - st.TotalExpenses - st.ScrapLoss
	if st.OperatingResult != attendu {
		t.Errorf("résultat = %d, incohérent avec ses composantes (%d)", st.OperatingResult, attendu)
	}

	// La situation patrimoniale doit refléter le même stock.
	bilan, err := s.reports.BalanceSheet("")
	if err != nil {
		t.Fatalf("situation : %v", err)
	}
	var valeurStock int64
	var unites int
	for _, p := range s.db.Products.All() {
		valeurStock += int64(p.Quantity) * p.PurchasePrice
		unites += p.Quantity
	}
	if bilan.StockValueSound != valeurStock {
		t.Errorf("valeur du stock au bilan = %d, calcul direct = %d", bilan.StockValueSound, valeurStock)
	}
	if bilan.Receivables != 0 {
		t.Errorf("créances = %d, attendu 0 : la facture est soldée", bilan.Receivables)
	}

	// --- 8. Le journal doit expliquer chaque unité manquante ----------------
	for _, p := range s.db.Products.All() {
		var solde int
		for _, m := range s.db.Movements.Find(func(m models.Movement) bool { return m.ProductID == p.ID }) {
			switch m.Type {
			case models.MovementIn, models.MovementRepair:
				solde += m.Quantity
			case models.MovementOut, models.MovementReturnSupplier, models.MovementDefect:
				solde -= m.Quantity
			case models.MovementAdjust:
				solde += m.Quantity // stock initial de l'import
			}
		}
		if solde != p.Quantity {
			t.Errorf("« %s » : le journal totalise %d, la fiche porte %d, une variation n'est pas tracée",
				p.Name, solde, p.Quantity)
		}
	}

	// --- 9. Les documents s'impriment ---------------------------------------
	for nom, produire := range map[string]func() (File, error){
		"facture":            func() (File, error) { return s.docs.Invoice(facture.ID) },
		"relevé client":      func() (File, error) { return s.docs.PartyStatement(client.ID) },
		"compte de résultat": func() (File, error) { return s.docs.IncomeStatement("", "") },
		"état du stock":      func() (File, error) { return s.docs.StockReport(ProductQuery{}) },
	} {
		f, err := produire()
		if err != nil {
			t.Errorf("%s : %v", nom, err)
			continue
		}
		if len(f.Content) < 1000 || string(f.Content[:4]) != "%PDF" {
			t.Errorf("%s : le document produit n'est pas un PDF valide", nom)
		}
	}

	// --- 10. La sauvegarde du soir restitue exactement l'état ---------------
	backups := NewBackups(s.db, s.sec)
	archive, err := backups.Create("fin de journée")
	if err != nil {
		t.Fatalf("sauvegarde : %v", err)
	}
	factureAvant := len(s.db.Invoices.All())
	stockAvant := s.reload(pc.ID).Quantity

	// On casse volontairement les données, puis on restaure.
	if err := s.sales.CancelInvoice(facture.ID, "essai de restauration"); err != nil {
		t.Fatalf("annulation : %v", err)
	}
	if err := backups.Restore(archive.Name); err != nil {
		t.Fatalf("restauration : %v", err)
	}
	rouvert, err := storage.Open(s.db.Dir)
	if err != nil {
		t.Fatalf("réouverture : %v", err)
	}
	if n := len(rouvert.Invoices.All()); n != factureAvant {
		t.Errorf("%d facture(s) après restauration, attendu %d", n, factureAvant)
	}
	restaure, err := rouvert.Products.Get(pc.ID)
	if err != nil {
		t.Fatalf("relecture du produit : %v", err)
	}
	if restaure.Quantity != stockAvant {
		t.Errorf("stock restauré = %d, attendu %d", restaure.Quantity, stockAvant)
	}

	// --- 11. La session se ferme --------------------------------------------
	if state := session.Logout(); state.Authenticated {
		t.Error("la session devrait être fermée")
	}
	if _, err := s.reports.Dashboard(); err == nil {
		t.Error("le tableau de bord répond encore après déconnexion")
	}
}

func mustFind(t *testing.T, s *suite, sku string) models.Product {
	t.Helper()
	p, err := s.db.Products.FindOne(func(p models.Product) bool { return p.SKU == sku })
	if err != nil {
		t.Fatalf("article %q introuvable après import", sku)
	}
	return p
}
