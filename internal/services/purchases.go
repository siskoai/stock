package services

import (
	"fmt"
	"math"
	"sort"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Purchases gère les entrées de marchandise : bons d'entrée fournisseur.
// C'est le seul point qui augmente le stock d'achat et met à jour le coût
// unitaire moyen pondéré (CUMP).
type Purchases struct{ core }

// NewPurchases construit le service des entrées.
func NewPurchases(db *storage.Database, a *auth.Service) *Purchases {
	return &Purchases{core{db: db, auth: a}}
}

// PurchaseLineInput est une ligne de bon d'entrée saisie dans l'interface.
type PurchaseLineInput struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
	UnitCost  int64  `json:"unitCost"` // prix d'achat unitaire HT
	Discount  int64  `json:"discount"`

	// TaxRate absent applique le taux par défaut ; à 0, la ligne est exonérée.
	TaxRate *float64 `json:"taxRate"`
}

// PurchaseInput est la charge utile de création d'un bon d'entrée.
type PurchaseInput struct {
	Date       string              `json:"date"`
	SupplierID string              `json:"supplierId"`
	Reference  string              `json:"reference"`
	Lines      []PurchaseLineInput `json:"lines"`
	OtherCosts int64               `json:"otherCosts"` // transport, douane, manutention
	Notes      string              `json:"notes"`
	// TargetMarginPct applique une marge cible au prix de vente des articles
	// reçus. Laissé à zéro, les prix de vente ne bougent pas.
	TargetMarginPct float64 `json:"targetMarginPct"`
}

// PurchaseQuery filtre la liste des bons d'entrée.
type PurchaseQuery struct {
	Search     string `json:"search"`
	SupplierID string `json:"supplierId"`
	Status     string `json:"status"`
	From       string `json:"from"`
	To         string `json:"to"`
	Limit      int    `json:"limit"`
}

// ListPurchases renvoie les bons d'entrée, du plus récent au plus ancien.
func (s *Purchases) ListPurchases(q PurchaseQuery) ([]models.Purchase, error) {
	if _, err := s.guard("purchases"); err != nil {
		return nil, err
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, err
	}
	out := []models.Purchase{}
	for _, p := range s.db.Purchases.All() {
		if q.SupplierID != "" && p.SupplierID != q.SupplierID {
			continue
		}
		if q.Status != "" && string(p.Status) != q.Status {
			continue
		}
		if !util.InRange(p.Date, from, to) {
			continue
		}
		if q.Search != "" && !(util.Contains(p.Number, q.Search) ||
			util.Contains(p.SupplierName, q.Search) || util.Contains(p.Reference, q.Search)) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// GetPurchase renvoie un bon d'entrée complet.
func (s *Purchases) GetPurchase(id string) (models.Purchase, error) {
	if _, err := s.guard("purchases"); err != nil {
		return models.Purchase{}, err
	}
	return s.db.Purchases.Get(id)
}

// CreatePurchase enregistre une réception de marchandise : le stock augmente,
// le coût moyen pondéré est recalculé, un mouvement d'entrée est écrit par
// ligne.
//
// Les frais annexes (OtherCosts) sont répartis au prorata de la valeur des
// lignes et intégrés au coût de revient : c'est ce coût complet qui sert
// ensuite au calcul de la marge.
//
// Le recalcul du coût moyen a lieu à l'intérieur de la transaction de stock,
// ligne après ligne, sur les quantités à jour : un même article présent sur
// deux lignes du bon est correctement cumulé.
func (s *Purchases) CreatePurchase(in PurchaseInput) (models.Purchase, error) {
	u, err := s.guard("purchases")
	if err != nil {
		return models.Purchase{}, err
	}
	if len(in.Lines) == 0 {
		return models.Purchase{}, fmt.Errorf("ajoutez au moins une ligne au bon d'entrée")
	}
	if in.TargetMarginPct < 0 || in.TargetMarginPct >= 100 {
		return models.Purchase{}, fmt.Errorf("la marge cible doit être comprise entre 0 et 100 %%")
	}
	date, err := parseDate(in.Date)
	if err != nil {
		return models.Purchase{}, err
	}
	settings := s.db.Settings()

	// 1. Validation complète avant toute écriture.
	docLines := make([]models.DocLine, len(in.Lines))
	for i, l := range in.Lines {
		if l.Quantity <= 0 {
			return models.Purchase{}, fmt.Errorf("ligne %d : la quantité doit être supérieure à zéro", i+1)
		}
		if l.UnitCost < 0 || l.Discount < 0 {
			return models.Purchase{}, fmt.Errorf("ligne %d : les montants ne peuvent pas être négatifs", i+1)
		}
		p, err := s.db.Products.Get(l.ProductID)
		if err != nil {
			return models.Purchase{}, fmt.Errorf("ligne %d : produit introuvable", i+1)
		}
		taxRate := settings.DefaultTaxRate
		if l.TaxRate != nil {
			taxRate = *l.TaxRate
		}
		if taxRate < 0 || taxRate > 100 {
			return models.Purchase{}, fmt.Errorf("ligne %d : taux de taxe invalide (%.2f %%)", i+1, taxRate)
		}
		docLines[i] = models.DocLine{
			ProductID: p.ID, ProductName: p.Name, SKU: p.SKU,
			Quantity: l.Quantity, UnitPrice: l.UnitCost, UnitCost: l.UnitCost,
			Discount: l.Discount, TaxRate: taxRate,
		}
	}

	docLines, subtotalHT, taxTotal, total := computeTotals(docLines, 0, false)
	otherCosts := maxInt64(in.OtherCosts, 0)
	total += otherCosts

	var supplierName string
	if in.SupplierID != "" {
		sup, err := s.db.Parties.Get(in.SupplierID)
		if err != nil || sup.Type != models.PartySupplier {
			return models.Purchase{}, fmt.Errorf("fournisseur introuvable")
		}
		supplierName = sup.Name
	}

	// 2. Répartition des frais annexes. Le reliquat d'arrondi va sur la
	//    dernière ligne : la somme des quotes-parts fait exactement OtherCosts.
	freight := make([]int64, len(docLines))
	remaining := otherCosts
	for i, l := range docLines {
		if i == len(docLines)-1 {
			freight[i] = remaining
			break
		}
		cut := share(otherCosts, l.LineHT, subtotalHT)
		if cut > remaining {
			cut = remaining
		}
		freight[i] = cut
		remaining -= cut
	}

	number, err := s.db.NextPurchaseNumber()
	if err != nil {
		return models.Purchase{}, err
	}

	now := time.Now()
	purchase := models.Purchase{
		ID: util.NewID("pur"), Number: number, Date: date,
		SupplierID: in.SupplierID, SupplierName: supplierName, Reference: trim(in.Reference),
		Lines: docLines, SubtotalHT: subtotalHT, TaxTotal: taxTotal,
		OtherCosts: otherCosts, Total: total,
		Status: models.StatusIssued, Notes: trim(in.Notes),
		UserID: u.ID, UserName: u.FullName, CreatedAt: now, UpdatedAt: now,
	}

	// 3. Le document est écrit d'abord : du stock sans justificatif serait pire
	//    qu'un justificatif sans stock, et ce dernier cas est réparé juste après.
	if err := s.db.Purchases.Insert(purchase); err != nil {
		s.db.ReleasePurchaseNumber(number)
		return models.Purchase{}, err
	}

	// 4. Entrée en stock et revalorisation, en une seule transaction.
	ops := make([]stockOp, 0, len(docLines))
	for i, l := range docLines {
		landed := l.LineHT + freight[i]
		landedUnit := landed
		if l.Quantity > 0 {
			landedUnit = int64(math.Round(float64(landed) / float64(l.Quantity)))
		}
		qty, unitCost, margin := l.Quantity, landedUnit, in.TargetMarginPct
		ops = append(ops, stockOp{
			ProductID: l.ProductID,
			Delta:     stockDelta{Sound: qty},
			Movement: models.Movement{
				Type: models.MovementIn, Date: date, Quantity: qty,
				UnitCost: unitCost, PartyID: in.SupplierID, PartyName: supplierName,
				DocumentID: purchase.ID, DocumentNo: purchase.Number,
				Reason: "Réception fournisseur",
			},
			Revalue: func(p *models.Product) {
				p.PurchasePrice = weightedAverageCost(p.Quantity, p.PurchasePrice, qty, unitCost)
				if margin > 0 {
					// Marge exprimée sur le prix de vente : PV = coût / (1 − marge).
					p.SalePrice = int64(math.Round(float64(p.PurchasePrice) / (1 - margin/100)))
				}
			},
		})
	}
	if _, err := s.applyStock(u, ops); err != nil {
		_ = s.db.Purchases.Delete(purchase.ID)
		s.db.ReleasePurchaseNumber(number)
		return models.Purchase{}, err
	}

	s.log(u, "CREATE", "purchase", purchase.ID, "Bon d'entrée %s, %d ligne(s), total %d",
		purchase.Number, len(docLines), purchase.Total)
	return purchase, nil
}

// CancelPurchase annule un bon d'entrée et retire du stock la marchandise
// reçue. Refusé si la marchandise a déjà été vendue en partie.
//
// Le coût moyen pondéré n'est pas recalculé à rebours : il refléterait un
// historique d'achats qui n'existe plus. Il se réaligne naturellement à la
// prochaine entrée. Cette limite est documentée dans docs/COMPTABILITE.md.
func (s *Purchases) CancelPurchase(id, reason string) error {
	u, err := s.guard("purchases")
	if err != nil {
		return err
	}
	purchase, err := s.db.Purchases.Get(id)
	if err != nil {
		return err
	}
	if purchase.Status == models.StatusCancelled {
		return fmt.Errorf("le bon d'entrée %s est déjà annulé", purchase.Number)
	}
	motif, err := requireText(reason, "Motif d'annulation")
	if err != nil {
		return err
	}

	// Vérification préalable sur le cumul par produit : une même référence peut
	// figurer sur plusieurs lignes du bon.
	needed := map[string]int{}
	for _, l := range purchase.Lines {
		needed[l.ProductID] += l.Quantity
	}
	for productID, qty := range needed {
		p, err := s.db.Products.Get(productID)
		if err != nil {
			return fmt.Errorf("un article du bon d'entrée n'existe plus : annulation impossible")
		}
		if p.Quantity < qty {
			return fmt.Errorf("« %s » : %d en stock, %d à retirer, une partie a déjà été vendue, l'annulation retirerait un stock inexistant",
				p.Name, p.Quantity, qty)
		}
	}

	ops := make([]stockOp, 0, len(purchase.Lines))
	for _, l := range purchase.Lines {
		ops = append(ops, stockOp{
			ProductID: l.ProductID,
			Delta:     stockDelta{Sound: -l.Quantity},
			Movement: models.Movement{
				Type: models.MovementReturnSupplier, Date: time.Now(), Quantity: l.Quantity,
				UnitCost: l.UnitCost, PartyID: purchase.SupplierID, PartyName: purchase.SupplierName,
				DocumentID: purchase.ID, DocumentNo: purchase.Number,
				Reason: "Annulation du bon d'entrée, " + motif,
			},
		})
	}
	if _, err := s.applyStock(u, ops); err != nil {
		return err
	}

	now := time.Now()
	purchase.Status = models.StatusCancelled
	purchase.CancelledAt = &now
	purchase.UpdatedAt = now
	purchase.Notes = appendNote(purchase.Notes, fmt.Sprintf("Annulé le %s par %s, %s",
		now.Format("02/01/2006"), u.FullName, motif))
	if err := s.db.Purchases.Update(purchase); err != nil {
		return err
	}
	s.log(u, "CANCEL", "purchase", purchase.ID, "Bon d'entrée %s annulé, %s", purchase.Number, motif)
	return nil
}

// ---------------------------------------------------------------------------

// weightedAverageCost calcule le nouveau coût unitaire moyen pondéré.
// Un stock existant négatif ou nul est ignoré : le nouveau coût est celui de
// l'entrée.
func weightedAverageCost(oldQty int, oldCost int64, newQty int, newCost int64) int64 {
	if oldQty <= 0 {
		return newCost
	}
	if newQty <= 0 {
		return oldCost
	}
	totalValue := float64(oldQty)*float64(oldCost) + float64(newQty)*float64(newCost)
	return int64(math.Round(totalValue / float64(oldQty+newQty)))
}

func maxInt64(v, min int64) int64 {
	if v < min {
		return min
	}
	return v
}

func appendNote(existing, added string) string {
	if trim(existing) == "" {
		return added
	}
	return existing + "\n" + added
}
