package services

import (
	"testing"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
)

// suite regroupe une base neuve et l'ensemble des services, prêts à l'emploi.
type suite struct {
	t   *testing.T
	db  *storage.Database
	sec *auth.Service

	catalog   *Catalog
	stock     *Stock
	sales     *Sales
	creances  *Creances
	purchases *Purchases
	expenses  *Expenses
	reports   *Reports
	users     *Users
	config    *Config
	export    *Export
	docs      *Documents
}

// newSuite ouvre une base dans un répertoire temporaire et ouvre une session
// administrateur : la quasi-totalité des tests part de là.
func newSuite(t *testing.T) *suite {
	t.Helper()
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	sec := auth.New(db.Users, 60)
	s := &suite{
		t: t, db: db, sec: sec,
		catalog:   NewCatalog(db, sec),
		stock:     NewStock(db, sec),
		sales:     NewSales(db, sec),
		creances:  NewCreances(db, sec),
		purchases: NewPurchases(db, sec),
		expenses:  NewExpenses(db, sec),
		reports:   NewReports(db, sec),
		users:     NewUsers(db, sec),
		config:    NewConfig(db, sec),
		export:    NewExport(db, sec),
		docs:      NewDocuments(db, sec),
	}
	if _, err := sec.CreateFirstAdmin("patron", "La Patronne", "motdepasse1"); err != nil {
		t.Fatalf("création de l'administrateur : %v", err)
	}
	if _, err := sec.Login("patron", "motdepasse1"); err != nil {
		t.Fatalf("connexion : %v", err)
	}
	return s
}

// loginAs crée un compte du rôle demandé et ouvre sa session.
func (s *suite) loginAs(username string, role models.Role) {
	s.t.Helper()
	current, _ := s.sec.Current()
	if _, err := s.sec.CreateUser(username, "Compte "+username, "motdepasse1", role); err != nil {
		s.t.Fatalf("création du compte %s : %v", username, err)
	}
	if _, err := s.sec.Login(username, "motdepasse1"); err != nil {
		s.t.Fatalf("connexion de %s : %v", username, err)
	}
	s.t.Cleanup(func() { _, _ = s.sec.Login(current.Username, "motdepasse1") })
}

// product crée un article avec un stock et un coût de départ.
func (s *suite) product(name string, purchase, sale int64, qty int) models.Product {
	s.t.Helper()
	p, err := s.catalog.SaveProduct(ProductInput{
		Name: name, PurchasePrice: purchase, SalePrice: sale,
		InitialQuantity: qty, Unit: "pièce", Active: true,
	})
	if err != nil {
		s.t.Fatalf("création du produit %q : %v", name, err)
	}
	return p
}

// reload relit une fiche produit depuis le magasin.
func (s *suite) reload(id string) models.Product {
	s.t.Helper()
	p, err := s.db.Products.Get(id)
	if err != nil {
		s.t.Fatalf("relecture du produit : %v", err)
	}
	return p
}

// setTax fixe le taux de taxe par défaut.
func (s *suite) setTax(rate float64) {
	s.t.Helper()
	settings := s.db.Settings()
	settings.DefaultTaxRate = rate
	if err := s.db.SaveSettings(settings); err != nil {
		s.t.Fatalf("enregistrement des paramètres : %v", err)
	}
}

func rate(v float64) *float64 { return &v }

func today() string { return time.Now().Format("2006-01-02") }
