package services

import (
	"strings"
	"testing"

	"comptoir/internal/models"
)

func TestCreateInvoice_DeduitLeStockEtCalculeLaMarge(t *testing.T) {
	s := newSuite(t)
	s.setTax(18)
	p := s.product("Clavier", 500000, 800000, 10)

	inv, err := s.sales.CreateInvoice(InvoiceInput{
		Lines:      []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}},
		AmountPaid: 1888000,
	})
	if err != nil {
		t.Fatalf("création de la facture : %v", err)
	}
	if inv.SubtotalHT != 1600000 {
		t.Errorf("HT = %d, attendu 1600000", inv.SubtotalHT)
	}
	if inv.Total != 1888000 {
		t.Errorf("total = %d, attendu 1888000 (1600000 + 18 %%)", inv.Total)
	}
	if inv.Margin != 600000 {
		t.Errorf("marge = %d, attendu 600000 (1600000 − 2 × 500000)", inv.Margin)
	}
	if inv.Status != models.StatusPaid {
		t.Errorf("statut = %s, attendu PAID", inv.Status)
	}
	if got := s.reload(p.ID).Quantity; got != 8 {
		t.Errorf("stock restant = %d, attendu 8", got)
	}
}

func TestCreateInvoice_DeuxLignesDuMemeArticleCumulentLeurStock(t *testing.T) {
	s := newSuite(t)
	p := s.product("Souris", 100000, 200000, 5)

	if _, err := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{
		{ProductID: p.ID, Quantity: 2},
		{ProductID: p.ID, Quantity: 3},
	}}); err != nil {
		t.Fatalf("création de la facture : %v", err)
	}
	if got := s.reload(p.ID).Quantity; got != 0 {
		t.Errorf("stock restant = %d, attendu 0 : les deux lignes doivent s'additionner", got)
	}
}

func TestCreateInvoice_RefuseLeCumulSuperieurAuStock(t *testing.T) {
	s := newSuite(t)
	p := s.product("Écran", 100000, 200000, 4)

	_, err := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{
		{ProductID: p.ID, Quantity: 3},
		{ProductID: p.ID, Quantity: 3},
	}})
	if err == nil {
		t.Fatal("la facture aurait dû être refusée : 6 demandés pour 4 en stock")
	}
	if got := s.reload(p.ID).Quantity; got != 4 {
		t.Errorf("stock = %d, attendu 4 : un refus ne doit rien consommer", got)
	}
	if n := s.db.Invoices.Count(); n != 0 {
		t.Errorf("%d facture(s) enregistrée(s), attendu 0", n)
	}
	// Le numéro réservé doit avoir été rendu : la numérotation ne saute pas.
	if c := s.db.Settings().InvoiceCounter; c != 0 {
		t.Errorf("compteur de factures = %d, attendu 0 : le numéro n'a pas été rendu", c)
	}
}

func TestDevis_PorteLaTaxeEtLaConserveALEmission(t *testing.T) {
	s := newSuite(t)
	s.setTax(18)
	p := s.product("Onduleur", 1000000, 1500000, 3)

	devis, err := s.sales.CreateInvoice(InvoiceInput{
		Draft: true,
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("création du devis : %v", err)
	}
	if devis.TaxTotal != 270000 {
		t.Fatalf("taxes du devis = %d, attendu 270000 : un devis sans taxe n'engage à rien", devis.TaxTotal)
	}
	if devis.Total != 1770000 {
		t.Errorf("total du devis = %d, attendu 1770000", devis.Total)
	}
	if got := s.reload(p.ID).Quantity; got != 3 {
		t.Errorf("stock = %d, attendu 3 : un devis ne sort pas de marchandise", got)
	}

	facture, err := s.sales.IssueDraft(devis.ID)
	if err != nil {
		t.Fatalf("émission du devis : %v", err)
	}
	if facture.TaxTotal != 270000 {
		t.Errorf("taxes de la facture = %d, attendu 270000 : le montant du devis doit être tenu", facture.TaxTotal)
	}
	if facture.Status != models.StatusIssued {
		t.Errorf("statut = %s, attendu ISSUED", facture.Status)
	}
	if got := s.reload(p.ID).Quantity; got != 2 {
		t.Errorf("stock = %d, attendu 2 : l'émission sort la marchandise", got)
	}
}

func TestDevis_LeCoutEstRealigneALEmission(t *testing.T) {
	s := newSuite(t)
	p := s.product("Disque SSD", 1000000, 1500000, 10)

	devis, err := s.sales.CreateInvoice(InvoiceInput{
		Draft: true,
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("création du devis : %v", err)
	}

	// Le fournisseur augmente ses prix entre le devis et la livraison.
	if _, err := s.purchases.CreatePurchase(PurchaseInput{
		Lines: []PurchaseLineInput{{ProductID: p.ID, Quantity: 10, UnitCost: 2000000}},
	}); err != nil {
		t.Fatalf("réception fournisseur : %v", err)
	}

	facture, err := s.sales.IssueDraft(devis.ID)
	if err != nil {
		t.Fatalf("émission du devis : %v", err)
	}
	// Coût moyen après réception : (10 × 1 000 000 + 10 × 2 000 000) / 20.
	if facture.CostTotal != 2*1500000 {
		t.Errorf("coût des ventes = %d, attendu %d : la marge doit refléter le coût du jour de la sortie",
			facture.CostTotal, 2*1500000)
	}
	if facture.Total != devis.Total {
		t.Errorf("total = %d, attendu %d : le prix promis au client ne change pas", facture.Total, devis.Total)
	}
}

func TestLigneExoneree_UnTauxZeroExpliciteEstRespecte(t *testing.T) {
	s := newSuite(t)
	s.setTax(18)
	p := s.product("Livre", 100000, 200000, 5)

	inv, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1, TaxRate: rate(0)}},
	})
	if err != nil {
		t.Fatalf("création de la facture : %v", err)
	}
	if inv.TaxTotal != 0 {
		t.Errorf("taxes = %d, attendu 0 : une exonération explicite doit être respectée", inv.TaxTotal)
	}
	if inv.Total != 200000 {
		t.Errorf("total = %d, attendu 200000", inv.Total)
	}
}

func TestLigneSansTaux_ApplicationDuTauxParDefaut(t *testing.T) {
	s := newSuite(t)
	s.setTax(18)
	p := s.product("Câble", 100000, 200000, 5)

	inv, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("création de la facture : %v", err)
	}
	if inv.TaxTotal != 36000 {
		t.Errorf("taxes = %d, attendu 36000 : sans taux précisé, celui des paramètres s'applique", inv.TaxTotal)
	}
}

func TestRegisterPayment_SoldeEtStatut(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Imprimante", 5000000, 8000000, 4)

	inv, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("création de la facture : %v", err)
	}
	if inv.Status != models.StatusIssued || inv.Balance != 8000000 {
		t.Fatalf("statut=%s solde=%d, attendu ISSUED / 8000000", inv.Status, inv.Balance)
	}

	inv, err = s.sales.RegisterPayment(PaymentInput{InvoiceID: inv.ID, Amount: 3000000})
	if err != nil {
		t.Fatalf("premier règlement : %v", err)
	}
	if inv.Status != models.StatusPartial || inv.Balance != 5000000 {
		t.Errorf("statut=%s solde=%d, attendu PARTIAL / 5000000", inv.Status, inv.Balance)
	}

	if _, err := s.sales.RegisterPayment(PaymentInput{InvoiceID: inv.ID, Amount: 9000000}); err == nil {
		t.Error("un règlement supérieur au solde aurait dû être refusé")
	}

	inv, err = s.sales.RegisterPayment(PaymentInput{InvoiceID: inv.ID, Amount: 5000000})
	if err != nil {
		t.Fatalf("solde du règlement : %v", err)
	}
	if inv.Status != models.StatusPaid || inv.Balance != 0 {
		t.Errorf("statut=%s solde=%d, attendu PAID / 0", inv.Status, inv.Balance)
	}
}

func TestCancelInvoice_RemetEnStockEtSignaleLAcompte(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Routeur", 1000000, 2000000, 5)

	inv, err := s.sales.CreateInvoice(InvoiceInput{
		Lines:      []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}},
		AmountPaid: 1500000,
	})
	if err != nil {
		t.Fatalf("création de la facture : %v", err)
	}
	if err := s.sales.CancelInvoice(inv.ID, "erreur de saisie"); err != nil {
		t.Fatalf("annulation : %v", err)
	}

	after, _ := s.db.Invoices.Get(inv.ID)
	if after.Status != models.StatusCancelled {
		t.Errorf("statut = %s, attendu CANCELLED", after.Status)
	}
	if after.RefundDue != 1500000 {
		t.Errorf("à rembourser = %d, attendu 1500000 : un acompte encaissé ne disparaît pas", after.RefundDue)
	}
	if !strings.Contains(after.Notes, "rembourser") {
		t.Errorf("les notes ne mentionnent pas le remboursement : %q", after.Notes)
	}
	if got := s.reload(p.ID).Quantity; got != 5 {
		t.Errorf("stock = %d, attendu 5 : la marchandise revient", got)
	}
	if err := s.sales.CancelInvoice(inv.ID, "encore"); err == nil {
		t.Error("une deuxième annulation aurait dû être refusée")
	}
}

func TestCancelInvoice_ExigeUnMotif(t *testing.T) {
	s := newSuite(t)
	p := s.product("Chargeur", 100000, 200000, 3)
	inv, _ := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}}})

	if err := s.sales.CancelInvoice(inv.ID, "   "); err == nil {
		t.Error("une annulation sans motif aurait dû être refusée")
	}
}

func TestNumerosDeSerie(t *testing.T) {
	s := newSuite(t)
	p, err := s.catalog.SaveProduct(ProductInput{
		Name: "Portable ThinkPad", SalePrice: 40000000, Serialized: true,
		InitialQuantity: 3, Active: true,
	})
	if err != nil {
		t.Fatalf("création du produit : %v", err)
	}

	if _, err := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{
		{ProductID: p.ID, Quantity: 2, Serials: []string{"SN-001"}},
	}}); err == nil {
		t.Error("un numéro pour deux unités aurait dû être refusé")
	}
	if _, err := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{
		{ProductID: p.ID, Quantity: 2, Serials: []string{"SN-001", "sn-001"}},
	}}); err == nil {
		t.Error("deux fois le même numéro aurait dû être refusé")
	}

	inv, err := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{
		{ProductID: p.ID, Quantity: 2, Serials: []string{"SN-001", "SN-002"}},
	}})
	if err != nil {
		t.Fatalf("création de la facture : %v", err)
	}
	if len(inv.Lines[0].Serials) != 2 {
		t.Errorf("%d numéro(s) conservé(s), attendu 2", len(inv.Lines[0].Serials))
	}

	simple := s.product("Câble HDMI", 100000, 200000, 5)
	if _, err := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{
		{ProductID: simple.ID, Quantity: 1, Serials: []string{"SN-999"}},
	}}); err == nil {
		t.Error("un article non suivi à l'unité ne doit pas accepter de numéro de série")
	}
}

func TestDeleteDraft_SeulementLesDevis(t *testing.T) {
	s := newSuite(t)
	p := s.product("Toner", 500000, 900000, 5)

	devis, _ := s.sales.CreateInvoice(InvoiceInput{Draft: true, Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}}})
	if err := s.sales.DeleteDraft(devis.ID); err != nil {
		t.Fatalf("suppression du devis : %v", err)
	}

	facture, _ := s.sales.CreateInvoice(InvoiceInput{Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}}})
	if err := s.sales.DeleteDraft(facture.ID); err == nil {
		t.Error("une facture émise ne doit jamais être supprimable")
	}
}
