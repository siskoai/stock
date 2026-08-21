package services

import (
	"fmt"
	"sort"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Reports produit les tableaux de bord, les rapports périodiques, le compte de
// résultat et les statistiques. Ce service ne modifie jamais de données.
type Reports struct{ core }

// NewReports construit le service de rapports.
func NewReports(db *storage.Database, a *auth.Service) *Reports {
	return &Reports{core{db: db, auth: a}}
}

// ---------------------------------------------------------------------------
// Découpage temporel
// ---------------------------------------------------------------------------

// Granularity est le pas d'agrégation d'un rapport.
type Granularity string

const (
	GranDay      Granularity = "day"
	GranWeek     Granularity = "week"
	GranMonth    Granularity = "month"
	GranQuarter  Granularity = "quarter"
	GranSemester Granularity = "semester"
	GranYear     Granularity = "year"
)

// bucket identifie la période d'agrégation à laquelle appartient une date.
// La clé est triable en tant que chaîne, le libellé est destiné à l'affichage.
func bucket(t time.Time, g Granularity) (key, label string) {
	switch g {
	case GranWeek:
		y, w := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w), fmt.Sprintf("S%02d %d", w, y)
	case GranMonth:
		return t.Format("2006-01"), fmt.Sprintf("%s %d", shortMonthFR(t.Month()), t.Year())
	case GranQuarter:
		q := (int(t.Month())-1)/3 + 1
		return fmt.Sprintf("%04d-Q%d", t.Year(), q), fmt.Sprintf("T%d %d", q, t.Year())
	case GranSemester:
		s := 1
		if int(t.Month()) > 6 {
			s = 2
		}
		return fmt.Sprintf("%04d-S%d", t.Year(), s), fmt.Sprintf("S%d %d", s, t.Year())
	case GranYear:
		return t.Format("2006"), t.Format("2006")
	default:
		return t.Format("2006-01-02"), t.Format("02/01")
	}
}

func shortMonthFR(m time.Month) string {
	names := [...]string{"janv.", "févr.", "mars", "avr.", "mai", "juin",
		"juil.", "août", "sept.", "oct.", "nov.", "déc."}
	return names[int(m)-1]
}

// advance passe au début de la période suivante, sert à générer les intervalles
// vides pour que les graphiques n'aient pas de trous.
func advance(t time.Time, g Granularity) time.Time {
	switch g {
	case GranWeek:
		return t.AddDate(0, 0, 7)
	case GranMonth:
		return t.AddDate(0, 1, 0)
	case GranQuarter:
		return t.AddDate(0, 3, 0)
	case GranSemester:
		return t.AddDate(0, 6, 0)
	case GranYear:
		return t.AddDate(1, 0, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}

func alignStart(t time.Time, g Granularity) time.Time {
	switch g {
	case GranWeek:
		// Semaine ISO : on recule jusqu'au lundi.
		offset := (int(t.Weekday()) + 6) % 7
		return util.StartOfDay(t.AddDate(0, 0, -offset))
	case GranMonth:
		return util.StartOfMonth(t)
	case GranQuarter:
		return util.StartOfQuarter(t)
	case GranSemester:
		return util.StartOfSemester(t)
	case GranYear:
		return util.StartOfYear(t)
	default:
		return util.StartOfDay(t)
	}
}

// ---------------------------------------------------------------------------
// Rapport de ventes périodique
// ---------------------------------------------------------------------------

// PeriodPoint est une ligne de rapport : un intervalle et ses agrégats.
type PeriodPoint struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	RevenueHT    int64   `json:"revenueHT"`  // chiffre d'affaires net hors taxe
	RevenueTTC   int64   `json:"revenueTTC"` // encaissable, taxes comprises
	TaxCollected int64   `json:"taxCollected"`
	CostOfSales  int64   `json:"costOfSales"`
	GrossMargin  int64   `json:"grossMargin"`
	Expenses     int64   `json:"expenses"`
	ScrapLoss    int64   `json:"scrapLoss"`
	NetResult    int64   `json:"netResult"`
	InvoiceCount int     `json:"invoiceCount"`
	UnitsSold    int     `json:"unitsSold"`
	Collected    int64   `json:"collected"`   // effectivement encaissé
	Outstanding  int64   `json:"outstanding"` // reste dû sur la période
	MarginRate   float64 `json:"marginRate"`  // marge brute / CA HT, en %
}

// SalesReport est le résultat d'un rapport périodique.
type SalesReport struct {
	Granularity      Granularity   `json:"granularity"`
	From             time.Time     `json:"from"`
	To               time.Time     `json:"to"`
	Points           []PeriodPoint `json:"points"`
	Total            PeriodPoint   `json:"total"`
	Best             *PeriodPoint  `json:"best,omitempty"`
	AveragePerPeriod int64         `json:"averagePerPeriod"`
	AverageTicket    int64         `json:"averageTicket"`
}

// ReportQuery décrit la période et le pas d'un rapport.
type ReportQuery struct {
	From        string      `json:"from"`
	To          string      `json:"to"`
	Granularity Granularity `json:"granularity"`
}

// SalesReport agrège les ventes, charges et pertes par intervalle.
// Les devis et les factures annulées sont exclus : seule une facture émise
// constitue du chiffre d'affaires.
func (s *Reports) SalesReport(q ReportQuery) (SalesReport, error) {
	if _, err := s.guard("finance"); err != nil {
		return SalesReport{}, err
	}
	return s.salesReport(q)
}

// salesReport est le calcul sans contrôle d'accès. Le tableau de bord s'en sert
// pour construire les courbes de chiffre d'affaires d'un vendeur, dont il retire
// ensuite les colonnes de coût, un vendeur voit ce qu'il vend, mais il le voit.
func (s *Reports) salesReport(q ReportQuery) (SalesReport, error) {
	gran := q.Granularity
	if gran == "" {
		gran = GranDay
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return SalesReport{}, err
	}
	if from.IsZero() {
		from = firstActivityDate(s.db, to)
	}

	// Squelette : un point par intervalle, y compris ceux sans activité.
	points := map[string]*PeriodPoint{}
	order := []string{}
	for cursor := alignStart(from, gran); !cursor.After(to); cursor = advance(cursor, gran) {
		key, label := bucket(cursor, gran)
		if _, exists := points[key]; !exists {
			points[key] = &PeriodPoint{Key: key, Label: label}
			order = append(order, key)
		}
	}
	at := func(t time.Time) *PeriodPoint {
		key, label := bucket(t, gran)
		p, ok := points[key]
		if !ok {
			p = &PeriodPoint{Key: key, Label: label}
			points[key] = p
			order = append(order, key)
		}
		return p
	}

	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) || !util.InRange(inv.Date, from, to) {
			continue
		}
		p := at(inv.Date)
		net := inv.SubtotalHT - inv.GlobalDiscount
		p.RevenueHT += net
		p.RevenueTTC += inv.Total
		p.TaxCollected += inv.TaxTotal
		p.CostOfSales += inv.CostTotal
		p.GrossMargin += net - inv.CostTotal
		p.Collected += inv.AmountPaid
		p.Outstanding += inv.Balance
		p.InvoiceCount++
		for _, l := range inv.Lines {
			p.UnitsSold += l.Quantity
		}
	}

	for _, e := range s.db.Expenses.All() {
		if util.InRange(e.Date, from, to) {
			at(e.Date).Expenses += e.Amount
		}
	}

	for _, m := range s.db.Movements.All() {
		if m.Type == models.MovementScrap && util.InRange(m.Date, from, to) {
			at(m.Date).ScrapLoss += int64(m.Quantity) * m.UnitCost
		}
	}

	sort.Strings(order)
	report := SalesReport{Granularity: gran, From: from, To: to}
	for _, key := range order {
		p := points[key]
		p.NetResult = p.GrossMargin - p.Expenses - p.ScrapLoss
		if p.RevenueHT > 0 {
			p.MarginRate = float64(p.GrossMargin) / float64(p.RevenueHT) * 100
		}
		report.Points = append(report.Points, *p)

		t := &report.Total
		t.RevenueHT += p.RevenueHT
		t.RevenueTTC += p.RevenueTTC
		t.TaxCollected += p.TaxCollected
		t.CostOfSales += p.CostOfSales
		t.GrossMargin += p.GrossMargin
		t.Expenses += p.Expenses
		t.ScrapLoss += p.ScrapLoss
		t.InvoiceCount += p.InvoiceCount
		t.UnitsSold += p.UnitsSold
		t.Collected += p.Collected
		t.Outstanding += p.Outstanding

		if report.Best == nil || p.RevenueHT > report.Best.RevenueHT {
			best := *p
			report.Best = &best
		}
	}

	report.Total.Key, report.Total.Label = "total", "Total"
	report.Total.NetResult = report.Total.GrossMargin - report.Total.Expenses - report.Total.ScrapLoss
	if report.Total.RevenueHT > 0 {
		report.Total.MarginRate = float64(report.Total.GrossMargin) / float64(report.Total.RevenueHT) * 100
	}
	if n := len(report.Points); n > 0 {
		report.AveragePerPeriod = report.Total.RevenueHT / int64(n)
	}
	if report.Total.InvoiceCount > 0 {
		report.AverageTicket = report.Total.RevenueTTC / int64(report.Total.InvoiceCount)
	}
	return report, nil
}

// countsAsSale exclut les devis et les factures annulées.
func countsAsSale(inv models.Invoice) bool {
	return inv.Status != models.StatusDraft && inv.Status != models.StatusCancelled
}

func firstActivityDate(db *storage.Database, fallback time.Time) time.Time {
	earliest := fallback
	for _, inv := range db.Invoices.All() {
		if inv.Date.Before(earliest) {
			earliest = inv.Date
		}
	}
	for _, e := range db.Expenses.All() {
		if e.Date.Before(earliest) {
			earliest = e.Date
		}
	}
	return util.StartOfDay(earliest)
}

// ---------------------------------------------------------------------------
// Tableau de bord
// ---------------------------------------------------------------------------

// SnapshotKPI est un indicateur avec sa variation par rapport à la période
// précédente de même durée.
type SnapshotKPI struct {
	Value    int64   `json:"value"`
	Previous int64   `json:"previous"`
	Change   float64 `json:"change"` // en pourcentage ; 0 si la base est nulle
	Count    int     `json:"count"`
}

// Dashboard rassemble tout ce qui s'affiche à l'ouverture de l'application.
type Dashboard struct {
	GeneratedAt time.Time `json:"generatedAt"`

	TodayRevenue  SnapshotKPI `json:"todayRevenue"`
	MonthRevenue  SnapshotKPI `json:"monthRevenue"`
	MonthMargin   SnapshotKPI `json:"monthMargin"`
	MonthExpenses SnapshotKPI `json:"monthExpenses"`
	MonthResult   SnapshotKPI `json:"monthResult"`

	StockValue       int64 `json:"stockValue"`
	StockUnits       int   `json:"stockUnits"`
	DefectiveUnits   int   `json:"defectiveUnits"`
	Outstanding      int64 `json:"outstanding"`
	OutstandingCount int   `json:"outstandingCount"`

	// Overdue isole, dans les impayés, ce dont l'échéance est dépassée. C'est
	// le chiffre qui appelle une action, là où le total appelle une surveillance.
	Overdue      int64 `json:"overdue"`
	OverdueCount int   `json:"overdueCount"`

	Last30Days      []PeriodPoint     `json:"last30Days"`
	Last12Months    []PeriodPoint     `json:"last12Months"`
	TopProducts     []ProductStat     `json:"topProducts"`
	LowStock        []ProductView     `json:"lowStock"`
	RecentInvoices  []models.Invoice  `json:"recentInvoices"`
	RecentMovements []models.Movement `json:"recentMovements"`
}

// Dashboard construit le tableau de bord d'accueil.
func (s *Reports) Dashboard() (Dashboard, error) {
	u, err := s.guard("")
	if err != nil {
		return Dashboard{}, err
	}
	now := time.Now()
	d := Dashboard{GeneratedAt: now}

	todayStart, todayEnd := util.StartOfDay(now), util.EndOfDay(now)
	yesterdayStart, yesterdayEnd := todayStart.AddDate(0, 0, -1), todayEnd.AddDate(0, 0, -1)
	monthStart := util.StartOfMonth(now)
	prevMonthStart := monthStart.AddDate(0, -1, 0)
	prevMonthEnd := monthStart.Add(-time.Nanosecond)

	todayAgg := s.aggregate(todayStart, todayEnd)
	yesterdayAgg := s.aggregate(yesterdayStart, yesterdayEnd)
	monthAgg := s.aggregate(monthStart, util.EndOfDay(now))
	prevMonthAgg := s.aggregate(prevMonthStart, prevMonthEnd)

	d.TodayRevenue = kpi(todayAgg.RevenueHT, yesterdayAgg.RevenueHT, todayAgg.InvoiceCount)
	d.MonthRevenue = kpi(monthAgg.RevenueHT, prevMonthAgg.RevenueHT, monthAgg.InvoiceCount)
	d.MonthMargin = kpi(monthAgg.GrossMargin, prevMonthAgg.GrossMargin, 0)
	d.MonthExpenses = kpi(monthAgg.Expenses, prevMonthAgg.Expenses, 0)
	d.MonthResult = kpi(monthAgg.NetResult, prevMonthAgg.NetResult, 0)

	for _, p := range s.db.Products.All() {
		if !p.Active {
			continue
		}
		d.StockValue += p.StockValue()
		d.StockUnits += p.Quantity
		d.DefectiveUnits += p.DefectiveQty
	}
	for _, inv := range s.db.Invoices.All() {
		if countsAsSale(inv) && inv.Balance > 0 {
			d.Outstanding += inv.Balance
			d.OutstandingCount++
			if retard, _ := classer(inv.DueDate, now); retard > 0 {
				d.Overdue += inv.Balance
				d.OverdueCount++
			}
		}
	}

	// Séries : 30 derniers jours et 12 derniers mois.
	if r, err := s.salesReport(ReportQuery{
		From:        now.AddDate(0, 0, -29).Format("2006-01-02"),
		To:          now.Format("2006-01-02"),
		Granularity: GranDay,
	}); err == nil {
		d.Last30Days = r.Points
	}
	if r, err := s.salesReport(ReportQuery{
		From:        util.StartOfMonth(now.AddDate(0, -11, 0)).Format("2006-01-02"),
		To:          now.Format("2006-01-02"),
		Granularity: GranMonth,
	}); err == nil {
		d.Last12Months = r.Points
	}

	if stats, err := s.TopProducts(ReportQuery{
		From: monthStart.Format("2006-01-02"), To: now.Format("2006-01-02"),
	}, 6); err == nil {
		d.TopProducts = stats
	}

	cat := Catalog{s.core}
	if low, err := cat.LowStockProducts(); err == nil {
		if len(low) > 8 {
			low = low[:8]
		}
		d.LowStock = low
	}

	sales := Sales{s.core}
	if inv, err := sales.ListInvoices(InvoiceQuery{Limit: 8}); err == nil {
		d.RecentInvoices = inv
	}
	stock := Stock{s.core}
	if mv, err := stock.ListMovements(MovementQuery{Limit: 10}); err == nil {
		d.RecentMovements = mv
	}

	// Un vendeur voit son activité, pas les indicateurs financiers de la
	// boutique. Le filtrage est appliqué ici, côté Go.
	if !auth.Can(u.Role, "finance") {
		d.MonthMargin, d.MonthExpenses, d.MonthResult = SnapshotKPI{}, SnapshotKPI{}, SnapshotKPI{}
		d.StockValue = 0
		for i := range d.Last30Days {
			d.Last30Days[i].CostOfSales, d.Last30Days[i].GrossMargin = 0, 0
			d.Last30Days[i].Expenses, d.Last30Days[i].NetResult = 0, 0
		}
		for i := range d.Last12Months {
			d.Last12Months[i].CostOfSales, d.Last12Months[i].GrossMargin = 0, 0
			d.Last12Months[i].Expenses, d.Last12Months[i].NetResult = 0, 0
		}
		for i := range d.TopProducts {
			d.TopProducts[i].Margin = 0
		}
	}
	return d, nil
}

func kpi(value, previous int64, count int) SnapshotKPI {
	k := SnapshotKPI{Value: value, Previous: previous, Count: count}
	if previous != 0 {
		k.Change = float64(value-previous) / absFloat(float64(previous)) * 100
	}
	return k
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// aggregate calcule les agrégats bruts d'un intervalle, sans découpage.
func (s *Reports) aggregate(from, to time.Time) PeriodPoint {
	var p PeriodPoint
	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) || !util.InRange(inv.Date, from, to) {
			continue
		}
		net := inv.SubtotalHT - inv.GlobalDiscount
		p.RevenueHT += net
		p.RevenueTTC += inv.Total
		p.TaxCollected += inv.TaxTotal
		p.CostOfSales += inv.CostTotal
		p.GrossMargin += net - inv.CostTotal
		p.Collected += inv.AmountPaid
		p.Outstanding += inv.Balance
		p.InvoiceCount++
		for _, l := range inv.Lines {
			p.UnitsSold += l.Quantity
		}
	}
	for _, e := range s.db.Expenses.All() {
		if util.InRange(e.Date, from, to) {
			p.Expenses += e.Amount
		}
	}
	for _, m := range s.db.Movements.All() {
		if m.Type == models.MovementScrap && util.InRange(m.Date, from, to) {
			p.ScrapLoss += int64(m.Quantity) * m.UnitCost
		}
	}
	p.NetResult = p.GrossMargin - p.Expenses - p.ScrapLoss
	if p.RevenueHT > 0 {
		p.MarginRate = float64(p.GrossMargin) / float64(p.RevenueHT) * 100
	}
	return p
}

// ---------------------------------------------------------------------------
// Compte de résultat et situation patrimoniale
// ---------------------------------------------------------------------------

// ExpenseLine est une rubrique de charge dans le compte de résultat.
type ExpenseLine struct {
	Category string  `json:"category"`
	Amount   int64   `json:"amount"`
	Share    float64 `json:"share"`
}

// IncomeStatement est le compte de résultat d'une période.
type IncomeStatement struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	RevenueHT      int64 `json:"revenueHT"`
	TaxCollected   int64 `json:"taxCollected"`
	RevenueTTC     int64 `json:"revenueTTC"`
	DiscountsGiven int64 `json:"discountsGiven"`

	CostOfSales int64   `json:"costOfSales"`
	GrossMargin int64   `json:"grossMargin"`
	MarginRate  float64 `json:"marginRate"`

	ExpenseLines  []ExpenseLine `json:"expenseLines"`
	TotalExpenses int64         `json:"totalExpenses"`
	ScrapLoss     int64         `json:"scrapLoss"`

	OperatingResult int64   `json:"operatingResult"`
	ResultRate      float64 `json:"resultRate"`

	InvoiceCount      int   `json:"invoiceCount"`
	UnitsSold         int   `json:"unitsSold"`
	AverageTicket     int64 `json:"averageTicket"`
	CashCollected     int64 `json:"cashCollected"`
	PurchasesPaid     int64 `json:"purchasesPaid"`
	EstimatedCashFlow int64 `json:"estimatedCashFlow"`
}

// IncomeStatement construit le compte de résultat d'une période.
//
// Le flux de trésorerie estimé suppose que les achats fournisseurs sont réglés
// à la réception, hypothèse courante en commerce de détail. Comptoir ne suit
// pas les échéances fournisseurs ; ce chiffre est donc une estimation, signalée
// comme telle dans l'interface.
func (s *Reports) IncomeStatement(from, to string) (IncomeStatement, error) {
	if _, err := s.guard("finance"); err != nil {
		return IncomeStatement{}, err
	}
	start, end, err := parseRange(from, to)
	if err != nil {
		return IncomeStatement{}, err
	}
	if start.IsZero() {
		start = firstActivityDate(s.db, end)
	}

	st := IncomeStatement{From: start, To: end}
	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) || !util.InRange(inv.Date, start, end) {
			continue
		}
		st.RevenueHT += inv.SubtotalHT - inv.GlobalDiscount
		st.TaxCollected += inv.TaxTotal
		st.RevenueTTC += inv.Total
		st.CostOfSales += inv.CostTotal
		st.CashCollected += inv.AmountPaid
		st.DiscountsGiven += inv.GlobalDiscount
		st.InvoiceCount++
		for _, l := range inv.Lines {
			st.DiscountsGiven += l.Discount
			st.UnitsSold += l.Quantity
		}
	}

	byCategory := map[string]int64{}
	for _, e := range s.db.Expenses.All() {
		if util.InRange(e.Date, start, end) {
			byCategory[e.Category] += e.Amount
			st.TotalExpenses += e.Amount
		}
	}
	for _, m := range s.db.Movements.All() {
		if m.Type == models.MovementScrap && util.InRange(m.Date, start, end) {
			st.ScrapLoss += int64(m.Quantity) * m.UnitCost
		}
	}
	for _, p := range s.db.Purchases.All() {
		if p.Status != models.StatusCancelled && util.InRange(p.Date, start, end) {
			st.PurchasesPaid += p.Total
		}
	}

	for cat, amount := range byCategory {
		line := ExpenseLine{Category: cat, Amount: amount}
		if st.TotalExpenses > 0 {
			line.Share = float64(amount) / float64(st.TotalExpenses) * 100
		}
		st.ExpenseLines = append(st.ExpenseLines, line)
	}
	sort.Slice(st.ExpenseLines, func(i, j int) bool { return st.ExpenseLines[i].Amount > st.ExpenseLines[j].Amount })

	st.GrossMargin = st.RevenueHT - st.CostOfSales
	st.OperatingResult = st.GrossMargin - st.TotalExpenses - st.ScrapLoss
	if st.RevenueHT > 0 {
		st.MarginRate = float64(st.GrossMargin) / float64(st.RevenueHT) * 100
		st.ResultRate = float64(st.OperatingResult) / float64(st.RevenueHT) * 100
	}
	if st.InvoiceCount > 0 {
		st.AverageTicket = st.RevenueTTC / int64(st.InvoiceCount)
	}
	st.EstimatedCashFlow = st.CashCollected - st.TotalExpenses - st.PurchasesPaid
	return st, nil
}

// BalanceSheet est une situation patrimoniale simplifiée, arrêtée à une date.
//
// Portée : Comptoir suit le stock, les créances clients et le résultat
// d'exploitation. Il ne tient pas de comptes bancaires, d'immobilisations ni de
// dettes fournisseurs, ce n'est pas un logiciel comptable certifié. Les
// rubriques absentes sont laissées explicites plutôt que fictives.
type BalanceSheet struct {
	AsOf time.Time `json:"asOf"`

	StockValueSound     int64 `json:"stockValueSound"`
	StockValueDefective int64 `json:"stockValueDefective"`
	Receivables         int64 `json:"receivables"`
	ReceivableCount     int   `json:"receivableCount"`
	TotalAssets         int64 `json:"totalAssets"`

	CumulativeRevenue     int64 `json:"cumulativeRevenue"`
	CumulativeCostOfSales int64 `json:"cumulativeCostOfSales"`
	CumulativeExpenses    int64 `json:"cumulativeExpenses"`
	CumulativeScrapLoss   int64 `json:"cumulativeScrapLoss"`
	CumulativeResult      int64 `json:"cumulativeResult"`

	TaxCollected int64 `json:"taxCollected"` // TVA facturée, à reverser

	StockUnits     int `json:"stockUnits"`
	DefectiveUnits int `json:"defectiveUnits"`
	ProductCount   int `json:"productCount"`
}

// BalanceSheet arrête la situation à une date donnée (vide = aujourd'hui).
func (s *Reports) BalanceSheet(asOf string) (BalanceSheet, error) {
	if _, err := s.guard("finance"); err != nil {
		return BalanceSheet{}, err
	}
	_, end, err := parseRange("", asOf)
	if err != nil {
		return BalanceSheet{}, err
	}
	b := BalanceSheet{AsOf: end}

	for _, p := range s.db.Products.All() {
		if !p.Active && p.Quantity == 0 && p.DefectiveQty == 0 {
			continue
		}
		b.ProductCount++
		b.StockUnits += p.Quantity
		b.DefectiveUnits += p.DefectiveQty
		b.StockValueSound += int64(p.Quantity) * p.PurchasePrice
		b.StockValueDefective += int64(p.DefectiveQty) * p.PurchasePrice
	}

	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) || inv.Date.After(end) {
			continue
		}
		if inv.Balance > 0 {
			b.Receivables += inv.Balance
			b.ReceivableCount++
		}
		b.CumulativeRevenue += inv.SubtotalHT - inv.GlobalDiscount
		b.CumulativeCostOfSales += inv.CostTotal
		b.TaxCollected += inv.TaxTotal
	}
	for _, e := range s.db.Expenses.All() {
		if !e.Date.After(end) {
			b.CumulativeExpenses += e.Amount
		}
	}
	for _, m := range s.db.Movements.All() {
		if m.Type == models.MovementScrap && !m.Date.After(end) {
			b.CumulativeScrapLoss += int64(m.Quantity) * m.UnitCost
		}
	}

	b.TotalAssets = b.StockValueSound + b.StockValueDefective + b.Receivables
	b.CumulativeResult = b.CumulativeRevenue - b.CumulativeCostOfSales - b.CumulativeExpenses - b.CumulativeScrapLoss
	return b, nil
}

// ---------------------------------------------------------------------------
// Statistiques
// ---------------------------------------------------------------------------

// ProductStat classe un produit selon son activité commerciale.
type ProductStat struct {
	ProductID  string  `json:"productId"`
	Name       string  `json:"name"`
	SKU        string  `json:"sku"`
	Category   string  `json:"category"`
	UnitsSold  int     `json:"unitsSold"`
	Revenue    int64   `json:"revenue"`
	Margin     int64   `json:"margin"`
	MarginRate float64 `json:"marginRate"`
	StockLeft  int     `json:"stockLeft"`
}

// TopProducts classe les produits par chiffre d'affaires sur une période.
func (s *Reports) TopProducts(q ReportQuery, limit int) ([]ProductStat, error) {
	u, err := s.guard("")
	if err != nil {
		return nil, err
	}
	showCost := s.canSeeCost(u)
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, err
	}
	cats := map[string]string{}
	for _, c := range s.db.Categories.All() {
		cats[c.ID] = c.Name
	}
	stock := map[string]models.Product{}
	for _, p := range s.db.Products.All() {
		stock[p.ID] = p
	}

	agg := map[string]*ProductStat{}
	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) || !util.InRange(inv.Date, from, to) {
			continue
		}
		for _, l := range inv.Lines {
			st, ok := agg[l.ProductID]
			if !ok {
				p := stock[l.ProductID]
				st = &ProductStat{
					ProductID: l.ProductID, Name: l.ProductName, SKU: l.SKU,
					Category: cats[p.CategoryID], StockLeft: p.Quantity,
				}
				if st.Category == "" {
					st.Category = "Sans catégorie"
				}
				agg[l.ProductID] = st
			}
			st.UnitsSold += l.Quantity
			st.Revenue += l.LineHT
			st.Margin += l.LineHT - int64(l.Quantity)*l.UnitCost
		}
	}

	out := make([]ProductStat, 0, len(agg))
	for _, st := range agg {
		if st.Revenue > 0 {
			st.MarginRate = float64(st.Margin) / float64(st.Revenue) * 100
		}
		if !showCost {
			st.Margin, st.MarginRate = 0, 0
		}
		out = append(out, *st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revenue > out[j].Revenue })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// NamedStat est un couple libellé / valeurs, utilisé par les répartitions.
type NamedStat struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Count  int     `json:"count"`
	Units  int     `json:"units"`
	Amount int64   `json:"amount"`
	Margin int64   `json:"margin"`
	Share  float64 `json:"share"`
}

// Statistics rassemble les répartitions d'une période.
type Statistics struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	ByCategory []NamedStat `json:"byCategory"`
	ByCustomer []NamedStat `json:"byCustomer"`
	ByPayment  []NamedStat `json:"byPayment"`
	ByMovement []NamedStat `json:"byMovement"`
	BySeller   []NamedStat `json:"bySeller"`
	ByWeekday  []NamedStat `json:"byWeekday"`

	TopProducts  []ProductStat `json:"topProducts"`
	SlowProducts []ProductStat `json:"slowProducts"`
}

// Statistics calcule toutes les répartitions d'une période.
func (s *Reports) Statistics(q ReportQuery) (Statistics, error) {
	u, err := s.guard("")
	if err != nil {
		return Statistics{}, err
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return Statistics{}, err
	}
	if from.IsZero() {
		from = firstActivityDate(s.db, to)
	}
	stats := Statistics{From: from, To: to}

	products := map[string]models.Product{}
	for _, p := range s.db.Products.All() {
		products[p.ID] = p
	}
	catNames := map[string]string{}
	for _, c := range s.db.Categories.All() {
		catNames[c.ID] = c.Name
	}

	byCategory := map[string]*NamedStat{}
	byCustomer := map[string]*NamedStat{}
	byPayment := map[string]*NamedStat{}
	bySeller := map[string]*NamedStat{}
	byWeekday := map[string]*NamedStat{}
	var totalRevenue int64

	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) || !util.InRange(inv.Date, from, to) {
			continue
		}
		net := inv.SubtotalHT - inv.GlobalDiscount
		totalRevenue += net

		add(byCustomer, keyOrDefault(inv.CustomerID, "walkin"), inv.CustomerName, net, inv.Margin, 1, 0)
		add(byPayment, string(inv.PaymentMethod), paymentLabel(inv.PaymentMethod), inv.Total, 0, 1, 0)
		add(bySeller, inv.UserID, inv.UserName, net, inv.Margin, 1, 0)

		wd := int(inv.Date.Weekday())
		add(byWeekday, fmt.Sprintf("%d", (wd+6)%7), weekdayFR(inv.Date.Weekday()), net, 0, 1, 0)

		for _, l := range inv.Lines {
			p := products[l.ProductID]
			catName := catNames[p.CategoryID]
			if catName == "" {
				catName = "Sans catégorie"
			}
			add(byCategory, p.CategoryID, catName, l.LineHT, l.LineHT-int64(l.Quantity)*l.UnitCost, 0, l.Quantity)
		}
	}

	byMovement := map[string]*NamedStat{}
	for _, m := range s.db.Movements.All() {
		if !util.InRange(m.Date, from, to) {
			continue
		}
		add(byMovement, string(m.Type), movementLabel(m.Type), int64(m.Quantity)*m.UnitCost, 0, 1, m.Quantity)
	}

	stats.ByCategory = finish(byCategory, totalRevenue, true)
	stats.ByCustomer = topN(finish(byCustomer, totalRevenue, true), 10)
	stats.ByPayment = finish(byPayment, 0, true)
	stats.ByMovement = finish(byMovement, 0, false)
	stats.BySeller = finish(bySeller, totalRevenue, true)
	stats.ByWeekday = finish(byWeekday, 0, false)

	// Un seul classement, découpé ensuite : le calcul balaie toutes les
	// factures de la période, il n'y a pas de raison de le faire deux fois.
	all, err := s.TopProducts(q, 0)
	if err != nil {
		return Statistics{}, err
	}
	stats.TopProducts = all
	if len(stats.TopProducts) > 10 {
		stats.TopProducts = stats.TopProducts[:10]
	}
	// Articles en stock qui n'ont rien vendu sur la période : de la trésorerie
	// immobilisée, c'est l'information la plus actionnable de la page.
	sold := map[string]bool{}
	for _, t := range all {
		sold[t.ProductID] = true
	}
	for _, p := range s.db.Products.All() {
		if p.Active && p.Quantity > 0 && !sold[p.ID] {
			stats.SlowProducts = append(stats.SlowProducts, ProductStat{
				ProductID: p.ID, Name: p.Name, SKU: p.SKU,
				Category:  defaultString(catNames[p.CategoryID], "Sans catégorie"),
				StockLeft: p.Quantity, Revenue: 0,
				Margin: int64(p.Quantity) * p.PurchasePrice, // capital immobilisé
			})
		}
	}
	sort.Slice(stats.SlowProducts, func(i, j int) bool { return stats.SlowProducts[i].Margin > stats.SlowProducts[j].Margin })
	if len(stats.SlowProducts) > 10 {
		stats.SlowProducts = stats.SlowProducts[:10]
	}

	if !auth.Can(u.Role, "finance") {
		for i := range stats.ByCategory {
			stats.ByCategory[i].Margin = 0
		}
		for i := range stats.ByCustomer {
			stats.ByCustomer[i].Margin = 0
		}
		for i := range stats.BySeller {
			stats.BySeller[i].Margin = 0
		}
		for i := range stats.ByMovement {
			stats.ByMovement[i].Amount = 0 // valorisé au coût d'achat
		}
		for i := range stats.TopProducts {
			stats.TopProducts[i].Margin, stats.TopProducts[i].MarginRate = 0, 0
		}
		for i := range stats.SlowProducts {
			stats.SlowProducts[i].Margin = 0
		}
	}
	return stats, nil
}

// ---------------------------------------------------------------------------

func add(m map[string]*NamedStat, key, label string, amount, margin int64, count, units int) {
	s, ok := m[key]
	if !ok {
		s = &NamedStat{Key: key, Label: label}
		m[key] = s
	}
	if s.Label == "" {
		s.Label = label
	}
	s.Amount += amount
	s.Margin += margin
	s.Count += count
	s.Units += units
}

func finish(m map[string]*NamedStat, total int64, sortByAmount bool) []NamedStat {
	out := make([]NamedStat, 0, len(m))
	for _, s := range m {
		if total > 0 {
			s.Share = float64(s.Amount) / float64(total) * 100
		}
		out = append(out, *s)
	}
	if sortByAmount {
		sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	}
	return out
}

func topN(list []NamedStat, n int) []NamedStat {
	if n > 0 && len(list) > n {
		return list[:n]
	}
	return list
}

func keyOrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func paymentLabel(m models.PaymentMethod) string {
	switch m {
	case models.PayCash:
		return "Espèces"
	case models.PayMobile:
		return "Mobile money"
	case models.PayTransfer:
		return "Virement"
	case models.PayCheck:
		return "Chèque"
	case models.PayCredit:
		return "Crédit"
	default:
		return string(m)
	}
}

func movementLabel(t models.MovementType) string {
	switch t {
	case models.MovementIn:
		return "Entrées"
	case models.MovementOut:
		return "Sorties"
	case models.MovementReturnCustomer:
		return "Retours clients"
	case models.MovementReturnSupplier:
		return "Retours fournisseurs"
	case models.MovementDefect:
		return "Défectueux"
	case models.MovementRepair:
		return "Réparations"
	case models.MovementScrap:
		return "Rebuts"
	case models.MovementAdjust:
		return "Inventaire"
	default:
		return string(t)
	}
}

func weekdayFR(d time.Weekday) string {
	names := [...]string{"Dimanche", "Lundi", "Mardi", "Mercredi", "Jeudi", "Vendredi", "Samedi"}
	return names[int(d)]
}
