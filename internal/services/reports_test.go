package services

import (
	"testing"
	"time"
)

func TestIncomeStatement(t *testing.T) {
	s := newSuite(t)
	s.setTax(18)
	p := s.product("Serveur", 100000000, 150000000, 5)

	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}}, Date: today(), AmountPaid: 100000000,
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}
	if _, err := s.expenses.SaveExpense(ExpenseInput{
		Category: "Loyer", Label: "Loyer du mois", Amount: 20000000, Date: today(),
	}); err != nil {
		t.Fatalf("charge : %v", err)
	}

	st, err := s.reports.IncomeStatement("", "")
	if err != nil {
		t.Fatalf("compte de résultat : %v", err)
	}
	if st.RevenueHT != 300000000 {
		t.Errorf("CA HT = %d, attendu 300000000", st.RevenueHT)
	}
	if st.TaxCollected != 54000000 {
		t.Errorf("taxes facturées = %d, attendu 54000000", st.TaxCollected)
	}
	if st.CostOfSales != 200000000 {
		t.Errorf("coût des ventes = %d, attendu 200000000", st.CostOfSales)
	}
	if st.GrossMargin != 100000000 {
		t.Errorf("marge brute = %d, attendu 100000000", st.GrossMargin)
	}
	if st.TotalExpenses != 20000000 {
		t.Errorf("charges = %d, attendu 20000000", st.TotalExpenses)
	}
	if st.OperatingResult != 80000000 {
		t.Errorf("résultat = %d, attendu 80000000", st.OperatingResult)
	}
	if st.CashCollected != 100000000 {
		t.Errorf("encaissé = %d, attendu 100000000", st.CashCollected)
	}
}

func TestIncomeStatement_ExclutDevisEtAnnulations(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Vidéoprojecteur", 20000000, 35000000, 10)

	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Draft: true, Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}}, Date: today(),
	}); err != nil {
		t.Fatalf("devis : %v", err)
	}
	annulee, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}}, Date: today(),
	})
	if err != nil {
		t.Fatalf("facture : %v", err)
	}
	if err := s.sales.CancelInvoice(annulee.ID, "erreur de saisie"); err != nil {
		t.Fatalf("annulation : %v", err)
	}
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}}, Date: today(),
	}); err != nil {
		t.Fatalf("vente réelle : %v", err)
	}

	st, err := s.reports.IncomeStatement("", "")
	if err != nil {
		t.Fatalf("compte de résultat : %v", err)
	}
	if st.RevenueHT != 70000000 {
		t.Errorf("CA HT = %d, attendu 70000000 : seule la vente réelle compte", st.RevenueHT)
	}
	if st.InvoiceCount != 1 {
		t.Errorf("factures comptées = %d, attendu 1", st.InvoiceCount)
	}
}

func TestSalesReport_SansTrouDansLaSerie(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Câble réseau", 100000, 250000, 100)

	now := time.Now()
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 4}},
		Date:  now.AddDate(0, 0, -6).Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}

	r, err := s.reports.SalesReport(ReportQuery{
		From:        now.AddDate(0, 0, -6).Format("2006-01-02"),
		To:          now.Format("2006-01-02"),
		Granularity: GranDay,
	})
	if err != nil {
		t.Fatalf("rapport : %v", err)
	}
	if len(r.Points) != 7 {
		t.Errorf("%d point(s), attendu 7 : les journées sans vente doivent apparaître à zéro", len(r.Points))
	}
	if r.Total.RevenueHT != 1000000 {
		t.Errorf("CA total = %d, attendu 1000000", r.Total.RevenueHT)
	}
	if r.Total.InvoiceCount != 1 {
		t.Errorf("factures = %d, attendu 1", r.Total.InvoiceCount)
	}
	if r.Best == nil || r.Best.RevenueHT != 1000000 {
		t.Error("la meilleure période n'est pas celle de la vente")
	}
}

func TestBalanceSheet(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Poste de travail", 20000000, 32000000, 10)

	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}}, AmountPaid: 30000000, Date: today(),
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}

	b, err := s.reports.BalanceSheet("")
	if err != nil {
		t.Fatalf("situation : %v", err)
	}
	if b.StockValueSound != 160000000 {
		t.Errorf("valeur du stock = %d, attendu 160000000 (8 × 20000000)", b.StockValueSound)
	}
	if b.Receivables != 34000000 {
		t.Errorf("créances = %d, attendu 34000000 (64000000 − 30000000)", b.Receivables)
	}
	if b.TotalAssets != 194000000 {
		t.Errorf("total actif = %d, attendu 194000000", b.TotalAssets)
	}
	if b.CumulativeResult != 24000000 {
		t.Errorf("résultat cumulé = %d, attendu 24000000", b.CumulativeResult)
	}
}

func TestStatistics(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	vendu := s.product("Article vendu", 100000, 300000, 20)
	dormant := s.product("Article dormant", 500000, 900000, 10)

	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: vendu.ID, Quantity: 5}}, Date: today(),
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}

	stats, err := s.reports.Statistics(ReportQuery{})
	if err != nil {
		t.Fatalf("statistiques : %v", err)
	}
	if len(stats.TopProducts) != 1 || stats.TopProducts[0].ProductID != vendu.ID {
		t.Fatalf("classement inattendu : %+v", stats.TopProducts)
	}
	if stats.TopProducts[0].Revenue != 1500000 {
		t.Errorf("CA de l'article = %d, attendu 1500000", stats.TopProducts[0].Revenue)
	}
	if len(stats.SlowProducts) != 1 || stats.SlowProducts[0].ProductID != dormant.ID {
		t.Errorf("l'article dormant n'est pas signalé : %+v", stats.SlowProducts)
	}
	if stats.SlowProducts[0].Margin != 5000000 {
		t.Errorf("capital immobilisé = %d, attendu 5000000", stats.SlowProducts[0].Margin)
	}
}

func TestExpenseBreakdown(t *testing.T) {
	s := newSuite(t)
	for _, e := range []struct {
		cat    string
		amount int64
	}{{"Loyer", 30000000}, {"Salaires", 60000000}, {"Loyer", 10000000}} {
		if _, err := s.expenses.SaveExpense(ExpenseInput{
			Category: e.cat, Label: e.cat, Amount: e.amount, Date: today(),
		}); err != nil {
			t.Fatalf("charge : %v", err)
		}
	}

	b, err := s.expenses.Breakdown("", "")
	if err != nil {
		t.Fatalf("répartition : %v", err)
	}
	if len(b) != 2 {
		t.Fatalf("%d rubrique(s), attendu 2", len(b))
	}
	if b[0].Category != "Salaires" || b[0].Amount != 60000000 {
		t.Errorf("première rubrique = %s (%d), attendu Salaires (60000000)", b[0].Category, b[0].Amount)
	}
	if b[1].Amount != 40000000 || b[1].Count != 2 {
		t.Errorf("Loyer = %d en %d écriture(s), attendu 40000000 en 2", b[1].Amount, b[1].Count)
	}
	if b[0].Share != 60 {
		t.Errorf("part des salaires = %.1f %%, attendu 60 %%", b[0].Share)
	}
}

func TestExportsEtDocuments(t *testing.T) {
	s := newSuite(t)
	s.setTax(18)
	p := s.product("Ordinateur de bureau", 20000000, 32000000, 6)
	inv, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}}, Date: today(),
	})
	if err != nil {
		t.Fatalf("vente : %v", err)
	}

	pdf, err := s.docs.Invoice(inv.ID)
	if err != nil {
		t.Fatalf("PDF de facture : %v", err)
	}
	if len(pdf.Content) < 1000 || string(pdf.Content[:4]) != "%PDF" {
		t.Errorf("le fichier produit n'est pas un PDF (%d octets)", len(pdf.Content))
	}
	for _, gen := range []struct {
		nom string
		fn  func() (File, error)
	}{
		{"rapport de ventes", func() (File, error) { return s.docs.SalesReport(ReportQuery{}) }},
		{"compte de résultat", func() (File, error) { return s.docs.IncomeStatement("", "") }},
		{"état du stock", func() (File, error) { return s.docs.StockReport(ProductQuery{}) }},
	} {
		f, err := gen.fn()
		if err != nil {
			t.Errorf("%s : %v", gen.nom, err)
			continue
		}
		if len(f.Content) < 1000 {
			t.Errorf("%s : document suspect (%d octets)", gen.nom, len(f.Content))
		}
	}

	csv, err := s.export.InvoiceLines(InvoiceQuery{})
	if err != nil {
		t.Fatalf("export CSV : %v", err)
	}
	if len(csv.Content) < 50 {
		t.Errorf("export CSV vide (%d octets)", len(csv.Content))
	}
	if string(csv.Content[:3]) != "\ufeff" {
		t.Error("le CSV n'a pas de BOM : Excel l'ouvrira mal")
	}
}
