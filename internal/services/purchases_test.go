package services

import (
	"testing"

	"comptoir/internal/models"
)

// mv abrège le type des mouvements dans les prédicats de recherche.
type mv = models.Movement

func TestCreatePurchase_EntreeEtCoutMoyen(t *testing.T) {
	s := newSuite(t)
	p := s.product("Barrette RAM", 1000000, 1500000, 10)

	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		Lines: []PurchaseLineInput{{ProductID: p.ID, Quantity: 10, UnitCost: 2000000, TaxRate: rate(0)}},
	}); err != nil {
		t.Fatalf("réception : %v", err)
	}

	after := s.reload(p.ID)
	if after.Quantity != 20 {
		t.Errorf("stock = %d, attendu 20", after.Quantity)
	}
	if after.PurchasePrice != 1500000 {
		t.Errorf("coût moyen = %d, attendu 1500000 : (10 × 1M + 10 × 2M) / 20", after.PurchasePrice)
	}
}

// Régression : deux lignes du même article dans un bon d'entrée doivent
// s'additionner. Une version antérieure appliquait chaque ligne sur un
// instantané pris avant la boucle, et la seconde écrasait la première.
func TestCreatePurchase_DeuxLignesDuMemeArticle(t *testing.T) {
	s := newSuite(t)
	p := s.product("Cartouche", 500000, 900000, 0)

	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		Lines: []PurchaseLineInput{
			{ProductID: p.ID, Quantity: 10, UnitCost: 400000, TaxRate: rate(0)},
			{ProductID: p.ID, Quantity: 10, UnitCost: 600000, TaxRate: rate(0)},
		},
	}); err != nil {
		t.Fatalf("réception : %v", err)
	}

	after := s.reload(p.ID)
	if after.Quantity != 20 {
		t.Fatalf("stock = %d, attendu 20 : les deux lignes doivent s'additionner", after.Quantity)
	}
	if after.PurchasePrice != 500000 {
		t.Errorf("coût moyen = %d, attendu 500000 : (10 × 400000 + 10 × 600000) / 20", after.PurchasePrice)
	}
	movements := s.db.Movements.Find(func(m mv) bool { return m.ProductID == p.ID })
	if len(movements) != 2 {
		t.Errorf("%d mouvement(s) écrit(s), attendu 2 (un par ligne)", len(movements))
	}
	if last := movements[len(movements)-1]; last.StockAfter != 20 {
		t.Errorf("stock après le dernier mouvement = %d, attendu 20", last.StockAfter)
	}
}

func TestCreatePurchase_FraisAnnexesRepartisAuProrata(t *testing.T) {
	s := newSuite(t)
	a := s.product("Article A", 0, 0, 0)
	b := s.product("Article B", 0, 0, 0)

	// 30 000 de marchandise, 3 000 de transport : le coût de revient est majoré
	// de 10 %, réparti au prorata de la valeur de chaque ligne.
	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		OtherCosts: 3000,
		Lines: []PurchaseLineInput{
			{ProductID: a.ID, Quantity: 2, UnitCost: 10000, TaxRate: rate(0)}, // 20 000
			{ProductID: b.ID, Quantity: 1, UnitCost: 10000, TaxRate: rate(0)}, // 10 000
		},
	}); err != nil {
		t.Fatalf("réception : %v", err)
	}

	if got := s.reload(a.ID).PurchasePrice; got != 11000 {
		t.Errorf("coût de A = %d, attendu 11000 ((20000 + 2000) / 2)", got)
	}
	if got := s.reload(b.ID).PurchasePrice; got != 11000 {
		t.Errorf("coût de B = %d, attendu 11000 (10000 + 1000)", got)
	}
}

func TestCreatePurchase_MargeCible(t *testing.T) {
	s := newSuite(t)
	p := s.product("Switch réseau", 0, 0, 0)

	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		TargetMarginPct: 25,
		Lines:           []PurchaseLineInput{{ProductID: p.ID, Quantity: 5, UnitCost: 750000, TaxRate: rate(0)}},
	}); err != nil {
		t.Fatalf("réception : %v", err)
	}
	// PV = coût / (1 − 0,25) = 750 000 / 0,75 = 1 000 000.
	if got := s.reload(p.ID).SalePrice; got != 1000000 {
		t.Errorf("prix de vente = %d, attendu 1000000", got)
	}
}

func TestCreatePurchase_LigneInvalideNEcritRien(t *testing.T) {
	s := newSuite(t)
	p := s.product("Ventilateur", 100000, 200000, 5)

	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		Lines: []PurchaseLineInput{
			{ProductID: p.ID, Quantity: 3, UnitCost: 100000},
			{ProductID: "produit-inexistant", Quantity: 1, UnitCost: 100000},
		},
	}); err == nil {
		t.Fatal("un produit introuvable aurait dû faire échouer le bon d'entrée")
	}
	if got := s.reload(p.ID).Quantity; got != 5 {
		t.Errorf("stock = %d, attendu 5 : rien ne doit être appliqué", got)
	}
	if n := s.db.Purchases.Count(); n != 0 {
		t.Errorf("%d bon(s) enregistré(s), attendu 0", n)
	}
	if c := s.db.Settings().PurchaseCounter; c != 0 {
		t.Errorf("compteur = %d, attendu 0 : le numéro n'a pas été rendu", c)
	}
}

func TestCancelPurchase_RefuseSiDejaVendu(t *testing.T) {
	s := newSuite(t)
	p := s.product("Webcam", 0, 500000, 0)

	pu, err := s.purchases.CreatePurchase(PurchaseInput{
		Lines: []PurchaseLineInput{{ProductID: p.ID, Quantity: 5, UnitCost: 300000, TaxRate: rate(0)}},
	})
	if err != nil {
		t.Fatalf("réception : %v", err)
	}
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 3}},
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}

	if err := s.purchases.CancelPurchase(pu.ID, "erreur fournisseur"); err == nil {
		t.Fatal("l'annulation aurait dû être refusée : 3 unités sur 5 sont déjà vendues")
	}
	if got := s.reload(p.ID).Quantity; got != 2 {
		t.Errorf("stock = %d, attendu 2 : un refus ne change rien", got)
	}
}

func TestCancelPurchase_RetireLaMarchandise(t *testing.T) {
	s := newSuite(t)
	p := s.product("Sacoche", 0, 500000, 0)

	pu, _ := s.purchases.CreatePurchase(PurchaseInput{
		Lines: []PurchaseLineInput{{ProductID: p.ID, Quantity: 5, UnitCost: 300000, TaxRate: rate(0)}},
	})
	if err := s.purchases.CancelPurchase(pu.ID, "livraison non conforme"); err != nil {
		t.Fatalf("annulation : %v", err)
	}
	if got := s.reload(p.ID).Quantity; got != 0 {
		t.Errorf("stock = %d, attendu 0", got)
	}
}
