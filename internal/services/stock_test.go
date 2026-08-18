package services

import (
	"testing"

	"comptoir/internal/models"
)

func TestDefectueux_CycleComplet(t *testing.T) {
	s := newSuite(t)
	p := s.product("Alimentation", 400000, 700000, 10)

	if _, err := s.stock.DeclareDefective(MovementInput{
		ProductID: p.ID, Quantity: 3, Reason: "ne démarre pas",
	}); err != nil {
		t.Fatalf("déclaration de défaut : %v", err)
	}
	after := s.reload(p.ID)
	if after.Quantity != 7 || after.DefectiveQty != 3 {
		t.Fatalf("stock = %d sain / %d défectueux, attendu 7 / 3", after.Quantity, after.DefectiveQty)
	}

	if _, err := s.stock.RepairDefective(MovementInput{ProductID: p.ID, Quantity: 1}); err != nil {
		t.Fatalf("réparation : %v", err)
	}
	after = s.reload(p.ID)
	if after.Quantity != 8 || after.DefectiveQty != 2 {
		t.Fatalf("après réparation : %d / %d, attendu 8 / 2", after.Quantity, after.DefectiveQty)
	}

	if _, err := s.stock.ScrapDefective(MovementInput{
		ProductID: p.ID, Quantity: 2, Reason: "irréparable",
	}); err != nil {
		t.Fatalf("rebut : %v", err)
	}
	after = s.reload(p.ID)
	if after.Quantity != 8 || after.DefectiveQty != 0 {
		t.Errorf("après rebut : %d / %d, attendu 8 / 0", after.Quantity, after.DefectiveQty)
	}
}

func TestDefectueux_RefuseAuDelaDuDisponible(t *testing.T) {
	s := newSuite(t)
	p := s.product("Batterie", 400000, 700000, 2)

	if _, err := s.stock.DeclareDefective(MovementInput{
		ProductID: p.ID, Quantity: 5, Reason: "gonflée",
	}); err == nil {
		t.Fatal("déclarer 5 défectueux sur 2 en stock aurait dû être refusé")
	}
	if got := s.reload(p.ID).Quantity; got != 2 {
		t.Errorf("stock = %d, attendu 2", got)
	}
	if _, err := s.stock.ScrapDefective(MovementInput{
		ProductID: p.ID, Quantity: 1, Reason: "essai",
	}); err == nil {
		t.Error("mettre au rebut sans stock défectueux aurait dû être refusé")
	}
}

func TestRetourFournisseur_PrendLeDefectueuxEnPriorite(t *testing.T) {
	s := newSuite(t)
	p := s.product("Modem", 400000, 700000, 10)
	if _, err := s.stock.DeclareDefective(MovementInput{
		ProductID: p.ID, Quantity: 4, Reason: "défaut d'usine",
	}); err != nil {
		t.Fatalf("déclaration de défaut : %v", err)
	}

	if _, err := s.stock.ReturnToSupplier(MovementInput{
		ProductID: p.ID, Quantity: 6, Reason: "retour sous garantie",
	}); err != nil {
		t.Fatalf("retour fournisseur : %v", err)
	}
	after := s.reload(p.ID)
	if after.DefectiveQty != 0 {
		t.Errorf("défectueux = %d, attendu 0 : ils partent en premier", after.DefectiveQty)
	}
	if after.Quantity != 4 {
		t.Errorf("stock sain = %d, attendu 4 (6 − 4 défectueux pris sur le sain)", after.Quantity)
	}
}

func TestAjustementInventaire(t *testing.T) {
	s := newSuite(t)
	p := s.product("Clé USB", 200000, 400000, 50)

	if _, err := s.stock.AdjustInventory(InventoryInput{
		ProductID: p.ID, CountedSound: 47, CountedDefect: 1, Reason: "comptage annuel",
	}); err != nil {
		t.Fatalf("ajustement : %v", err)
	}
	after := s.reload(p.ID)
	if after.Quantity != 47 || after.DefectiveQty != 1 {
		t.Errorf("stock = %d / %d, attendu 47 / 1", after.Quantity, after.DefectiveQty)
	}

	if _, err := s.stock.AdjustInventory(InventoryInput{
		ProductID: p.ID, CountedSound: 47, CountedDefect: 1, Reason: "re-comptage",
	}); err == nil {
		t.Error("un ajustement sans écart aurait dû être refusé")
	}
	if _, err := s.stock.AdjustInventory(InventoryInput{
		ProductID: p.ID, CountedSound: -1, Reason: "erreur",
	}); err == nil {
		t.Error("un comptage négatif aurait dû être refusé")
	}
}

// Chaque variation de stock laisse un mouvement daté : c'est la règle qui rend
// l'inventaire vérifiable.
func TestChaqueVariationLaisseUnMouvement(t *testing.T) {
	s := newSuite(t)
	p := s.product("Casque", 200000, 400000, 20) // 1 mouvement : stock initial

	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		Lines: []PurchaseLineInput{{ProductID: p.ID, Quantity: 5, UnitCost: 200000}},
	}); err != nil {
		t.Fatalf("réception : %v", err)
	}
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}},
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}
	if _, err := s.stock.DeclareDefective(MovementInput{
		ProductID: p.ID, Quantity: 1, Reason: "grésille",
	}); err != nil {
		t.Fatalf("défaut : %v", err)
	}

	movements := s.db.Movements.Find(func(m models.Movement) bool { return m.ProductID == p.ID })
	if len(movements) != 4 {
		t.Fatalf("%d mouvement(s), attendu 4 (initial, entrée, vente, défaut)", len(movements))
	}
	// Le stock final doit correspondre au dernier StockAfter enregistré.
	last := movements[len(movements)-1]
	if got := s.reload(p.ID).Quantity; got != last.StockAfter {
		t.Errorf("stock = %d mais le journal dit %d", got, last.StockAfter)
	}
	// Chaque mouvement porte une référence unique.
	refs := map[string]bool{}
	for _, m := range movements {
		if m.Ref == "" {
			t.Error("un mouvement est sans référence")
		}
		if refs[m.Ref] {
			t.Errorf("référence en double : %s", m.Ref)
		}
		refs[m.Ref] = true
	}
}

func TestHistorique(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Tablette", 5000000, 8000000, 10)
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 3}},
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}

	h, err := s.stock.History(p.ID)
	if err != nil {
		t.Fatalf("historique : %v", err)
	}
	if h.TotalSold != 3 {
		t.Errorf("vendus = %d, attendu 3", h.TotalSold)
	}
	if h.Revenue != 24000000 {
		t.Errorf("chiffre d'affaires = %d, attendu 24000000", h.Revenue)
	}
	if h.CostOfSales != 15000000 {
		t.Errorf("coût des ventes = %d, attendu 15000000", h.CostOfSales)
	}
}

func TestSummary(t *testing.T) {
	s := newSuite(t)
	s.product("Article vendable", 100000, 200000, 10)
	rupture, err := s.catalog.SaveProduct(ProductInput{
		Name: "Article en rupture", PurchasePrice: 100000, SalePrice: 200000, Active: true,
	})
	if err != nil {
		t.Fatalf("création : %v", err)
	}
	_ = rupture

	sum, err := s.stock.Summary()
	if err != nil {
		t.Fatalf("résumé : %v", err)
	}
	if sum.ProductCount != 2 {
		t.Errorf("articles = %d, attendu 2", sum.ProductCount)
	}
	if sum.TotalUnits != 10 {
		t.Errorf("unités = %d, attendu 10", sum.TotalUnits)
	}
	if sum.StockValueCost != 1000000 {
		t.Errorf("valeur au coût = %d, attendu 1000000", sum.StockValueCost)
	}
	if sum.OutOfStockCount != 1 {
		t.Errorf("ruptures = %d, attendu 1", sum.OutOfStockCount)
	}
}
