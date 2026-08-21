package services

import (
	"encoding/json"
	"regexp"
	"testing"

	"comptoir/internal/models"
)

// Une tranche Go laissée à nil se sérialise en « null », pas en « [] ».
// L'interface reçoit ce null là où elle attend un tableau, appelle .map ou
// .length dessus, lève une exception, et React démonte tout le rendu. Le
// symptôme est un écran entièrement noir, sans message.
//
// Le piège ne se voit que sur des données clairsemées : un poste bien rempli
// n'a jamais de liste vide, et le défaut dort jusqu'au jour où un commerçant
// ouvre l'écran des créances sans avoir d'impayé.
//
// Ce test balaie donc toutes les réponses du moteur sur une boutique vide.
func TestAucuneListeNulleDansLesReponses(t *testing.T) {
	s := newSuite(t)
	verifier(t, s)
}

// Le même contrôle sur une boutique qui a vécu : certaines listes se remplissent
// et d'autres restent vides, ce qui est le cas réel d'un commerce qui vend sans
// tenir de charges, ou qui écoule tout son stock.
func TestAucuneListeNulle_BoutiquePartielle(t *testing.T) {
	s := newSuite(t)
	s.setTax(0)
	p := s.product("Article", 100000, 200000, 5)
	if _, err := s.sales.CreateInvoice(InvoiceInput{
		Lines: []InvoiceLineInput{{ProductID: p.ID, Quantity: 5}}, Date: today(),
	}); err != nil {
		t.Fatal(err)
	}
	verifier(t, s)
}

func verifier(t *testing.T, s *suite) {
	t.Helper()
	reponses := map[string]func() (any, error){
		"tableau de bord":    func() (any, error) { return s.reports.Dashboard() },
		"rapport de ventes":  func() (any, error) { return s.reports.SalesReport(ReportQuery{}) },
		"compte de résultat": func() (any, error) { return s.reports.IncomeStatement("", "") },
		"situation":          func() (any, error) { return s.reports.BalanceSheet("") },
		"statistiques":       func() (any, error) { return s.reports.Statistics(ReportQuery{}) },
		"meilleurs articles": func() (any, error) { return s.reports.TopProducts(ReportQuery{}, 10) },
		"créances":           func() (any, error) { return s.creances.Etat(CreanceQuery{}) },
		"articles":           func() (any, error) { return s.catalog.ListProducts(ProductQuery{}) },
		"catégories":         func() (any, error) { return s.catalog.ListCategories() },
		"clients":            func() (any, error) { return s.catalog.ListParties(string(models.PartyCustomer), "") },
		"fournisseurs":       func() (any, error) { return s.catalog.ListParties(string(models.PartySupplier), "") },
		"stock à surveiller": func() (any, error) { return s.catalog.LowStockProducts() },
		"mouvements":         func() (any, error) { return s.stock.ListMovements(MovementQuery{}) },
		"résumé du stock":    func() (any, error) { return s.stock.Summary() },
		"factures":           func() (any, error) { return s.sales.ListInvoices(InvoiceQuery{}) },
		"bons d'entrée":      func() (any, error) { return s.purchases.ListPurchases(PurchaseQuery{}) },
		"charges":            func() (any, error) { return s.expenses.ListExpenses(ExpenseQuery{}) },
		"répartition":        func() (any, error) { return s.expenses.Breakdown("", "") },
		"rubriques":          func() (any, error) { return s.expenses.Categories() },
		"comptes":            func() (any, error) { return s.users.List() },
		"rôles":              func() (any, error) { return s.users.Roles() },
		"journal d'audit":    func() (any, error) { return s.users.Audit(AuditQuery{}) },
		"actions du journal": func() (any, error) { return s.users.AuditActions() },
		"paramètres":         func() (any, error) { return s.config.Get() },
		"emplacements":       func() (any, error) { return s.config.DataLocation() },
		"sauvegardes":        func() (any, error) { return NewBackups(s.db, s.sec).List() },
	}

	// Un champ nommé, à null, dans une réponse : « "lignes":null ».
	nul := regexp.MustCompile(`"([a-zA-Z]+)":null`)

	for nom, appel := range reponses {
		valeur, err := appel()
		if err != nil {
			t.Errorf("%s : %v", nom, err)
			continue
		}
		brut, err := json.Marshal(valeur)
		if err != nil {
			t.Errorf("%s : sérialisation impossible : %v", nom, err)
			continue
		}
		// Une réponse qui est elle-même une liste vide doit donner « [] ».
		if string(brut) == "null" {
			t.Errorf("%s : la réponse entière vaut null au lieu d'une liste vide", nom)
			continue
		}
		for _, m := range nul.FindAllStringSubmatch(string(brut), -1) {
			champ := m[1]
			// Les pointeurs facultatifs valent légitimement null : ils sont
			// déclarés « omitempty » et l'interface les traite comme absents.
			if facultatif[champ] {
				continue
			}
			t.Errorf("%s : le champ %q vaut null ; l'interface attend une liste et lèvera une exception",
				nom, champ)
		}
	}
}

// facultatif liste les champs dont null est une valeur normale.
var facultatif = map[string]bool{
	"lastLogin": true, "lastActivity": true, "cancelledAt": true,
	"dueDate": true, "best": true, "user": true, "serials": true,
}
