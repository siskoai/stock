package services

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
)

// Export produit les fichiers destinés au tableur et le dépôt sur disque.
//
// Format retenu : CSV séparé par des points-virgules, encodé UTF-8 avec BOM et
// virgule décimale. C'est ce qu'Excel francophone ouvre d'un double-clic sans
// assistant d'importation. Les montants sont exportés en unités monétaires
// (et non en centièmes) : le fichier est destiné à être lu par une personne.
type Export struct{ core }

// NewExport construit le service d'export.
func NewExport(db *storage.Database, a *auth.Service) *Export {
	return &Export{core{db: db, auth: a}}
}

// File est un fichier produit par l'application, prêt à être enregistré.
// Content est transmis à l'interface en base64 par la sérialisation JSON.
type File struct {
	Name    string `json:"name"`
	MIME    string `json:"mime"`
	Content []byte `json:"content"`
}

// ---------------------------------------------------------------------------
// Écriture CSV
// ---------------------------------------------------------------------------

type sheet struct {
	buf  bytes.Buffer
	w    *csv.Writer
	dec  int
	rows int
}

func newSheet(decimals int) *sheet {
	s := &sheet{dec: decimals}
	s.buf.WriteString("\ufeff") // BOM : Excel reconnaît alors l'UTF-8
	s.w = csv.NewWriter(&s.buf)
	s.w.Comma = ';'
	return s
}

func (s *sheet) row(cells ...string) {
	_ = s.w.Write(cells)
	s.rows++
}

// money rend un montant en unités, avec virgule décimale.
func (s *sheet) money(amount int64) string {
	v := strconv.FormatFloat(float64(amount)/100, 'f', s.dec, 64)
	return strings.Replace(v, ".", ",", 1)
}

func (s *sheet) rate(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 2, 64), ".", ",", 1)
}

func date(t time.Time) string { return t.Format("02/01/2006") }

func (s *sheet) file(name string) File {
	s.w.Flush()
	return File{
		Name:    fmt.Sprintf("%s_%s.csv", name, timeNow().Format("2006-01-02")),
		MIME:    "text/csv",
		Content: s.buf.Bytes(),
	}
}

// ---------------------------------------------------------------------------
// Exports
// ---------------------------------------------------------------------------

// Products exporte l'inventaire. Les colonnes de coût ne sont écrites que si le
// rôle y a accès : un export ne doit pas contourner ce que l'écran masque.
func (s *Export) Products(q ProductQuery) (File, error) {
	u, err := s.guard("")
	if err != nil {
		return File{}, err
	}
	cat := Catalog{s.core}
	products, err := cat.ListProducts(q)
	if err != nil {
		return File{}, err
	}
	showCost := s.canSeeCost(u)

	sh := newSheet(s.db.Settings().Decimals)
	head := []string{"Référence", "Désignation", "Catégorie", "Marque", "Modèle",
		"Emplacement", "Unité", "Stock", "Défectueux", "Seuil d'alerte", "Prix de vente"}
	if showCost {
		head = append(head, "Coût moyen", "Valeur du stock", "Marge unitaire", "Taux de marge %")
	}
	sh.row(head...)

	for _, p := range products {
		row := []string{p.SKU, p.Name, p.CategoryName, p.Brand, p.Model,
			p.Location, p.Unit, strconv.Itoa(p.Quantity), strconv.Itoa(p.DefectiveQty),
			strconv.Itoa(p.MinStock), sh.money(p.SalePrice)}
		if showCost {
			row = append(row, sh.money(p.PurchasePrice), sh.money(p.StockValue),
				sh.money(p.MarginAmount), sh.rate(p.MarginRate))
		}
		sh.row(row...)
	}
	return sh.file("inventaire"), nil
}

// Invoices exporte les factures, une ligne par facture.
func (s *Export) Invoices(q InvoiceQuery) (File, error) {
	u, err := s.guard("")
	if err != nil {
		return File{}, err
	}
	sales := Sales{s.core}
	invoices, err := sales.ListInvoices(q)
	if err != nil {
		return File{}, err
	}
	showCost := s.canSeeCost(u)

	sh := newSheet(s.db.Settings().Decimals)
	head := []string{"Numéro", "Date", "Client", "Téléphone", "Statut", "Règlement",
		"Total HT", "Remise", "Taxes", "Total TTC", "Encaissé", "Solde", "Vendeur"}
	if showCost {
		head = append(head, "Coût des ventes", "Marge")
	}
	sh.row(head...)

	for _, inv := range invoices {
		row := []string{inv.Number, date(inv.Date), inv.CustomerName, inv.CustomerPhone,
			statusLabel(inv.Status), paymentLabel(inv.PaymentMethod),
			sh.money(inv.SubtotalHT), sh.money(inv.GlobalDiscount), sh.money(inv.TaxTotal),
			sh.money(inv.Total), sh.money(inv.AmountPaid), sh.money(inv.Balance), inv.UserName}
		if showCost {
			row = append(row, sh.money(inv.CostTotal), sh.money(inv.Margin))
		}
		sh.row(row...)
	}
	return sh.file("factures"), nil
}

// InvoiceLines exporte le détail ligne à ligne : c'est le fichier qui sert aux
// analyses croisées dans un tableur.
func (s *Export) InvoiceLines(q InvoiceQuery) (File, error) {
	u, err := s.guard("")
	if err != nil {
		return File{}, err
	}
	sales := Sales{s.core}
	invoices, err := sales.ListInvoices(q)
	if err != nil {
		return File{}, err
	}
	showCost := s.canSeeCost(u)

	sh := newSheet(s.db.Settings().Decimals)
	head := []string{"Numéro", "Date", "Client", "Référence", "Désignation",
		"Quantité", "Prix unitaire", "Remise", "Taux de taxe %", "Total HT", "Taxes", "Total TTC", "N° de série"}
	if showCost {
		head = append(head, "Coût unitaire", "Marge ligne")
	}
	sh.row(head...)

	for _, inv := range invoices {
		for _, l := range inv.Lines {
			row := []string{inv.Number, date(inv.Date), inv.CustomerName, l.SKU, l.ProductName,
				strconv.Itoa(l.Quantity), sh.money(l.UnitPrice), sh.money(l.Discount),
				sh.rate(l.TaxRate), sh.money(l.LineHT), sh.money(l.TaxAmount), sh.money(l.LineTTC),
				strings.Join(l.Serials, " ")}
			if showCost {
				row = append(row, sh.money(l.UnitCost), sh.money(l.LineHT-int64(l.Quantity)*l.UnitCost))
			}
			sh.row(row...)
		}
	}
	return sh.file("lignes_de_vente"), nil
}

// Movements exporte le journal de stock.
func (s *Export) Movements(q MovementQuery) (File, error) {
	u, err := s.guard("")
	if err != nil {
		return File{}, err
	}
	stock := Stock{s.core}
	movements, err := stock.ListMovements(q)
	if err != nil {
		return File{}, err
	}
	showCost := s.canSeeCost(u)

	sh := newSheet(s.db.Settings().Decimals)
	head := []string{"Référence", "Date", "Type", "Article", "Désignation", "Quantité",
		"Stock après", "Tiers", "Document", "Motif", "Opérateur"}
	if showCost {
		head = append(head, "Coût unitaire", "Valeur")
	}
	sh.row(head...)

	for _, m := range movements {
		row := []string{m.Ref, date(m.Date), movementLabel(m.Type), m.ProductSKU, m.ProductName,
			strconv.Itoa(m.Quantity), strconv.Itoa(m.StockAfter), m.PartyName, m.DocumentNo,
			m.Reason, m.UserName}
		if showCost {
			row = append(row, sh.money(m.UnitCost), sh.money(int64(m.Quantity)*m.UnitCost))
		}
		sh.row(row...)
	}
	return sh.file("mouvements_de_stock"), nil
}

// Expenses exporte les charges d'exploitation.
func (s *Export) Expenses(q ExpenseQuery) (File, error) {
	if _, err := s.guard("expenses"); err != nil {
		return File{}, err
	}
	exp := Expenses{s.core}
	list, err := exp.ListExpenses(q)
	if err != nil {
		return File{}, err
	}
	sh := newSheet(s.db.Settings().Decimals)
	sh.row("Date", "Rubrique", "Libellé", "Bénéficiaire", "Montant", "Règlement", "Saisi par", "Notes")
	for _, e := range list {
		sh.row(date(e.Date), e.Category, e.Label, e.Beneficiary, sh.money(e.Amount),
			paymentLabel(e.PaymentMethod), e.UserName, e.Notes)
	}
	return sh.file("charges"), nil
}

// Parties exporte les clients ou les fournisseurs.
func (s *Export) Parties(partyType string) (File, error) {
	if _, err := s.guard(""); err != nil {
		return File{}, err
	}
	cat := Catalog{s.core}
	list, err := cat.ListParties(partyType, "")
	if err != nil {
		return File{}, err
	}
	sh := newSheet(s.db.Settings().Decimals)
	sh.row("Nom", "Société", "Téléphone", "E-mail", "Adresse", "Ville", "NIF",
		"Documents", "Total facturé", "Impayés", "Dernière activité", "Actif")
	for _, p := range list {
		last := ""
		if p.LastActivity != nil {
			last = date(*p.LastActivity)
		}
		sh.row(p.Name, p.Company, p.Phone, p.Email, p.Address, p.City, p.TaxID,
			strconv.Itoa(p.DocumentCount), sh.money(p.TotalAmount), sh.money(p.OutstandingBalance),
			last, yesNo(p.Active))
	}
	name := "clients"
	if partyType == string(models.PartySupplier) {
		name = "fournisseurs"
	}
	return sh.file(name), nil
}

// SalesReport exporte un rapport périodique, un intervalle par ligne.
func (s *Export) SalesReport(q ReportQuery) (File, error) {
	if _, err := s.guard("finance"); err != nil {
		return File{}, err
	}
	rep := Reports{s.core}
	r, err := rep.SalesReport(q)
	if err != nil {
		return File{}, err
	}
	sh := newSheet(s.db.Settings().Decimals)
	sh.row("Période", "Factures", "Articles vendus", "CA HT", "Taxes", "CA TTC",
		"Coût des ventes", "Marge brute", "Taux de marge %", "Charges", "Pertes", "Résultat",
		"Encaissé", "Reste dû")
	for _, p := range r.Points {
		sh.row(p.Label, strconv.Itoa(p.InvoiceCount), strconv.Itoa(p.UnitsSold),
			sh.money(p.RevenueHT), sh.money(p.TaxCollected), sh.money(p.RevenueTTC),
			sh.money(p.CostOfSales), sh.money(p.GrossMargin), sh.rate(p.MarginRate),
			sh.money(p.Expenses), sh.money(p.ScrapLoss), sh.money(p.NetResult),
			sh.money(p.Collected), sh.money(p.Outstanding))
	}
	t := r.Total
	sh.row("TOTAL", strconv.Itoa(t.InvoiceCount), strconv.Itoa(t.UnitsSold),
		sh.money(t.RevenueHT), sh.money(t.TaxCollected), sh.money(t.RevenueTTC),
		sh.money(t.CostOfSales), sh.money(t.GrossMargin), sh.rate(t.MarginRate),
		sh.money(t.Expenses), sh.money(t.ScrapLoss), sh.money(t.NetResult),
		sh.money(t.Collected), sh.money(t.Outstanding))
	return sh.file("rapport_de_ventes"), nil
}

// ---------------------------------------------------------------------------
// Dépôt sur disque
// ---------------------------------------------------------------------------

// Save écrit un fichier produit dans le dossier exports/ et renvoie son chemin.
// Sert de repli lorsque l'utilisateur ne passe pas par un sélecteur natif.
func (s *Export) Save(f File) (string, error) {
	if _, err := s.guard(""); err != nil {
		return "", err
	}
	dir := filepath.Join(s.db.Dir, "exports")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(f.Name))
	if err := os.WriteFile(path, f.Content, 0o640); err != nil {
		return "", fmt.Errorf("écriture du fichier : %w", err)
	}
	return path, nil
}

// ---------------------------------------------------------------------------

func statusLabel(st models.DocStatus) string {
	switch st {
	case models.StatusDraft:
		return "Devis"
	case models.StatusIssued:
		return "Émise"
	case models.StatusPartial:
		return "Partiellement réglée"
	case models.StatusPaid:
		return "Réglée"
	case models.StatusCancelled:
		return "Annulée"
	default:
		return string(st)
	}
}

func yesNo(v bool) string {
	if v {
		return "oui"
	}
	return "non"
}
