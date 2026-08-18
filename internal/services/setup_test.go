package services

import (
	"strings"
	"testing"

	"comptoir/internal/auth"
	"comptoir/internal/brand"
	"comptoir/internal/storage"
)

// newBlank ouvre une base vierge, sans compte : l'état d'un poste qui vient
// d'être installé.
func newBlank(t *testing.T) (*storage.Database, *auth.Service, *Session) {
	t.Helper()
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	sec := auth.New(db.Users, 60)
	return db, sec, NewSession(db, sec)
}

func validSetup() SetupInput {
	return SetupInput{
		Username: "patron", FullName: "La Patronne", Password: "motdepasse1",
		CompanyName: "Sahel Informatique", LegalForm: "SARL",
		City: "Bamako", Country: "Mali", Phone: "+223 00 00 00 00",
		Currency: "XOF", CurrencySymbol: "FCFA", Decimals: 0,
		DefaultTaxRate: 18, SeedCategories: true,
		AutoBackup: true, BackupsToKeep: 30, Theme: "light",
	}
}

func TestSetup_PremierDemarrage(t *testing.T) {
	db, _, session := newBlank(t)

	if state := session.State(); !state.NeedsSetup {
		t.Fatal("un poste sans compte doit demander sa configuration")
	}

	state, err := session.Setup(validSetup())
	if err != nil {
		t.Fatalf("configuration : %v", err)
	}
	if state.NeedsSetup {
		t.Error("la configuration est faite, l'assistant ne doit plus s'afficher")
	}
	if !state.Authenticated || state.User == nil {
		t.Fatal("la session devrait être ouverte dans la foulée")
	}
	if state.User.Role != "ADMIN" {
		t.Errorf("rôle = %s, attendu ADMIN", state.User.Role)
	}
	if state.CompanyName != "Sahel Informatique" {
		t.Errorf("société = %q", state.CompanyName)
	}

	settings := db.Settings()
	if settings.DefaultTaxRate != 18 || settings.CurrencySymbol != "FCFA" {
		t.Errorf("paramètres non appliqués : %+v", settings)
	}
	if settings.City != "Bamako" || settings.LegalForm != "SARL" {
		t.Errorf("identité de l'entreprise non appliquée : %+v", settings)
	}
	if settings.BackupsToKeep != 30 || !settings.AutoBackup {
		t.Errorf("réglages de sauvegarde non appliqués : %+v", settings)
	}
	if db.Categories.Count() == 0 {
		t.Error("les catégories de départ n'ont pas été créées")
	}
}

func TestSetup_SansCategoriesDeDepart(t *testing.T) {
	db, _, session := newBlank(t)
	in := validSetup()
	in.SeedCategories = false

	if _, err := session.Setup(in); err != nil {
		t.Fatalf("configuration : %v", err)
	}
	if n := db.Categories.Count(); n != 0 {
		t.Errorf("%d catégorie(s) créée(s) alors que l'utilisateur n'en voulait pas", n)
	}
}

// Une saisie refusée ne doit pas laisser le poste à moitié configuré :
// l'assistant doit pouvoir être relancé.
func TestSetup_RefusLaisseLePosteVierge(t *testing.T) {
	cases := map[string]func(*SetupInput){
		"mot de passe trop faible": func(in *SetupInput) { in.Password = "court" },
		"société sans nom":         func(in *SetupInput) { in.CompanyName = "  " },
		"taux de taxe aberrant":    func(in *SetupInput) { in.DefaultTaxRate = 250 },
		"décimales impossibles":    func(in *SetupInput) { in.Decimals = 7 },
		"identifiant trop court":   func(in *SetupInput) { in.Username = "ab" },
	}
	for nom, casser := range cases {
		t.Run(nom, func(t *testing.T) {
			db, _, session := newBlank(t)
			in := validSetup()
			casser(&in)

			if _, err := session.Setup(in); err == nil {
				t.Fatal("la configuration aurait dû être refusée")
			}
			if !session.State().NeedsSetup {
				t.Error("le poste ne demande plus sa configuration alors qu'elle a échoué")
			}
			if db.Users.Count() != 0 {
				t.Error("un compte a été créé malgré le refus")
			}
		})
	}
}

func TestSetup_UneSeuleFois(t *testing.T) {
	_, _, session := newBlank(t)
	if _, err := session.Setup(validSetup()); err != nil {
		t.Fatalf("première configuration : %v", err)
	}
	in := validSetup()
	in.Username = "intrus"
	if _, err := session.Setup(in); err == nil {
		t.Error("un poste déjà configuré ne doit pas pouvoir l'être une seconde fois")
	}
}

// La mention de paternité doit être servie à l'interface dès le premier écran,
// avant même qu'un compte existe.
func TestBrand_DisponibleAvantToutCompte(t *testing.T) {
	_, _, session := newBlank(t)

	a := session.Brand()
	if !a.Intact {
		t.Fatalf("identité visuelle altérée : %s", a.Alert)
	}
	if a.Author != brand.Author {
		t.Errorf("auteur = %q, attendu %q", a.Author, brand.Author)
	}
	if !strings.HasPrefix(a.LogoDataURL, "data:image/png;base64,") {
		t.Error("le logo n'est pas transmis sous une forme affichable")
	}

	state := session.State()
	if state.Author != brand.Author || state.Notice == "" {
		t.Errorf("la mention n'accompagne pas l'état : %+v", state)
	}
	if !state.BrandingIntact {
		t.Error("l'état signale une identité visuelle altérée")
	}

	// L'état est relu toutes les minutes : il ne doit pas charrier l'image.
	if strings.Contains(state.Notice, "base64") || len(state.Notice) > 200 {
		t.Error("l'état transporte l'image : il est relu trop souvent pour cela")
	}
}

func TestSetup_MonnaiesProposees(t *testing.T) {
	_, _, session := newBlank(t)
	list := session.Currencies()
	if len(list) < 3 {
		t.Fatalf("%d monnaie(s) proposée(s)", len(list))
	}
	var xof bool
	for _, c := range list {
		if c.Code == "XOF" {
			xof = true
			if c.Decimals != 0 {
				t.Errorf("le franc CFA a %d décimale(s), attendu 0", c.Decimals)
			}
		}
	}
	if !xof {
		t.Error("le franc CFA ne figure pas dans les monnaies proposées")
	}
	if len(session.DefaultCategoryNames()) == 0 {
		t.Error("aucune catégorie de départ n'est proposée")
	}
}
