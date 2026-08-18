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

// Expenses gère les charges d'exploitation : loyer, salaires, électricité,
// transport… Le coût d'achat des marchandises n'y figure pas, il est porté par
// les bons d'entrée et ressort en coût des ventes.
type Expenses struct{ core }

// NewExpenses construit le service des charges.
func NewExpenses(db *storage.Database, a *auth.Service) *Expenses {
	return &Expenses{core{db: db, auth: a}}
}

// ExpenseInput est la charge utile de création ou modification.
type ExpenseInput struct {
	ID            string               `json:"id"`
	Date          string               `json:"date"`
	Category      string               `json:"category"`
	Label         string               `json:"label"`
	Amount        int64                `json:"amount"`
	PaymentMethod models.PaymentMethod `json:"paymentMethod"`
	Beneficiary   string               `json:"beneficiary"`
	Notes         string               `json:"notes"`
}

// ExpenseQuery filtre la liste des charges.
type ExpenseQuery struct {
	Search   string `json:"search"`
	Category string `json:"category"`
	From     string `json:"from"`
	To       string `json:"to"`
	Limit    int    `json:"limit"`
}

// Categories renvoie la liste des rubriques proposées, enrichie de celles déjà
// utilisées dans les données.
func (s *Expenses) Categories() ([]string, error) {
	if _, err := s.guard(""); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, c := range models.ExpenseCategories {
		seen[c] = true
		out = append(out, c)
	}
	for _, e := range s.db.Expenses.All() {
		if e.Category != "" && !seen[e.Category] {
			seen[e.Category] = true
			out = append(out, e.Category)
		}
	}
	return out, nil
}

// ListExpenses renvoie les charges, de la plus récente à la plus ancienne.
func (s *Expenses) ListExpenses(q ExpenseQuery) ([]models.Expense, error) {
	if _, err := s.guard("expenses"); err != nil {
		return nil, err
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, err
	}
	out := []models.Expense{}
	for _, e := range s.db.Expenses.All() {
		if q.Category != "" && e.Category != q.Category {
			continue
		}
		if !util.InRange(e.Date, from, to) {
			continue
		}
		if q.Search != "" && !(util.Contains(e.Label, q.Search) ||
			util.Contains(e.Beneficiary, q.Search) || util.Contains(e.Notes, q.Search)) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// SaveExpense crée ou met à jour une charge.
func (s *Expenses) SaveExpense(in ExpenseInput) (models.Expense, error) {
	u, err := s.guard("expenses")
	if err != nil {
		return models.Expense{}, err
	}
	label, err := requireText(in.Label, "Libellé")
	if err != nil {
		return models.Expense{}, err
	}
	category, err := requireText(in.Category, "Rubrique")
	if err != nil {
		return models.Expense{}, err
	}
	if in.Amount <= 0 {
		return models.Expense{}, fmt.Errorf("le montant doit être supérieur à zéro")
	}
	date, err := parseDate(in.Date)
	if err != nil {
		return models.Expense{}, err
	}

	now := time.Now()
	if in.ID == "" {
		e := models.Expense{
			ID: util.NewID("exp"), Date: date, Category: category, Label: label,
			Amount: in.Amount, PaymentMethod: defaultPayment(in.PaymentMethod),
			Beneficiary: trim(in.Beneficiary), Notes: trim(in.Notes),
			UserID: u.ID, UserName: u.FullName, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.db.Expenses.Insert(e); err != nil {
			return models.Expense{}, err
		}
		s.log(u, "CREATE", "expense", e.ID, "Charge « %s » (%s) de %d enregistrée", e.Label, e.Category, e.Amount)
		return e, nil
	}

	e, err := s.db.Expenses.Get(in.ID)
	if err != nil {
		return models.Expense{}, err
	}
	previous := e.Amount
	e.Date, e.Category, e.Label, e.Amount = date, category, label, in.Amount
	e.PaymentMethod = defaultPayment(in.PaymentMethod)
	e.Beneficiary, e.Notes, e.UpdatedAt = trim(in.Beneficiary), trim(in.Notes), now
	if err := s.db.Expenses.Update(e); err != nil {
		return models.Expense{}, err
	}
	s.log(u, "UPDATE", "expense", e.ID, "Charge « %s » modifiée (%d → %d)", e.Label, previous, e.Amount)
	return e, nil
}

// DeleteExpense supprime une charge.
func (s *Expenses) DeleteExpense(id string) error {
	u, err := s.guard("expenses")
	if err != nil {
		return err
	}
	e, err := s.db.Expenses.Get(id)
	if err != nil {
		return err
	}
	if err := s.db.Expenses.Delete(id); err != nil {
		return err
	}
	s.log(u, "DELETE", "expense", id, "Charge « %s » de %d supprimée", e.Label, e.Amount)
	return nil
}

// ExpenseBreakdown résume les charges d'une période par rubrique.
type ExpenseBreakdown struct {
	Category string  `json:"category"`
	Amount   int64   `json:"amount"`
	Count    int     `json:"count"`
	Share    float64 `json:"share"` // part du total, en pourcentage
}

// Breakdown renvoie la répartition des charges sur une période, de la rubrique
// la plus lourde à la plus légère.
func (s *Expenses) Breakdown(from, to string) ([]ExpenseBreakdown, error) {
	if _, err := s.guard("expenses"); err != nil {
		return nil, err
	}
	start, end, err := parseRange(from, to)
	if err != nil {
		return nil, err
	}
	byCategory := map[string]*ExpenseBreakdown{}
	var total int64
	for _, e := range s.db.Expenses.All() {
		if !util.InRange(e.Date, start, end) {
			continue
		}
		b, ok := byCategory[e.Category]
		if !ok {
			b = &ExpenseBreakdown{Category: e.Category}
			byCategory[e.Category] = b
		}
		b.Amount += e.Amount
		b.Count++
		total += e.Amount
	}
	out := make([]ExpenseBreakdown, 0, len(byCategory))
	for _, b := range byCategory {
		if total > 0 {
			b.Share = float64(b.Amount) / float64(total) * 100
		}
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	return out, nil
}
