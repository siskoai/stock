package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/pdfgen"
	"comptoir/internal/storage"
)

// Documents produit les PDF imprimables. Le service se contente d'assembler des
// données déjà calculées et de les passer à pdfgen : la mise en page ne connaît
// rien du métier, le métier ne connaît rien du PDF.
type Documents struct{ core }

// NewDocuments construit le service d'impression.
func NewDocuments(db *storage.Database, a *auth.Service) *Documents {
	return &Documents{core{db: db, auth: a}}
}

func (s *Documents) money(v int64) string {
	return pdfgen.FormatMoney(v, s.db.Settings().Decimals)
}

// slug rend un fragment de nom de fichier propre.
func slug(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), " ", "_")
}

// Invoice rend une facture ou un devis.
func (s *Documents) Invoice(id string) (File, error) {
	if _, err := s.guard(""); err != nil {
		return File{}, err
	}
	inv, err := s.db.Invoices.Get(id)
	if err != nil {
		return File{}, err
	}
	data, err := pdfgen.Invoice(inv, s.db.Settings())
	if err != nil {
		return File{}, fmt.Errorf("génération du PDF : %w", err)
	}
	kind := "facture"
	if inv.Status == models.StatusDraft {
		kind = "devis"
	}
	return File{Name: fmt.Sprintf("%s_%s.pdf", kind, slug(inv.Number)), MIME: "application/pdf", Content: data}, nil
}

// Purchase rend un bon d'entrée.
func (s *Documents) Purchase(id string) (File, error) {
	if _, err := s.guard("purchases"); err != nil {
		return File{}, err
	}
	pu, err := s.db.Purchases.Get(id)
	if err != nil {
		return File{}, err
	}
	data, err := pdfgen.PurchaseNote(pu, s.db.Settings())
	if err != nil {
		return File{}, fmt.Errorf("génération du PDF : %w", err)
	}
	return File{Name: fmt.Sprintf("bon_entree_%s.pdf", slug(pu.Number)), MIME: "application/pdf", Content: data}, nil
}

// Movement rend le justificatif d'un mouvement de stock : bon de sortie, bon de
// retour, constat de défaut selon le type. C'est la pièce que l'on fait signer
// quand de la marchandise bouge sans qu'une facture l'accompagne.
func (s *Documents) Movement(id string) (File, error) {
	u, err := s.guard("")
	if err != nil {
		return File{}, err
	}
	m, err := s.db.Movements.Get(id)
	if err != nil {
		return File{}, err
	}
	// Le coût figure sur le bon : un vendeur ne doit pas l'y trouver.
	if !s.canSeeCost(u) {
		m.UnitCost = 0
	}
	data, err := pdfgen.MovementNote(m, s.db.Settings())
	if err != nil {
		return File{}, fmt.Errorf("génération du PDF : %w", err)
	}
	return File{
		Name:    fmt.Sprintf("mouvement_%s.pdf", slug(m.Ref)),
		MIME:    "application/pdf",
		Content: data,
	}, nil
}

// SalesReport rend le rapport périodique de ventes.
func (s *Documents) SalesReport(q ReportQuery) (File, error) {
	if _, err := s.guard("finance"); err != nil {
		return File{}, err
	}
	rep := Reports{s.core}
	r, err := rep.SalesReport(q)
	if err != nil {
		return File{}, err
	}
	sym := s.db.Settings().CurrencySymbol

	rows := make([][]string, 0, len(r.Points))
	for _, p := range r.Points {
		rows = append(rows, []string{
			p.Label, strconv.Itoa(p.InvoiceCount), s.money(p.RevenueHT),
			s.money(p.CostOfSales), s.money(p.GrossMargin),
			fmt.Sprintf("%.1f %%", p.MarginRate), s.money(p.Expenses), s.money(p.NetResult),
		})
	}
	t := r.Total
	doc := pdfgen.ReportDoc{
		Title:    "Rapport de ventes",
		Subtitle: "Chiffre d'affaires, marge et résultat par période. Les devis et les factures annulées sont exclus.",
		Period:   periodLabel(r.From, r.To),
		KPIs: []pdfgen.ReportKPI{
			{Label: "Chiffre d'affaires HT", Value: s.money(t.RevenueHT) + " " + sym,
				Hint: fmt.Sprintf("%d facture(s)", t.InvoiceCount)},
			{Label: "Marge brute", Value: s.money(t.GrossMargin) + " " + sym,
				Hint: fmt.Sprintf("%.1f %% du CA", t.MarginRate)},
			{Label: "Charges", Value: s.money(t.Expenses) + " " + sym,
				Hint: "hors coût des marchandises"},
			{Label: "Résultat", Value: s.money(t.NetResult) + " " + sym,
				Hint: fmt.Sprintf("panier moyen %s %s", s.money(r.AverageTicket), sym)},
		},
		Tables: []pdfgen.ReportTable{{
			Title: "Détail par période",
			Columns: []pdfgen.ReportColumn{
				{Header: "Période", Width: 30, Align: "L"},
				{Header: "Fact.", Width: 14, Align: "C"},
				{Header: "CA HT", Align: "R"},
				{Header: "Coût", Align: "R"},
				{Header: "Marge", Align: "R"},
				{Header: "Taux", Width: 16, Align: "R"},
				{Header: "Charges", Align: "R"},
				{Header: "Résultat", Align: "R"},
			},
			Rows: rows,
			TotalRow: []string{"TOTAL", strconv.Itoa(t.InvoiceCount), s.money(t.RevenueHT),
				s.money(t.CostOfSales), s.money(t.GrossMargin),
				fmt.Sprintf("%.1f %%", t.MarginRate), s.money(t.Expenses), s.money(t.NetResult)},
		}},
		Notes: []string{
			"Montants exprimés en " + sym + ", hors taxes sauf mention contraire.",
			"Le résultat correspond à la marge brute diminuée des charges d'exploitation et des pertes sur rebuts.",
		},
	}
	data, err := pdfgen.Report(doc, s.db.Settings())
	if err != nil {
		return File{}, fmt.Errorf("génération du PDF : %w", err)
	}
	return File{Name: "rapport_de_ventes.pdf", MIME: "application/pdf", Content: data}, nil
}

// IncomeStatement rend le compte de résultat d'une période.
func (s *Documents) IncomeStatement(from, to string) (File, error) {
	if _, err := s.guard("finance"); err != nil {
		return File{}, err
	}
	rep := Reports{s.core}
	st, err := rep.IncomeStatement(from, to)
	if err != nil {
		return File{}, err
	}
	sym := s.db.Settings().CurrencySymbol

	produits := [][]string{
		{"Ventes hors taxes", s.money(st.RevenueHT)},
		{"Remises accordées", "− " + s.money(st.DiscountsGiven)},
		{"Taxes facturées (à reverser)", s.money(st.TaxCollected)},
	}
	charges := make([][]string, 0, len(st.ExpenseLines)+2)
	charges = append(charges, []string{"Coût des marchandises vendues", s.money(st.CostOfSales)})
	for _, l := range st.ExpenseLines {
		charges = append(charges, []string{l.Category, s.money(l.Amount)})
	}
	if st.ScrapLoss > 0 {
		charges = append(charges, []string{"Pertes sur rebuts", s.money(st.ScrapLoss)})
	}

	doc := pdfgen.ReportDoc{
		Title:    "Compte de résultat",
		Subtitle: "Produits et charges de la période, sur la base des factures émises.",
		Period:   periodLabel(st.From, st.To),
		KPIs: []pdfgen.ReportKPI{
			{Label: "Chiffre d'affaires HT", Value: s.money(st.RevenueHT) + " " + sym,
				Hint: fmt.Sprintf("%d facture(s), %d article(s)", st.InvoiceCount, st.UnitsSold)},
			{Label: "Marge brute", Value: s.money(st.GrossMargin) + " " + sym,
				Hint: fmt.Sprintf("%.1f %% du CA", st.MarginRate)},
			{Label: "Total des charges", Value: s.money(st.TotalExpenses) + " " + sym,
				Hint: fmt.Sprintf("%d rubrique(s)", len(st.ExpenseLines))},
			{Label: "Résultat d'exploitation", Value: s.money(st.OperatingResult) + " " + sym,
				Hint: fmt.Sprintf("%.1f %% du CA", st.ResultRate)},
		},
		Tables: []pdfgen.ReportTable{
			{
				Title:    "Produits",
				Columns:  []pdfgen.ReportColumn{{Header: "Rubrique", Align: "L"}, {Header: "Montant", Width: 45, Align: "R"}},
				Rows:     produits,
				TotalRow: []string{"Chiffre d'affaires net HT", s.money(st.RevenueHT)},
			},
			{
				Title:    "Charges",
				Columns:  []pdfgen.ReportColumn{{Header: "Rubrique", Align: "L"}, {Header: "Montant", Width: 45, Align: "R"}},
				Rows:     charges,
				TotalRow: []string{"Total des charges", s.money(st.CostOfSales + st.TotalExpenses + st.ScrapLoss)},
			},
			{
				Title:   "Trésorerie",
				Note:    "Estimation : les achats fournisseurs sont supposés réglés à la réception. Comptoir ne suit pas les échéances fournisseurs.",
				Columns: []pdfgen.ReportColumn{{Header: "Flux", Align: "L"}, {Header: "Montant", Width: 45, Align: "R"}},
				Rows: [][]string{
					{"Encaissements clients", s.money(st.CashCollected)},
					{"Achats fournisseurs", "− " + s.money(st.PurchasesPaid)},
					{"Charges d'exploitation", "− " + s.money(st.TotalExpenses)},
				},
				TotalRow: []string{"Flux de trésorerie estimé", s.money(st.EstimatedCashFlow)},
			},
		},
		Notes: []string{
			"Résultat d'exploitation = marge brute − charges − pertes sur rebuts.",
			"Document de gestion interne : il ne remplace pas les états comptables établis par un professionnel.",
		},
	}
	data, err := pdfgen.Report(doc, s.db.Settings())
	if err != nil {
		return File{}, fmt.Errorf("génération du PDF : %w", err)
	}
	return File{Name: "compte_de_resultat.pdf", MIME: "application/pdf", Content: data}, nil
}

// StockReport rend l'état du stock, valorisé si le rôle y a droit.
func (s *Documents) StockReport(q ProductQuery) (File, error) {
	u, err := s.guard("")
	if err != nil {
		return File{}, err
	}
	cat := Catalog{s.core}
	products, err := cat.ListProducts(q)
	if err != nil {
		return File{}, err
	}
	stock := Stock{s.core}
	sum, err := stock.Summary()
	if err != nil {
		return File{}, err
	}
	showCost := s.canSeeCost(u)
	sym := s.db.Settings().CurrencySymbol

	cols := []pdfgen.ReportColumn{
		{Header: "Référence", Width: 30, Align: "L"},
		{Header: "Désignation", Align: "L"},
		{Header: "Catégorie", Width: 34, Align: "L"},
		{Header: "Stock", Width: 16, Align: "C"},
		{Header: "Déf.", Width: 13, Align: "C"},
		{Header: "PV", Width: 24, Align: "R"},
	}
	if showCost {
		cols = append(cols,
			pdfgen.ReportColumn{Header: "Coût", Width: 24, Align: "R"},
			pdfgen.ReportColumn{Header: "Valeur", Width: 26, Align: "R"})
	}

	rows := make([][]string, 0, len(products))
	for _, p := range products {
		row := []string{p.SKU, p.Name, p.CategoryName,
			strconv.Itoa(p.Quantity), strconv.Itoa(p.DefectiveQty), s.money(p.SalePrice)}
		if showCost {
			row = append(row, s.money(p.PurchasePrice), s.money(p.StockValue))
		}
		rows = append(rows, row)
	}
	total := []string{"TOTAL", fmt.Sprintf("%d article(s)", len(products)), "",
		strconv.Itoa(sum.TotalUnits), strconv.Itoa(sum.DefectiveUnits), ""}
	if showCost {
		total = append(total, "", s.money(sum.StockValueCost))
	}

	kpis := []pdfgen.ReportKPI{
		{Label: "Articles actifs", Value: strconv.Itoa(sum.ProductCount), Hint: fmt.Sprintf("%d catégorie(s)", sum.CategoryCount)},
		{Label: "Unités en stock", Value: strconv.Itoa(sum.TotalUnits), Hint: fmt.Sprintf("%d défectueuse(s)", sum.DefectiveUnits)},
		{Label: "Sous le seuil", Value: strconv.Itoa(sum.LowStockCount), Hint: fmt.Sprintf("%d en rupture", sum.OutOfStockCount)},
	}
	if showCost {
		kpis = append(kpis, pdfgen.ReportKPI{
			Label: "Valeur du stock", Value: s.money(sum.StockValueCost) + " " + sym,
			Hint: "au coût moyen pondéré"})
	}

	doc := pdfgen.ReportDoc{
		Title:    "État du stock",
		Subtitle: "Inventaire théorique à la date d'édition, tel qu'il ressort du journal des mouvements.",
		Period:   "Au " + timeNow().Format("02/01/2006"),
		KPIs:     kpis,
		Tables:   []pdfgen.ReportTable{{Title: "Articles", Columns: cols, Rows: rows, TotalRow: total}},
		Notes: []string{
			"Le stock défectueux est comptabilisé séparément et n'est jamais vendable.",
			"Un écart avec le comptage physique se corrige par un ajustement d'inventaire, qui laisse une trace.",
		},
	}
	data, err := pdfgen.Report(doc, s.db.Settings())
	if err != nil {
		return File{}, fmt.Errorf("génération du PDF : %w", err)
	}
	return File{Name: "etat_du_stock.pdf", MIME: "application/pdf", Content: data}, nil
}

// PartyStatement rend le relevé de compte d'un client : ses factures, ce qu'il
// a réglé et ce qu'il doit.
func (s *Documents) PartyStatement(partyID string) (File, error) {
	if _, err := s.guard(""); err != nil {
		return File{}, err
	}
	party, err := s.db.Parties.Get(partyID)
	if err != nil {
		return File{}, err
	}
	sym := s.db.Settings().CurrencySymbol

	rows := [][]string{}
	var total, paid, due int64
	for _, inv := range s.db.Invoices.All() {
		if inv.CustomerID != partyID || !countsAsSale(inv) {
			continue
		}
		rows = append(rows, []string{
			inv.Number, inv.Date.Format("02/01/2006"), statusLabel(inv.Status),
			s.money(inv.Total), s.money(inv.AmountPaid), s.money(inv.Balance),
		})
		total += inv.Total
		paid += inv.AmountPaid
		due += inv.Balance
	}

	doc := pdfgen.ReportDoc{
		Title:    "Relevé de compte",
		Subtitle: party.Name + phoneSuffix(party),
		Period:   "Au " + timeNow().Format("02/01/2006"),
		KPIs: []pdfgen.ReportKPI{
			{Label: "Total facturé", Value: s.money(total) + " " + sym, Hint: fmt.Sprintf("%d facture(s)", len(rows))},
			{Label: "Réglé", Value: s.money(paid) + " " + sym},
			{Label: "Reste dû", Value: s.money(due) + " " + sym, Hint: "toutes échéances confondues"},
		},
		Tables: []pdfgen.ReportTable{{
			Title: "Factures",
			Columns: []pdfgen.ReportColumn{
				{Header: "Numéro", Width: 34, Align: "L"},
				{Header: "Date", Width: 26, Align: "L"},
				{Header: "Statut", Align: "L"},
				{Header: "Total", Width: 30, Align: "R"},
				{Header: "Réglé", Width: 30, Align: "R"},
				{Header: "Solde", Width: 30, Align: "R"},
			},
			Rows:     rows,
			TotalRow: []string{"TOTAL", "", "", s.money(total), s.money(paid), s.money(due)},
		}},
		Notes: []string{"Les devis et les factures annulées ne figurent pas sur ce relevé."},
	}
	data, err := pdfgen.Report(doc, s.db.Settings())
	if err != nil {
		return File{}, fmt.Errorf("génération du PDF : %w", err)
	}
	return File{Name: "releve_" + slug(party.Name) + ".pdf", MIME: "application/pdf", Content: data}, nil
}

func phoneSuffix(p models.Party) string {
	if trim(p.Phone) == "" {
		return ""
	}
	return ", " + p.Phone
}

// periodLabel rend « Du 01/01/2026 au 31/03/2026 ».
func periodLabel(from, to time.Time) string {
	return fmt.Sprintf("Du %s au %s", from.Format("02/01/2006"), to.Format("02/01/2006"))
}
