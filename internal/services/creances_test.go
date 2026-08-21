package services

import (
	"strings"
	"testing"
	"time"

	"comptoir/internal/models"
)

// creditVendu enregistre une vente laissée entièrement à crédit, échue depuis
// le nombre de jours indiqué.
func (s *suite) creditVendu(t *testing.T, client, produit string, quantite, jourEcheance int) models.Invoice {
	t.Helper()
	p, err := s.db.Products.FindOne(func(p models.Product) bool { return p.Name == produit })
	if err != nil {
		t.Fatalf("produit %q introuvable", produit)
	}
	echeance := time.Now().AddDate(0, 0, jourEcheance).Format("2006-01-02")
	inv, err := s.sales.CreateInvoice(InvoiceInput{
		CustomerName: client, CustomerPhone: "+223 00 00 00 00",
		Lines:         []InvoiceLineInput{{ProductID: p.ID, Quantity: quantite}},
		PaymentMethod: models.PayCredit, AmountPaid: 0,
		DueDate: echeance, Date: today(),
	})
	if err != nil {
		t.Fatalf("vente à crédit : %v", err)
	}
	return inv
}

func TestVenteACredit_LaisseUnSoldeEtUneEcheance(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	s.product("Onduleur", 1000000, 2000000, 10)

	inv := s.creditVendu(t, "Cabinet Diallo", "Onduleur", 2, 30)

	if inv.Status != models.StatusIssued {
		t.Errorf("statut = %s, attendu ISSUED : une vente à crédit n'est pas réglée", inv.Status)
	}
	if inv.Balance != 4000000 {
		t.Errorf("solde = %d, attendu 4000000", inv.Balance)
	}
	if inv.AmountPaid != 0 {
		t.Errorf("encaissé = %d, attendu 0", inv.AmountPaid)
	}
	if inv.DueDate == nil {
		t.Fatal("aucune échéance sur une vente à crédit")
	}
}

// Le délai des paramètres s'applique quand aucune échéance n'est saisie.
func TestVenteACredit_EcheanceParDefaut(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Écran", 500000, 900000, 10)
	reglages := s.db.Settings()
	reglages.DefaultPaymentTermDays = 45
	if err := s.db.SaveSettings(reglages); err != nil {
		t.Fatal(err)
	}

	inv, err := s.sales.CreateInvoice(InvoiceInput{
		CustomerName:  "Mariam Keïta",
		Lines:         []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}},
		PaymentMethod: models.PayCredit, Date: today(),
	})
	if err != nil {
		t.Fatalf("vente : %v", err)
	}
	if inv.DueDate == nil {
		t.Fatal("aucune échéance appliquée")
	}
	jours := int(inv.DueDate.Sub(time.Now()).Hours() / 24)
	if jours < 43 || jours > 46 {
		t.Errorf("échéance dans %d jours, attendu environ 45", jours)
	}
}

// On ne relance pas « Client comptoir ».
func TestVenteACredit_ExigeUnClientNomme(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Clavier", 100000, 200000, 10)

	_, err := s.sales.CreateInvoice(InvoiceInput{
		Lines:         []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}},
		PaymentMethod: models.PayCredit, Date: today(),
	})
	if err == nil {
		t.Fatal("une vente à crédit sans client nommé aurait dû être refusée")
	}
	if !strings.Contains(err.Error(), "nommer son client") {
		t.Errorf("message peu explicite : %v", err)
	}
	// Une vente réglée comptant n'a évidemment pas cette exigence.
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}}, AmountPaid: 200000, Date: today(),
	}); err != nil {
		t.Errorf("vente au comptant refusée à tort : %v", err)
	}
}

func TestCreances_ClassementParAnciennete(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	s.product("Article", 100000, 100000, 500)

	// Une par tranche, plus une non échue.
	s.creditVendu(t, "Client à jour", "Article", 1, 30)     // non échue
	s.creditVendu(t, "Client en retard", "Article", 2, -10) // 10 jours
	s.creditVendu(t, "Client tardif", "Article", 3, -45)    // 45 jours
	s.creditVendu(t, "Client oublieux", "Article", 4, -75)  // 75 jours
	s.creditVendu(t, "Client perdu", "Article", 5, -200)    // 200 jours

	etat, err := s.creances.Etat(CreanceQuery{})
	if err != nil {
		t.Fatalf("état des créances : %v", err)
	}
	if etat.Nombre != 5 {
		t.Fatalf("%d créance(s), attendu 5", etat.Nombre)
	}
	if etat.Total != 1500000 {
		t.Errorf("total dû = %d, attendu 1500000", etat.Total)
	}
	// Tout sauf la non échue est en retard : 2+3+4+5 = 14 unités à 1000.
	if etat.EnRetard != 1400000 {
		t.Errorf("en retard = %d, attendu 1400000", etat.EnRetard)
	}
	if etat.NombreRetard != 4 {
		t.Errorf("%d facture(s) en retard, attendu 4", etat.NombreRetard)
	}

	attendues := map[Tranche]int64{
		TrancheNonEchue: 100000, Tranche1a30: 200000,
		Tranche31a60: 300000, Tranche61a90: 400000, TrancheAuDela: 500000,
	}
	for _, tr := range etat.Tranches {
		if attendu, ok := attendues[tr.Tranche]; ok && tr.Montant != attendu {
			t.Errorf("tranche %s = %d, attendu %d", tr.Libelle, tr.Montant, attendu)
		}
		delete(attendues, tr.Tranche)
	}
	if len(attendues) != 0 {
		t.Errorf("tranches manquantes : %v", attendues)
	}

	// La plus en retard vient en tête : c'est l'ordre où l'on relance.
	if etat.Lignes[0].CustomerName != "Client perdu" {
		t.Errorf("première ligne = %q, attendu « Client perdu »", etat.Lignes[0].CustomerName)
	}
	if etat.Debiteurs[0].Nom != "Client perdu" {
		t.Errorf("premier débiteur = %q, attendu « Client perdu »", etat.Debiteurs[0].Nom)
	}
}

// Une facture sans échéance n'est pas déclarée en retard : personne n'a promis
// de date. Elle est signalée à part.
func TestCreances_SansEcheance(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Article", 100000, 100000, 10)
	reglages := s.db.Settings()
	reglages.DefaultPaymentTermDays = 0
	if err := s.db.SaveSettings(reglages); err != nil {
		t.Fatal(err)
	}
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		CustomerName:  "Sans terme",
		Lines:         []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}},
		PaymentMethod: models.PayCredit, Date: today(),
	}); err != nil {
		t.Fatal(err)
	}

	etat, err := s.creances.Etat(CreanceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if etat.EnRetard != 0 {
		t.Errorf("en retard = %d, attendu 0 : aucune date n'a été convenue", etat.EnRetard)
	}
	if len(etat.Tranches) != 1 || etat.Tranches[0].Tranche != TrancheSansTerme {
		t.Errorf("tranches = %+v, attendu une seule « sans échéance »", etat.Tranches)
	}
}

func TestCreances_FiltreEnRetard(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	s.product("Article", 100000, 100000, 100)
	s.creditVendu(t, "À jour", "Article", 1, 30)
	s.creditVendu(t, "En retard", "Article", 1, -5)

	etat, err := s.creances.Etat(CreanceQuery{SeulementEnRetard: true})
	if err != nil {
		t.Fatal(err)
	}
	if etat.Nombre != 1 || etat.Lignes[0].CustomerName != "En retard" {
		t.Errorf("filtre inopérant : %d ligne(s), première = %q", etat.Nombre, etat.Lignes[0].CustomerName)
	}
}

// Un règlement solde la créance et retire l'échéance.
func TestCreances_ReglementSoldeLaDette(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	s.product("Article", 100000, 100000, 10)
	inv := s.creditVendu(t, "Bon payeur", "Article", 2, -10)

	if _, err := s.sales.RegisterPayment(PaymentInput{InvoiceID: inv.ID, Amount: inv.Balance}); err != nil {
		t.Fatalf("règlement : %v", err)
	}
	apres, _ := s.db.Invoices.Get(inv.ID)
	if apres.DueDate != nil {
		t.Error("l'échéance subsiste alors que la facture est soldée")
	}
	etat, err := s.creances.Etat(CreanceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if etat.Nombre != 0 {
		t.Errorf("%d créance(s) restante(s), attendu 0", etat.Nombre)
	}
}

func TestCreances_ReporterEcheance(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	s.product("Article", 100000, 100000, 10)
	inv := s.creditVendu(t, "Client négociateur", "Article", 1, -5)

	nouvelle := time.Now().AddDate(0, 0, 20).Format("2006-01-02")
	apres, err := s.creances.FixerEcheance(EcheanceInput{
		InvoiceID: inv.ID, DueDate: nouvelle, Note: "accord verbal du gérant",
	})
	if err != nil {
		t.Fatalf("report : %v", err)
	}
	if apres.DueDate == nil || apres.DueDate.Before(time.Now()) {
		t.Errorf("échéance non reportée : %v", apres.DueDate)
	}
	if !strings.Contains(apres.Notes, "accord verbal") {
		t.Errorf("le motif du report n'est pas conservé : %q", apres.Notes)
	}

	etat, _ := s.creances.Etat(CreanceQuery{})
	if etat.EnRetard != 0 {
		t.Errorf("la créance est encore comptée en retard après report")
	}

	// Une facture réglée n'a pas d'échéance à reporter.
	if _, err := s.sales.RegisterPayment(PaymentInput{InvoiceID: inv.ID, Amount: apres.Balance}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.creances.FixerEcheance(EcheanceInput{InvoiceID: inv.ID, DueDate: nouvelle}); err == nil {
		t.Error("reporter l'échéance d'une facture réglée aurait dû être refusé")
	}
}

func TestCreances_TableauDeBordSignaleLesRetards(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	s.product("Article", 100000, 100000, 100)
	s.creditVendu(t, "À jour", "Article", 3, 30)
	s.creditVendu(t, "En retard", "Article", 2, -20)

	d, err := s.reports.Dashboard()
	if err != nil {
		t.Fatal(err)
	}
	if d.Outstanding != 500000 {
		t.Errorf("impayés = %d, attendu 500000", d.Outstanding)
	}
	if d.Overdue != 200000 {
		t.Errorf("en retard = %d, attendu 200000", d.Overdue)
	}
	if d.OverdueCount != 1 {
		t.Errorf("%d facture(s) en retard, attendu 1", d.OverdueCount)
	}
}

func TestRelance_Document(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	s.product("Article", 100000, 100000, 100)
	client, err := s.catalog.SaveParty(PartyInput{
		Type: models.PartyCustomer, Name: "Cabinet Diallo", Phone: "+223 76 12 44 08", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.db.Products.FindOne(func(p models.Product) bool { return p.Name == "Article" })
	for _, jours := range []int{-40, -5} {
		if _, err := s.sales.CreateInvoice(InvoiceInput{
			CustomerID: client.ID, PaymentMethod: models.PayCredit, Date: today(),
			DueDate: time.Now().AddDate(0, 0, jours).Format("2006-01-02"),
			Lines:   []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	f, err := s.docs.Reminder(client.ID)
	if err != nil {
		t.Fatalf("relance : %v", err)
	}
	if len(f.Content) < 1000 || string(f.Content[:4]) != "%PDF" {
		t.Errorf("document invalide (%d octets)", len(f.Content))
	}

	// Un client à jour n'a rien à se voir relancer.
	autre, _ := s.catalog.SaveParty(PartyInput{
		Type: models.PartyCustomer, Name: "Client sans dette", Active: true,
	})
	if _, err := s.docs.Reminder(autre.ID); err == nil {
		t.Error("une relance a été produite pour un client sans impayé")
	}
}
