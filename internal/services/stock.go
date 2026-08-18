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

// Stock gère les mouvements de marchandise : retours, produits défectueux,
// réparations, rebuts et corrections d'inventaire. Les entrées d'achat sont
// dans le service Purchases, les sorties de vente dans le service Sales, mais
// tous écrivent dans le même journal de mouvements.
type Stock struct{ core }

// NewStock construit le service de mouvements.
func NewStock(db *storage.Database, a *auth.Service) *Stock {
	return &Stock{core{db: db, auth: a}}
}

// ---------------------------------------------------------------------------
// Écriture d'un mouvement (utilisé aussi par Sales et Purchases)
// ---------------------------------------------------------------------------

// stockDelta décrit l'effet d'un mouvement sur les deux compteurs d'un produit.
type stockDelta struct {
	Sound     int // variation du stock sain
	Defective int // variation du stock défectueux
}

// stockOp est une opération élémentaire d'un lot : une variation de stock et le
// mouvement qui la justifie.
type stockOp struct {
	ProductID string
	Delta     stockDelta
	Movement  models.Movement

	// Revalue, s'il est fourni, ajuste la fiche produit (coût moyen, prix de
	// vente) dans la même écriture que la variation de stock. Il est appelé
	// avant l'application du delta : p porte donc les quantités d'avant
	// l'opération, ce qu'attend le calcul du coût moyen pondéré.
	Revalue func(p *models.Product)
}

// applyStock applique un lot d'opérations de stock. Tout le lot passe ou rien
// ne passe, et l'ensemble ne coûte qu'une écriture de products.json et une de
// movements.json, quel que soit le nombre de lignes du document.
//
// Les produits sont relus à l'intérieur de la transaction : deux lignes du même
// article dans un même document s'additionnent correctement au lieu de s'écraser.
func (c *core) applyStock(u models.User, ops []stockOp) ([]models.Movement, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	refs, err := c.db.NextMovementRefs(len(ops))
	if err != nil {
		return nil, err
	}

	var written []models.Movement
	before := map[string]models.Product{} // état d'avant, pour la compensation

	err = c.db.Products.Transaction(func(items []models.Product) ([]models.Product, error) {
		index := make(map[string]int, len(items))
		for i, p := range items {
			index[p.ID] = i
		}
		written = make([]models.Movement, 0, len(ops))
		now := time.Now()

		for i, op := range ops {
			idx, ok := index[op.ProductID]
			if !ok {
				return nil, fmt.Errorf("produit introuvable : le mouvement ne peut pas être enregistré")
			}
			p := &items[idx]
			if _, seen := before[p.ID]; !seen {
				before[p.ID] = *p
			}
			if op.Revalue != nil {
				op.Revalue(p)
			}

			newSound := p.Quantity + op.Delta.Sound
			newDefective := p.DefectiveQty + op.Delta.Defective
			if newSound < 0 {
				return nil, fmt.Errorf("stock insuffisant pour « %s » : %d en stock, %d demandé(s)",
					p.Name, p.Quantity, -op.Delta.Sound)
			}
			if newDefective < 0 {
				return nil, fmt.Errorf("stock défectueux insuffisant pour « %s » : %d disponible(s), %d demandé(s)",
					p.Name, p.DefectiveQty, -op.Delta.Defective)
			}
			p.Quantity, p.DefectiveQty, p.UpdatedAt = newSound, newDefective, now

			m := op.Movement
			m.ID = util.NewID("mvt")
			m.Ref = refs[i]
			m.ProductID, m.ProductName, m.ProductSKU = p.ID, p.Name, p.SKU
			m.StockAfter = newSound
			m.UserID, m.UserName = u.ID, u.FullName
			m.CreatedAt = now
			if m.Date.IsZero() {
				m.Date = now
			}
			written = append(written, m)
		}
		return items, nil
	})
	if err != nil {
		return nil, err
	}

	// Le journal est écrit ensuite. S'il échoue, les fiches produits sont
	// remises exactement dans leur état d'avant : aucune quantité ne bouge sans
	// mouvement correspondant, c'est la règle qui tient tout le reste.
	if err := c.db.Movements.Transaction(func(items []models.Movement) ([]models.Movement, error) {
		return append(items, written...), nil
	}); err != nil {
		c.restoreProducts(before)
		return nil, fmt.Errorf("le mouvement n'a pas pu être enregistré, le stock est inchangé : %w", err)
	}
	return written, nil
}

// restoreProducts remet des fiches produits dans un état antérieur connu.
func (c *core) restoreProducts(before map[string]models.Product) {
	if len(before) == 0 {
		return
	}
	_ = c.db.Products.Transaction(func(items []models.Product) ([]models.Product, error) {
		for i, p := range items {
			if prev, ok := before[p.ID]; ok {
				items[i] = prev
			}
		}
		return items, nil
	})
}

// applyMovement applique une opération unique. Le produit passé sert à
// identifier l'article ; ses quantités sont relues dans la transaction.
func (c *core) applyMovement(u models.User, p models.Product, delta stockDelta, m models.Movement) (models.Movement, error) {
	out, err := c.applyStock(u, []stockOp{{ProductID: p.ID, Delta: delta, Movement: m}})
	if err != nil {
		return models.Movement{}, err
	}
	return out[0], nil
}

// ---------------------------------------------------------------------------
// Consultation
// ---------------------------------------------------------------------------

// MovementQuery filtre le journal des mouvements.
type MovementQuery struct {
	Search    string   `json:"search"`
	Types     []string `json:"types"`
	ProductID string   `json:"productId"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Limit     int      `json:"limit"`
}

// ListMovements renvoie les mouvements du plus récent au plus ancien.
func (s *Stock) ListMovements(q MovementQuery) ([]models.Movement, error) {
	if _, err := s.guard(""); err != nil {
		return nil, err
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, t := range q.Types {
		allowed[t] = true
	}

	out := []models.Movement{}
	for _, m := range s.db.Movements.All() {
		if len(allowed) > 0 && !allowed[string(m.Type)] {
			continue
		}
		if q.ProductID != "" && m.ProductID != q.ProductID {
			continue
		}
		if !util.InRange(m.Date, from, to) {
			continue
		}
		if q.Search != "" && !(util.Contains(m.ProductName, q.Search) ||
			util.Contains(m.ProductSKU, q.Search) ||
			util.Contains(m.PartyName, q.Search) ||
			util.Contains(m.DocumentNo, q.Search) ||
			util.Contains(m.Reason, q.Search)) {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// StockSummary donne les indicateurs globaux du magasin.
type StockSummary struct {
	ProductCount    int   `json:"productCount"`
	TotalUnits      int   `json:"totalUnits"`
	DefectiveUnits  int   `json:"defectiveUnits"`
	StockValueCost  int64 `json:"stockValueCost"` // valorisation au coût
	StockValueSale  int64 `json:"stockValueSale"` // valorisation au prix de vente
	PotentialMargin int64 `json:"potentialMargin"`
	LowStockCount   int   `json:"lowStockCount"`
	OutOfStockCount int   `json:"outOfStockCount"`
	CategoryCount   int   `json:"categoryCount"`
}

// Summary calcule l'état du stock à l'instant présent.
func (s *Stock) Summary() (StockSummary, error) {
	if _, err := s.guard(""); err != nil {
		return StockSummary{}, err
	}
	var sum StockSummary
	sum.CategoryCount = s.db.Categories.Count()
	for _, p := range s.db.Products.All() {
		if !p.Active {
			continue
		}
		sum.ProductCount++
		sum.TotalUnits += p.Quantity
		sum.DefectiveUnits += p.DefectiveQty
		sum.StockValueCost += int64(p.Quantity) * p.PurchasePrice
		sum.StockValueSale += int64(p.Quantity) * p.SalePrice
		if p.Quantity <= 0 {
			sum.OutOfStockCount++
		} else if p.IsLow() {
			sum.LowStockCount++
		}
	}
	sum.PotentialMargin = sum.StockValueSale - sum.StockValueCost
	return sum, nil
}

// ---------------------------------------------------------------------------
// Opérations de stock
// ---------------------------------------------------------------------------

// MovementInput est la charge utile des opérations manuelles de stock.
type MovementInput struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
	Date      string `json:"date"`
	Reason    string `json:"reason"`
	Notes     string `json:"notes"`
	PartyID   string `json:"partyId"`
	InvoiceID string `json:"invoiceId"` // retour client rattaché à une facture
	Restock   bool   `json:"restock"`   // retour client : remettre en stock vendable ?
}

func (s *Stock) prepare(in MovementInput) (models.Product, time.Time, error) {
	if in.Quantity <= 0 {
		return models.Product{}, time.Time{}, fmt.Errorf("la quantité doit être supérieure à zéro")
	}
	p, err := s.db.Products.Get(in.ProductID)
	if err != nil {
		return models.Product{}, time.Time{}, fmt.Errorf("produit introuvable")
	}
	date, err := parseDate(in.Date)
	if err != nil {
		return models.Product{}, time.Time{}, err
	}
	return p, date, nil
}

// DeclareDefective sort des articles du stock vendable et les place en stock
// défectueux. Le stock total ne change pas ; sa composition, si.
func (s *Stock) DeclareDefective(in MovementInput) (models.Movement, error) {
	u, err := s.guard("stock")
	if err != nil {
		return models.Movement{}, err
	}
	p, date, err := s.prepare(in)
	if err != nil {
		return models.Movement{}, err
	}
	reason, err := requireText(in.Reason, "Motif")
	if err != nil {
		return models.Movement{}, err
	}
	m, err := s.applyMovement(u, p, stockDelta{Sound: -in.Quantity, Defective: in.Quantity}, models.Movement{
		Type: models.MovementDefect, Date: date, Quantity: in.Quantity,
		UnitCost: p.PurchasePrice, Reason: reason, Notes: trim(in.Notes),
	})
	if err != nil {
		return models.Movement{}, err
	}
	s.log(u, "DEFECT", "product", p.ID, "%d × « %s » déclaré(s) défectueux, %s", in.Quantity, p.Name, reason)
	return m, nil
}

// RepairDefective replace des articles réparés dans le stock vendable.
func (s *Stock) RepairDefective(in MovementInput) (models.Movement, error) {
	u, err := s.guard("stock")
	if err != nil {
		return models.Movement{}, err
	}
	p, date, err := s.prepare(in)
	if err != nil {
		return models.Movement{}, err
	}
	m, err := s.applyMovement(u, p, stockDelta{Sound: in.Quantity, Defective: -in.Quantity}, models.Movement{
		Type: models.MovementRepair, Date: date, Quantity: in.Quantity,
		UnitCost: p.PurchasePrice,
		Reason:   defaultString(in.Reason, "Article réparé et remis en vente"), Notes: trim(in.Notes),
	})
	if err != nil {
		return models.Movement{}, err
	}
	s.log(u, "REPAIR", "product", p.ID, "%d × « %s » remis en stock après réparation", in.Quantity, p.Name)
	return m, nil
}

// ScrapDefective met au rebut des articles défectueux : ils quittent
// définitivement le stock et la valeur correspondante devient une perte, reprise
// dans le compte de résultat.
func (s *Stock) ScrapDefective(in MovementInput) (models.Movement, error) {
	u, err := s.guard("stock")
	if err != nil {
		return models.Movement{}, err
	}
	p, date, err := s.prepare(in)
	if err != nil {
		return models.Movement{}, err
	}
	reason, err := requireText(in.Reason, "Motif")
	if err != nil {
		return models.Movement{}, err
	}
	m, err := s.applyMovement(u, p, stockDelta{Defective: -in.Quantity}, models.Movement{
		Type: models.MovementScrap, Date: date, Quantity: in.Quantity,
		UnitCost: p.PurchasePrice, Reason: reason, Notes: trim(in.Notes),
	})
	if err != nil {
		return models.Movement{}, err
	}
	s.log(u, "SCRAP", "product", p.ID, "%d × « %s » mis au rebut (perte %d), %s",
		in.Quantity, p.Name, int64(in.Quantity)*p.PurchasePrice, reason)
	return m, nil
}

// ReturnFromCustomer enregistre un retour client. Selon l'état de la
// marchandise, elle rejoint le stock vendable ou le stock défectueux.
// L'avoir financier se fait par annulation ou avoir sur la facture ; ce
// mouvement ne touche que le stock physique.
func (s *Stock) ReturnFromCustomer(in MovementInput) (models.Movement, error) {
	u, err := s.guard("stock")
	if err != nil {
		return models.Movement{}, err
	}
	p, date, err := s.prepare(in)
	if err != nil {
		return models.Movement{}, err
	}
	reason, err := requireText(in.Reason, "Motif du retour")
	if err != nil {
		return models.Movement{}, err
	}

	delta := stockDelta{Defective: in.Quantity}
	destination := "stock défectueux"
	if in.Restock {
		delta = stockDelta{Sound: in.Quantity}
		destination = "stock vendable"
	}

	mv := models.Movement{
		Type: models.MovementReturnCustomer, Date: date, Quantity: in.Quantity,
		UnitCost: p.PurchasePrice, UnitPrice: p.SalePrice,
		Reason: reason, Notes: trim(in.Notes),
	}
	if in.PartyID != "" {
		if party, err := s.db.Parties.Get(in.PartyID); err == nil {
			mv.PartyID, mv.PartyName = party.ID, party.Name
		}
	}
	if in.InvoiceID != "" {
		if inv, err := s.db.Invoices.Get(in.InvoiceID); err == nil {
			mv.DocumentID, mv.DocumentNo = inv.ID, inv.Number
			if mv.PartyName == "" {
				mv.PartyID, mv.PartyName = inv.CustomerID, inv.CustomerName
			}
			// Le prix de reprise est celui effectivement facturé.
			for _, l := range inv.Lines {
				if l.ProductID == p.ID {
					mv.UnitPrice = l.UnitPrice
					mv.UnitCost = l.UnitCost
					break
				}
			}
		}
	}

	m, err := s.applyMovement(u, p, delta, mv)
	if err != nil {
		return models.Movement{}, err
	}
	s.log(u, "RETURN_IN", "product", p.ID, "Retour client : %d × « %s » vers %s, %s",
		in.Quantity, p.Name, destination, reason)
	return m, nil
}

// ReturnToSupplier renvoie de la marchandise au fournisseur. La sortie est
// prise sur le stock défectueux en priorité, puis sur le stock sain.
func (s *Stock) ReturnToSupplier(in MovementInput) (models.Movement, error) {
	u, err := s.guard("stock")
	if err != nil {
		return models.Movement{}, err
	}
	p, date, err := s.prepare(in)
	if err != nil {
		return models.Movement{}, err
	}
	reason, err := requireText(in.Reason, "Motif du retour")
	if err != nil {
		return models.Movement{}, err
	}
	if p.Quantity+p.DefectiveQty < in.Quantity {
		return models.Movement{}, fmt.Errorf("« %s » : %d unité(s) disponibles au total, %d demandée(s)",
			p.Name, p.Quantity+p.DefectiveQty, in.Quantity)
	}

	fromDefective := minInt(in.Quantity, p.DefectiveQty)
	fromSound := in.Quantity - fromDefective

	mv := models.Movement{
		Type: models.MovementReturnSupplier, Date: date, Quantity: in.Quantity,
		UnitCost: p.PurchasePrice, Reason: reason, Notes: trim(in.Notes),
	}
	if in.PartyID != "" {
		if party, err := s.db.Parties.Get(in.PartyID); err == nil {
			mv.PartyID, mv.PartyName = party.ID, party.Name
		}
	}

	m, err := s.applyMovement(u, p, stockDelta{Sound: -fromSound, Defective: -fromDefective}, mv)
	if err != nil {
		return models.Movement{}, err
	}
	s.log(u, "RETURN_OUT", "product", p.ID, "Retour fournisseur : %d × « %s » (%d défectueux), %s",
		in.Quantity, p.Name, fromDefective, reason)
	return m, nil
}

// InventoryInput porte une correction d'inventaire : la quantité comptée
// physiquement remplace la quantité théorique.
type InventoryInput struct {
	ProductID     string `json:"productId"`
	CountedSound  int    `json:"countedSound"`
	CountedDefect int    `json:"countedDefect"`
	Reason        string `json:"reason"`
	Date          string `json:"date"`
}

// AdjustInventory aligne le stock théorique sur le stock compté. L'écart est
// tracé : c'est la seule façon de corriger une quantité sans passer par un
// achat ou une vente.
func (s *Stock) AdjustInventory(in InventoryInput) (models.Movement, error) {
	u, err := s.guard("stock")
	if err != nil {
		return models.Movement{}, err
	}
	if in.CountedSound < 0 || in.CountedDefect < 0 {
		return models.Movement{}, fmt.Errorf("les quantités comptées ne peuvent pas être négatives")
	}
	p, err := s.db.Products.Get(in.ProductID)
	if err != nil {
		return models.Movement{}, fmt.Errorf("produit introuvable")
	}
	date, err := parseDate(in.Date)
	if err != nil {
		return models.Movement{}, err
	}
	reason, err := requireText(in.Reason, "Motif de l'ajustement")
	if err != nil {
		return models.Movement{}, err
	}

	deltaSound := in.CountedSound - p.Quantity
	deltaDefect := in.CountedDefect - p.DefectiveQty
	if deltaSound == 0 && deltaDefect == 0 {
		return models.Movement{}, fmt.Errorf("le comptage correspond déjà au stock enregistré, rien à corriger")
	}

	m, err := s.applyMovement(u, p, stockDelta{Sound: deltaSound, Defective: deltaDefect}, models.Movement{
		Type: models.MovementAdjust, Date: date, Quantity: absInt(deltaSound) + absInt(deltaDefect),
		UnitCost: p.PurchasePrice,
		Reason:   reason,
		Notes: fmt.Sprintf("Théorique %d sain / %d défectueux → compté %d / %d (écart %+d / %+d)",
			p.Quantity, p.DefectiveQty, in.CountedSound, in.CountedDefect, deltaSound, deltaDefect),
	})
	if err != nil {
		return models.Movement{}, err
	}
	s.log(u, "ADJUST", "product", p.ID, "Inventaire « %s » : écart %+d sain, %+d défectueux, %s",
		p.Name, deltaSound, deltaDefect, reason)
	return m, nil
}

// ---------------------------------------------------------------------------
// Fiche de vie d'un produit
// ---------------------------------------------------------------------------

// ProductHistory rassemble tout ce qui est arrivé à un article.
type ProductHistory struct {
	Product     ProductView       `json:"product"`
	Movements   []models.Movement `json:"movements"`
	TotalIn     int               `json:"totalIn"`
	TotalOut    int               `json:"totalOut"`
	TotalSold   int               `json:"totalSold"`
	Revenue     int64             `json:"revenue"`
	CostOfSales int64             `json:"costOfSales"`
}

// History renvoie la fiche de vie complète d'un produit.
func (s *Stock) History(productID string) (ProductHistory, error) {
	u, err := s.guard("")
	if err != nil {
		return ProductHistory{}, err
	}
	p, err := s.db.Products.Get(productID)
	if err != nil {
		return ProductHistory{}, err
	}
	cats := map[string]models.Category{}
	for _, c := range s.db.Categories.All() {
		cats[c.ID] = c
	}
	showCost := s.canSeeCost(u)
	cat := Catalog{s.core}
	h := ProductHistory{Product: cat.viewProduct(p, cats, showCost)}

	for _, m := range s.db.Movements.Find(func(m models.Movement) bool { return m.ProductID == productID }) {
		h.Movements = append(h.Movements, m)
		switch m.Type {
		case models.MovementIn, models.MovementReturnCustomer:
			h.TotalIn += m.Quantity
		case models.MovementOut:
			h.TotalOut += m.Quantity
			h.TotalSold += m.Quantity
			h.Revenue += int64(m.Quantity) * m.UnitPrice
			h.CostOfSales += int64(m.Quantity) * m.UnitCost
		case models.MovementReturnSupplier, models.MovementScrap:
			h.TotalOut += m.Quantity
		}
	}
	sort.Slice(h.Movements, func(i, j int) bool { return h.Movements[i].Date.After(h.Movements[j].Date) })
	if !showCost {
		h.CostOfSales = 0
		for i := range h.Movements {
			h.Movements[i].UnitCost = 0
		}
	}
	return h, nil
}

// ---------------------------------------------------------------------------

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
