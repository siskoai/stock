package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Sales gère les sorties de marchandise et la facturation. Une facture émise
// est la source unique du chiffre d'affaires : rien n'est compté comme vendu
// sans facture correspondante.
type Sales struct{ core }

// NewSales construit le service de vente.
func NewSales(db *storage.Database, a *auth.Service) *Sales {
	return &Sales{core{db: db, auth: a}}
}

// InvoiceLineInput est une ligne de facture saisie dans l'interface.
type InvoiceLineInput struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unitPrice"` // laissé à 0 : le prix catalogue s'applique
	Discount  int64  `json:"discount"`

	// TaxRate distingue « non renseigné » de « zéro ». Absent, le taux par
	// défaut des paramètres s'applique ; à 0, la ligne est exonérée, une
	// exonération doit pouvoir s'exprimer.
	TaxRate *float64 `json:"taxRate"`

	// Serials porte les numéros de série lorsque l'article est suivi à l'unité.
	Serials []string `json:"serials"`
}

// InvoiceInput est la charge utile de création d'une facture.
type InvoiceInput struct {
	Date  string             `json:"date"`
	Lines []InvoiceLineInput `json:"lines"`

	// Client enregistré. Laissé vide, les champs libres ci-dessous sont
	// utilisés, le cas courant d'une vente au comptoir.
	CustomerID      string `json:"customerId"`
	CustomerName    string `json:"customerName"`
	CustomerPhone   string `json:"customerPhone"`
	CustomerAddress string `json:"customerAddress"`
	CustomerTaxID   string `json:"customerTaxId"`

	GlobalDiscount int64                `json:"globalDiscount"`
	AmountPaid     int64                `json:"amountPaid"`
	PaymentMethod  models.PaymentMethod `json:"paymentMethod"`
	Notes          string               `json:"notes"`

	// Draft enregistre un devis : rien n'est déduit du stock, aucun chiffre
	// d'affaires n'est comptabilisé tant que la facture n'est pas émise. Les
	// montants, eux, sont calculés comme sur une facture, un devis sans TVA
	// n'engagerait à rien.
	Draft bool `json:"draft"`
}

// InvoiceQuery filtre la liste des factures.
type InvoiceQuery struct {
	Search     string `json:"search"`
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
	From       string `json:"from"`
	To         string `json:"to"`
	OnlyUnpaid bool   `json:"onlyUnpaid"`
	Limit      int    `json:"limit"`
}

// ListInvoices renvoie les factures, de la plus récente à la plus ancienne.
func (s *Sales) ListInvoices(q InvoiceQuery) ([]models.Invoice, error) {
	u, err := s.guard("")
	if err != nil {
		return nil, err
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, err
	}
	showCost := s.canSeeCost(u)
	out := []models.Invoice{}
	for _, inv := range s.db.Invoices.All() {
		if q.CustomerID != "" && inv.CustomerID != q.CustomerID {
			continue
		}
		if q.Status != "" && string(inv.Status) != q.Status {
			continue
		}
		if q.OnlyUnpaid && (inv.Balance <= 0 || inv.Status == models.StatusCancelled) {
			continue
		}
		if !util.InRange(inv.Date, from, to) {
			continue
		}
		if q.Search != "" && !(util.Contains(inv.Number, q.Search) ||
			util.Contains(inv.CustomerName, q.Search) ||
			util.Contains(inv.CustomerPhone, q.Search) ||
			lineMatches(inv.Lines, q.Search)) {
			continue
		}
		out = append(out, redactInvoice(inv, showCost))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// redactInvoice retire le coût de revient et la marge quand le rôle n'y a pas
// accès. Le document reste complet par ailleurs : un vendeur doit voir ce qu'il
// a facturé.
func redactInvoice(inv models.Invoice, showCost bool) models.Invoice {
	if showCost {
		return inv
	}
	inv.CostTotal, inv.Margin = 0, 0
	lines := make([]models.DocLine, len(inv.Lines))
	copy(lines, inv.Lines)
	for i := range lines {
		lines[i].UnitCost = 0
	}
	inv.Lines = lines
	return inv
}

func lineMatches(lines []models.DocLine, search string) bool {
	for _, l := range lines {
		if util.Contains(l.ProductName, search) || util.Contains(l.SKU, search) {
			return true
		}
		for _, sn := range l.Serials {
			if util.Contains(sn, search) {
				return true
			}
		}
	}
	return false
}

// GetInvoice renvoie une facture complète.
func (s *Sales) GetInvoice(id string) (models.Invoice, error) {
	u, err := s.guard("")
	if err != nil {
		return models.Invoice{}, err
	}
	inv, err := s.db.Invoices.Get(id)
	if err != nil {
		return models.Invoice{}, err
	}
	return redactInvoice(inv, s.canSeeCost(u)), nil
}

// buildLines valide la saisie et construit les lignes du document. Les prix,
// les taux et les coûts sont pris à la source : ce que l'interface envoie ne
// sert qu'à désigner l'article, la quantité et l'éventuelle remise.
func (s *Sales) buildLines(in InvoiceInput, settings models.Settings) ([]models.DocLine, map[string]int, error) {
	docLines := make([]models.DocLine, len(in.Lines))
	needed := map[string]int{} // cumul par produit : deux lignes du même article

	for i, l := range in.Lines {
		if l.Quantity <= 0 {
			return nil, nil, fmt.Errorf("ligne %d : la quantité doit être supérieure à zéro", i+1)
		}
		if l.UnitPrice < 0 || l.Discount < 0 {
			return nil, nil, fmt.Errorf("ligne %d : les montants ne peuvent pas être négatifs", i+1)
		}
		p, err := s.db.Products.Get(l.ProductID)
		if err != nil {
			return nil, nil, fmt.Errorf("ligne %d : produit introuvable", i+1)
		}
		if !p.Active {
			return nil, nil, fmt.Errorf("ligne %d : « %s » est archivé et ne peut plus être vendu", i+1, p.Name)
		}
		price := l.UnitPrice
		if price <= 0 {
			price = p.SalePrice
		}
		taxRate := settings.DefaultTaxRate
		if l.TaxRate != nil {
			taxRate = *l.TaxRate
		}
		if taxRate < 0 || taxRate > 100 {
			return nil, nil, fmt.Errorf("ligne %d : taux de taxe invalide (%.2f %%)", i+1, taxRate)
		}
		serials, err := checkSerials(p, l.Quantity, l.Serials)
		if err != nil {
			return nil, nil, fmt.Errorf("ligne %d : %w", i+1, err)
		}

		needed[p.ID] += l.Quantity
		docLines[i] = models.DocLine{
			ProductID: p.ID, ProductName: p.Name, SKU: p.SKU,
			Quantity: l.Quantity, UnitPrice: price, UnitCost: p.PurchasePrice,
			Discount: l.Discount, TaxRate: taxRate, Serials: serials,
		}
	}
	return docLines, needed, nil
}

// checkSerials valide les numéros de série d'une ligne : un article suivi à
// l'unité en accepte zéro (saisie différée) ou exactement un par unité, tous
// distincts ; un article non suivi n'en accepte aucun.
func checkSerials(p models.Product, qty int, serials []string) ([]string, error) {
	clean := make([]string, 0, len(serials))
	for _, sn := range serials {
		if v := trim(sn); v != "" {
			clean = append(clean, v)
		}
	}
	if len(clean) == 0 {
		return nil, nil
	}
	if !p.Serialized {
		return nil, fmt.Errorf("« %s » n'est pas suivi par numéro de série", p.Name)
	}
	if len(clean) != qty {
		return nil, fmt.Errorf("%d numéro(s) de série pour %d unité(s) de « %s »", len(clean), qty, p.Name)
	}
	seen := map[string]bool{}
	for _, sn := range clean {
		key := strings.ToUpper(sn)
		if seen[key] {
			return nil, fmt.Errorf("le numéro de série « %s » est saisi deux fois", sn)
		}
		seen[key] = true
	}
	return clean, nil
}

// checkAvailability vérifie le stock sur le cumul par produit, avant toute
// écriture : deux lignes du même article doivent tenir ensemble.
func (s *Sales) checkAvailability(needed map[string]int) error {
	for productID, qty := range needed {
		p, err := s.db.Products.Get(productID)
		if err != nil {
			return fmt.Errorf("un article de la facture n'existe plus")
		}
		if p.Quantity < qty {
			return fmt.Errorf("stock insuffisant pour « %s » : %d disponible(s), %d demandé(s)",
				p.Name, p.Quantity, qty)
		}
	}
	return nil
}

// outOps construit les opérations de sortie de stock d'une facture.
func outOps(inv models.Invoice, date time.Time, reason string) []stockOp {
	ops := make([]stockOp, 0, len(inv.Lines))
	for _, l := range inv.Lines {
		ops = append(ops, stockOp{
			ProductID: l.ProductID,
			Delta:     stockDelta{Sound: -l.Quantity},
			Movement: models.Movement{
				Type: models.MovementOut, Date: date, Quantity: l.Quantity,
				UnitCost: l.UnitCost, UnitPrice: l.UnitPrice,
				PartyID: inv.CustomerID, PartyName: inv.CustomerName,
				DocumentID: inv.ID, DocumentNo: inv.Number,
				Reason: reason,
				Notes:  serialNote(l.Serials),
			},
		})
	}
	return ops
}

func serialNote(serials []string) string {
	if len(serials) == 0 {
		return ""
	}
	return "N° de série : " + strings.Join(serials, ", ")
}

// CreateInvoice émet une facture de vente et sort la marchandise du stock.
//
// Déroulé : validation intégrale, réservation du numéro, écriture du document,
// puis application du stock en un seul lot. Si le lot échoue, le document est
// retiré et son numéro rendu, jamais de facture avec un stock partiellement
// déduit, jamais de trou dans la numérotation.
func (s *Sales) CreateInvoice(in InvoiceInput) (models.Invoice, error) {
	u, err := s.guard("sales")
	if err != nil {
		return models.Invoice{}, err
	}
	if len(in.Lines) == 0 {
		return models.Invoice{}, fmt.Errorf("ajoutez au moins un article à la facture")
	}
	date, err := parseDate(in.Date)
	if err != nil {
		return models.Invoice{}, err
	}
	settings := s.db.Settings()

	docLines, needed, err := s.buildLines(in, settings)
	if err != nil {
		return models.Invoice{}, err
	}
	if !in.Draft {
		if err := s.checkAvailability(needed); err != nil {
			return models.Invoice{}, err
		}
	}

	docLines, subtotalHT, taxTotal, total := computeTotals(docLines, in.GlobalDiscount, settings.PricesIncludeTax)

	var costTotal int64
	for _, l := range docLines {
		costTotal += int64(l.Quantity) * l.UnitCost
	}

	// Identité du client.
	customerName := trim(in.CustomerName)
	inv := models.Invoice{
		CustomerID:      in.CustomerID,
		CustomerPhone:   trim(in.CustomerPhone),
		CustomerAddress: trim(in.CustomerAddress),
		CustomerTaxID:   trim(in.CustomerTaxID),
	}
	if in.CustomerID != "" {
		c, err := s.db.Parties.Get(in.CustomerID)
		if err != nil || c.Type != models.PartyCustomer {
			return models.Invoice{}, fmt.Errorf("client introuvable")
		}
		customerName = c.Name
		inv.CustomerPhone = defaultString(inv.CustomerPhone, c.Phone)
		inv.CustomerAddress = defaultString(inv.CustomerAddress, c.Address)
		inv.CustomerTaxID = defaultString(inv.CustomerTaxID, c.TaxID)
	}
	if customerName == "" {
		customerName = "Client comptoir"
	}

	// Règlement.
	paid := maxInt64(in.AmountPaid, 0)
	if paid > total {
		paid = total
	}
	status := models.StatusIssued
	switch {
	case in.Draft:
		status = models.StatusDraft
		paid = 0
	case paid >= total && total > 0:
		status = models.StatusPaid
	case paid > 0:
		status = models.StatusPartial
	}

	number, err := s.db.NextInvoiceNumber()
	if err != nil {
		return models.Invoice{}, err
	}

	now := time.Now()
	inv.ID = util.NewID("inv")
	inv.Number = number
	inv.Date = date
	inv.CustomerName = customerName
	inv.Lines = docLines
	inv.SubtotalHT = subtotalHT
	inv.GlobalDiscount = in.GlobalDiscount
	inv.TaxTotal = taxTotal
	inv.Total = total
	inv.AmountPaid = paid
	inv.Balance = total - paid
	inv.CostTotal = costTotal
	inv.Margin = subtotalHT - in.GlobalDiscount - costTotal
	inv.PaymentMethod = defaultPayment(in.PaymentMethod)
	inv.Status = status
	inv.Notes = trim(in.Notes)
	inv.UserID, inv.UserName = u.ID, u.FullName
	inv.CreatedAt, inv.UpdatedAt = now, now

	if err := s.db.Invoices.Insert(inv); err != nil {
		s.db.ReleaseInvoiceNumber(number)
		return models.Invoice{}, err
	}

	if !in.Draft {
		if _, err := s.applyStock(u, outOps(inv, date, "Vente")); err != nil {
			_ = s.db.Invoices.Delete(inv.ID)
			s.db.ReleaseInvoiceNumber(number)
			return models.Invoice{}, err
		}
	}

	kind := "Facture"
	if in.Draft {
		kind = "Devis"
	}
	s.log(u, "CREATE", "invoice", inv.ID, "%s %s, %s, %d ligne(s), total %d, marge %d",
		kind, inv.Number, inv.CustomerName, len(docLines), inv.Total, inv.Margin)
	return redactInvoice(inv, s.canSeeCost(u)), nil
}

// IssueDraft convertit un devis en facture : le stock est déduit et le chiffre
// d'affaires devient effectif.
//
// Les prix de vente restent ceux du devis, c'est un engagement pris envers le
// client. Les coûts, eux, sont réalignés sur le coût moyen du jour : la marge
// doit refléter ce que la marchandise a réellement coûté au moment où elle sort.
func (s *Sales) IssueDraft(id string) (models.Invoice, error) {
	u, err := s.guard("sales")
	if err != nil {
		return models.Invoice{}, err
	}
	inv, err := s.db.Invoices.Get(id)
	if err != nil {
		return models.Invoice{}, err
	}
	if inv.Status != models.StatusDraft {
		return models.Invoice{}, fmt.Errorf("la facture %s est déjà émise", inv.Number)
	}

	needed := map[string]int{}
	for _, l := range inv.Lines {
		needed[l.ProductID] += l.Quantity
	}
	if err := s.checkAvailability(needed); err != nil {
		return models.Invoice{}, err
	}

	lines := make([]models.DocLine, len(inv.Lines))
	copy(lines, inv.Lines)
	var costTotal int64
	for i := range lines {
		if p, err := s.db.Products.Get(lines[i].ProductID); err == nil {
			lines[i].UnitCost = p.PurchasePrice
		}
		costTotal += int64(lines[i].Quantity) * lines[i].UnitCost
	}
	inv.Lines = lines
	inv.CostTotal = costTotal
	inv.Margin = inv.SubtotalHT - inv.GlobalDiscount - costTotal

	if _, err := s.applyStock(u, outOps(inv, time.Now(), "Vente (devis confirmé)")); err != nil {
		return models.Invoice{}, err
	}

	inv.Status = models.StatusIssued
	if inv.AmountPaid >= inv.Total && inv.Total > 0 {
		inv.Status = models.StatusPaid
	} else if inv.AmountPaid > 0 {
		inv.Status = models.StatusPartial
	}
	inv.UpdatedAt = time.Now()
	if err := s.db.Invoices.Update(inv); err != nil {
		return models.Invoice{}, err
	}
	s.log(u, "ISSUE", "invoice", inv.ID, "Devis %s converti en facture, total %d, marge %d",
		inv.Number, inv.Total, inv.Margin)
	return redactInvoice(inv, s.canSeeCost(u)), nil
}

// PaymentInput enregistre un règlement partiel ou total.
type PaymentInput struct {
	InvoiceID string               `json:"invoiceId"`
	Amount    int64                `json:"amount"`
	Method    models.PaymentMethod `json:"method"`
	Note      string               `json:"note"`
}

// RegisterPayment ajoute un règlement à une facture et met à jour son solde.
func (s *Sales) RegisterPayment(in PaymentInput) (models.Invoice, error) {
	u, err := s.guard("sales")
	if err != nil {
		return models.Invoice{}, err
	}
	inv, err := s.db.Invoices.Get(in.InvoiceID)
	if err != nil {
		return models.Invoice{}, err
	}
	if inv.Status == models.StatusCancelled {
		return models.Invoice{}, fmt.Errorf("la facture %s est annulée", inv.Number)
	}
	if inv.Status == models.StatusDraft {
		return models.Invoice{}, fmt.Errorf("émettez d'abord le devis %s", inv.Number)
	}
	if in.Amount <= 0 {
		return models.Invoice{}, fmt.Errorf("le montant réglé doit être supérieur à zéro")
	}
	if in.Amount > inv.Balance {
		return models.Invoice{}, fmt.Errorf("le montant dépasse le solde dû (%d)", inv.Balance)
	}

	inv.AmountPaid += in.Amount
	inv.Balance = inv.Total - inv.AmountPaid
	if inv.Balance <= 0 {
		inv.Status = models.StatusPaid
		inv.Balance = 0
	} else {
		inv.Status = models.StatusPartial
	}
	inv.PaymentMethod = defaultPayment(in.Method)
	inv.UpdatedAt = time.Now()
	if trim(in.Note) != "" {
		inv.Notes = appendNote(inv.Notes, fmt.Sprintf("%s, règlement %d : %s",
			time.Now().Format("02/01/2006"), in.Amount, trim(in.Note)))
	}
	if err := s.db.Invoices.Update(inv); err != nil {
		return models.Invoice{}, err
	}
	s.log(u, "PAYMENT", "invoice", inv.ID, "Règlement de %d sur la facture %s, solde %d",
		in.Amount, inv.Number, inv.Balance)
	return redactInvoice(inv, s.canSeeCost(u)), nil
}

// CancelInvoice annule une facture et remet la marchandise en stock.
// La facture est conservée avec le statut « Annulée » : une numérotation ne
// doit jamais comporter de trou.
//
// Un acompte déjà encaissé n'est pas effacé silencieusement : il devient un
// montant à rembourser (RefundDue), visible dans la liste des factures.
func (s *Sales) CancelInvoice(id, reason string) error {
	u, err := s.guard("sales")
	if err != nil {
		return err
	}
	inv, err := s.db.Invoices.Get(id)
	if err != nil {
		return err
	}
	if inv.Status == models.StatusCancelled {
		return fmt.Errorf("la facture %s est déjà annulée", inv.Number)
	}
	motif, err := requireText(reason, "Motif d'annulation")
	if err != nil {
		return err
	}

	if inv.Status != models.StatusDraft {
		ops := make([]stockOp, 0, len(inv.Lines))
		for _, l := range inv.Lines {
			ops = append(ops, stockOp{
				ProductID: l.ProductID,
				Delta:     stockDelta{Sound: l.Quantity},
				Movement: models.Movement{
					Type: models.MovementReturnCustomer, Date: time.Now(), Quantity: l.Quantity,
					UnitCost: l.UnitCost, UnitPrice: l.UnitPrice,
					PartyID: inv.CustomerID, PartyName: inv.CustomerName,
					DocumentID: inv.ID, DocumentNo: inv.Number,
					Reason: "Annulation de facture, " + motif,
					Notes:  serialNote(l.Serials),
				},
			})
		}
		if _, err := s.applyStock(u, ops); err != nil {
			return err
		}
	}

	now := time.Now()
	refund := inv.AmountPaid
	inv.Status = models.StatusCancelled
	inv.CancelledAt = &now
	inv.Balance = 0
	inv.RefundDue = refund
	inv.UpdatedAt = now
	inv.Notes = appendNote(inv.Notes, fmt.Sprintf("Annulée le %s par %s, %s",
		now.Format("02/01/2006"), u.FullName, motif))
	if refund > 0 {
		inv.Notes = appendNote(inv.Notes, fmt.Sprintf("Acompte de %d encaissé : à rembourser au client.", refund))
	}
	if err := s.db.Invoices.Update(inv); err != nil {
		return err
	}
	s.log(u, "CANCEL", "invoice", inv.ID, "Facture %s annulée, %s (à rembourser : %d)",
		inv.Number, motif, refund)
	return nil
}

// DeleteDraft supprime un devis non émis. Une facture émise n'est jamais
// supprimée, seulement annulée.
func (s *Sales) DeleteDraft(id string) error {
	u, err := s.guard("sales")
	if err != nil {
		return err
	}
	inv, err := s.db.Invoices.Get(id)
	if err != nil {
		return err
	}
	if inv.Status != models.StatusDraft {
		return fmt.Errorf("seul un devis peut être supprimé ; la facture %s doit être annulée", inv.Number)
	}
	if err := s.db.Invoices.Delete(id); err != nil {
		return err
	}
	s.log(u, "DELETE", "invoice", id, "Devis %s supprimé", inv.Number)
	return nil
}

// ---------------------------------------------------------------------------

func defaultPayment(m models.PaymentMethod) models.PaymentMethod {
	if m == "" {
		return models.PayCash
	}
	return m
}
