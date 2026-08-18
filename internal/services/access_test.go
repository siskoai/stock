package services

import (
	"errors"
	"testing"

	"comptoir/internal/auth"
	"comptoir/internal/models"
)

// Un vendeur vend. Il ne doit voir ni prix d'achat, ni marge, ni valorisation
// du stock : ces chiffres sont retirés côté Go, pas masqués côté interface.
func TestVendeur_NeVoitNiCoutNiMarge(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Ordinateur portable", 30000000, 45000000, 5)
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 1}},
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}

	s.loginAs("vendeur", models.RoleSeller)

	products, err := s.catalog.ListProducts(ProductQuery{})
	if err != nil {
		t.Fatalf("liste des produits : %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("%d produit(s), attendu 1", len(products))
	}
	v := products[0]
	if v.PurchasePrice != 0 || v.StockValue != 0 || v.MarginAmount != 0 || v.MarginRate != 0 {
		t.Errorf("un vendeur voit le coût : achat=%d valeur=%d marge=%d taux=%.1f",
			v.PurchasePrice, v.StockValue, v.MarginAmount, v.MarginRate)
	}
	if v.SalePrice != 45000000 || v.Quantity != 4 {
		t.Errorf("le prix de vente et le stock doivent rester visibles : %d / %d", v.SalePrice, v.Quantity)
	}

	invoices, err := s.sales.ListInvoices(InvoiceQuery{})
	if err != nil {
		t.Fatalf("liste des factures : %v", err)
	}
	if invoices[0].Margin != 0 || invoices[0].CostTotal != 0 || invoices[0].Lines[0].UnitCost != 0 {
		t.Error("un vendeur voit la marge de la facture")
	}
	if invoices[0].Total != 45000000 {
		t.Errorf("le total facturé doit rester visible : %d", invoices[0].Total)
	}

	cats, err := s.catalog.ListCategories()
	if err != nil {
		t.Fatalf("liste des catégories : %v", err)
	}
	for _, c := range cats {
		if c.StockValue != 0 {
			t.Errorf("catégorie « %s » : valeur de stock visible (%d)", c.Name, c.StockValue)
		}
	}
}

func TestVendeur_DomainesInterdits(t *testing.T) {
	s := newSuite(t)
	s.loginAs("vendeur2", models.RoleSeller)

	if _, err := s.expenses.ListExpenses(ExpenseQuery{}); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("charges : erreur = %v, attendu ErrForbidden", err)
	}
	if _, err := s.reports.IncomeStatement("", ""); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("compte de résultat : erreur = %v, attendu ErrForbidden", err)
	}
	if _, err := s.reports.SalesReport(ReportQuery{}); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("rapport de ventes : erreur = %v, attendu ErrForbidden", err)
	}
	if _, err := s.purchases.ListPurchases(PurchaseQuery{}); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("bons d'entrée : erreur = %v, attendu ErrForbidden", err)
	}
	if _, err := s.users.List(); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("comptes : erreur = %v, attendu ErrForbidden", err)
	}
	if _, err := s.catalog.SaveProduct(ProductInput{Name: "Article interdit"}); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("création de produit : erreur = %v, attendu ErrForbidden", err)
	}
}

// Le tableau de bord d'un vendeur reste utilisable : il voit son chiffre
// d'affaires, mais pas la marge ni les charges.
func TestVendeur_TableauDeBordSansIndicateursFinanciers(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Imprimante laser", 10000000, 18000000, 10)
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 2}}, Date: today(),
	}); err != nil {
		t.Fatalf("vente : %v", err)
	}

	s.loginAs("vendeur3", models.RoleSeller)
	d, err := s.reports.Dashboard()
	if err != nil {
		t.Fatalf("tableau de bord : %v", err)
	}
	if len(d.Last30Days) == 0 {
		t.Fatal("les courbes sont vides : un vendeur doit voir son activité")
	}
	if d.TodayRevenue.Value != 36000000 {
		t.Errorf("CA du jour = %d, attendu 36000000", d.TodayRevenue.Value)
	}
	if d.MonthMargin.Value != 0 || d.MonthExpenses.Value != 0 || d.StockValue != 0 {
		t.Errorf("indicateurs financiers visibles : marge=%d charges=%d stock=%d",
			d.MonthMargin.Value, d.MonthExpenses.Value, d.StockValue)
	}
	for _, pt := range d.Last30Days {
		if pt.GrossMargin != 0 || pt.CostOfSales != 0 {
			t.Errorf("la courbe expose la marge : %+v", pt)
			break
		}
	}
}

func TestGerant_AccedeALaFinanceMaisPasAuxComptes(t *testing.T) {
	s := newSuite(t)
	s.loginAs("gerant", models.RoleManager)

	if _, err := s.reports.IncomeStatement("", ""); err != nil {
		t.Errorf("un gérant doit accéder au compte de résultat : %v", err)
	}
	if _, err := s.expenses.ListExpenses(ExpenseQuery{}); err != nil {
		t.Errorf("un gérant doit accéder aux charges : %v", err)
	}
	if _, err := s.users.List(); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("comptes : erreur = %v, attendu ErrForbidden", err)
	}
	if _, err := s.config.Save(s.db.Settings()); !errors.Is(err, auth.ErrForbidden) {
		t.Errorf("paramètres : erreur = %v, attendu ErrForbidden", err)
	}
}

// Un mot de passe réinitialisé n'ouvre rien tant qu'il n'a pas été remplacé.
func TestMotDePasseAChanger_BloqueToutAcces(t *testing.T) {
	s := newSuite(t)
	session := NewSession(s.db, s.sec)

	u, err := s.sec.CreateUser("caissier", "Le Caissier", "motdepasse1", models.RoleSeller)
	if err != nil {
		t.Fatalf("création du compte : %v", err)
	}
	temporaire, err := s.sec.ResetPassword(u.ID, "")
	if err != nil {
		t.Fatalf("réinitialisation : %v", err)
	}
	if len(temporaire) < 8 {
		t.Fatalf("mot de passe provisoire trop court : %q", temporaire)
	}

	state, err := session.Login("caissier", temporaire)
	if err != nil {
		t.Fatalf("connexion avec le mot de passe provisoire : %v", err)
	}
	if state.User == nil || !state.User.MustChangePwd {
		t.Fatal("l'interface n'est pas prévenue que le mot de passe doit changer")
	}
	if _, err := s.catalog.ListProducts(ProductQuery{}); !errors.Is(err, auth.ErrMustChangePassword) {
		t.Errorf("liste des produits : erreur = %v, attendu ErrMustChangePassword", err)
	}

	if _, err := session.ChangePassword(temporaire, "nouveaupass9"); err != nil {
		t.Fatalf("changement de mot de passe : %v", err)
	}
	if _, err := s.catalog.ListProducts(ProductQuery{}); err != nil {
		t.Errorf("après changement, l'accès doit être rendu : %v", err)
	}
}

func TestDernierAdministrateur_NePeutPasEtreRetrograde(t *testing.T) {
	s := newSuite(t)
	current, _ := s.sec.Current()

	if _, err := s.users.Update(UserInput{ID: current.ID, FullName: "La Patronne", Role: models.RoleSeller}); err == nil {
		t.Error("rétrograder le dernier administrateur aurait dû être refusé")
	}
	if _, err := s.users.SetActive(current.ID, false); err == nil {
		t.Error("désactiver son propre compte aurait dû être refusé")
	}

	// Avec un second administrateur, la rétrogradation redevient possible.
	if _, err := s.users.Create(UserInput{
		Username: "adjoint", FullName: "L'Adjoint", Password: "motdepasse1", Role: models.RoleAdmin,
	}); err != nil {
		t.Fatalf("création du second administrateur : %v", err)
	}
	if _, err := s.users.Update(UserInput{ID: current.ID, FullName: "La Patronne", Role: models.RoleManager}); err != nil {
		t.Errorf("la rétrogradation devrait passer : %v", err)
	}
}

func TestSession_EtatEtPremierDemarrage(t *testing.T) {
	s := newSuite(t)
	session := NewSession(s.db, s.sec)

	state := session.State()
	if state.NeedsSetup {
		t.Error("un compte existe : l'écran de premier démarrage ne doit pas s'afficher")
	}
	if !state.Authenticated || state.User == nil {
		t.Fatal("la session administrateur devrait être ouverte")
	}
	if len(state.Scopes) == 0 {
		t.Error("les domaines autorisés ne sont pas transmis à l'interface")
	}

	state = session.Logout()
	if state.Authenticated {
		t.Error("la session devrait être fermée")
	}
	if _, err := s.catalog.ListProducts(ProductQuery{}); !errors.Is(err, auth.ErrNotAuthenticated) {
		t.Errorf("hors session : erreur = %v, attendu ErrNotAuthenticated", err)
	}
}
