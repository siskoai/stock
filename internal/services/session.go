package services

import (
	"fmt"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/brand"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Session est la façade d'authentification exposée à l'interface. C'est le seul
// service dont les méthodes s'appellent sans session ouverte.
type Session struct{ core }

// NewSession construit le service de session.
func NewSession(db *storage.Database, a *auth.Service) *Session {
	return &Session{core{db: db, auth: a}}
}

// State décrit ce que l'interface doit savoir pour choisir son écran d'accueil.
type State struct {
	// NeedsSetup est vrai au tout premier lancement : aucun compte n'existe
	// encore, l'interface propose la création de l'administrateur.
	NeedsSetup bool `json:"needsSetup"`

	Authenticated bool               `json:"authenticated"`
	User          *models.PublicUser `json:"user,omitempty"`
	Scopes        []string           `json:"scopes"`

	CompanyName    string `json:"companyName"`
	CurrencySymbol string `json:"currencySymbol"`
	Decimals       int    `json:"decimals"`
	Theme          string `json:"theme"`
	AppVersion     string `json:"appVersion"`

	// Author et Notice portent la mention de paternité prévue par la licence.
	// Le logo lui-même n'est pas transmis ici : il pèse près d'un mégaoctet et
	// cet état est relu toutes les minutes. L'interface le demande une fois,
	// au démarrage, par Brand().
	Author         string `json:"author"`
	Notice         string `json:"notice"`
	BrandingIntact bool   `json:"brandingIntact"`
}

// Version est renseignée à la compilation (voir Makefile / build).
var Version = "dev"

// State renvoie l'état courant sans jamais échouer : une session expirée est un
// état, pas une erreur.
func (s *Session) State() State {
	settings := s.db.Settings()
	st := State{
		NeedsSetup:     !s.auth.HasUsers(),
		Scopes:         []string{},
		CompanyName:    settings.CompanyName,
		CurrencySymbol: settings.CurrencySymbol,
		Decimals:       settings.Decimals,
		Theme:          settings.Theme,
		AppVersion:     Version,
		Author:         brand.Author,
		Notice:         brand.Notice,
		BrandingIntact: brand.Verify() == nil,
	}
	if u, err := s.auth.Current(); err == nil {
		pub := u.Sanitized()
		st.Authenticated = true
		st.User = &pub
		st.Scopes = auth.ScopesFor(u.Role)
	}
	return st
}

// Brand renvoie l'identité visuelle de l'auteur : le logo, la mention de
// paternité, et l'état de son intégrité.
//
// L'image vient du binaire, pas des fichiers d'interface : c'est ce qui rend
// son remplacement impossible sans modifier le code source et recompiler.
// Voir internal/brand et l'article 3 de la licence.
func (s *Session) Brand() brand.Attribution { return brand.Current() }

// Currencies renvoie les monnaies proposées. Sans contrôle de session :
// l'assistant de premier démarrage en a besoin avant qu'un compte existe.
func (s *Session) Currencies() []CurrencyPreset { return Currencies }

// DefaultCategoryNames renvoie les catégories que l'assistant propose d'amorcer,
// pour qu'il puisse les montrer avant de les créer.
func (s *Session) DefaultCategoryNames() []string {
	out := make([]string, 0, len(models.DefaultCategories))
	for _, c := range models.DefaultCategories {
		out = append(out, c.Name)
	}
	return out
}

// SetupInput porte l'ensemble des réponses de l'assistant de premier démarrage.
// Tout arrive d'un coup : chaque étape de l'assistant remplit une partie, et
// rien n'est écrit tant que la dernière n'est pas validée.
type SetupInput struct {
	// Compte administrateur.
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Password string `json:"password"`

	// Identité de l'entreprise.
	CompanyName string `json:"companyName"`
	LegalForm   string `json:"legalForm"`
	TaxID       string `json:"taxId"`
	RCCM        string `json:"rccm"`
	Address     string `json:"address"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`

	// Monnaie et taxes.
	Currency         string  `json:"currency"`
	CurrencySymbol   string  `json:"currencySymbol"`
	Decimals         int     `json:"decimals"`
	DefaultTaxRate   float64 `json:"defaultTaxRate"`
	PricesIncludeTax bool    `json:"pricesIncludeTax"`

	// Catalogue de départ : les catégories courantes d'une boutique
	// d'informatique, que l'utilisateur pourra renommer ou supprimer.
	SeedCategories bool `json:"seedCategories"`

	// Sauvegardes.
	AutoBackup    bool `json:"autoBackup"`
	BackupsToKeep int  `json:"backupsToKeep"`

	// Thème de l'interface.
	Theme string `json:"theme"`
}

// Setup applique les réponses de l'assistant de premier démarrage.
//
// L'ordre est délibéré : tout est validé avant la moindre écriture, puis les
// paramètres sont enregistrés, puis le compte est créé. Un échec en cours de
// route laisse ainsi le poste à l'état « premier démarrage », où l'assistant
// peut être relancé, plutôt qu'à moitié configuré.
func (s *Session) Setup(in SetupInput) (State, error) {
	if s.auth.HasUsers() {
		return s.State(), fmt.Errorf("ce poste est déjà configuré")
	}
	// Validation du compte avant toute écriture : c'est la partie que
	// l'utilisateur ne peut pas corriger après coup sans repartir de zéro.
	if err := auth.ValidatePassword(in.Password); err != nil {
		return s.State(), err
	}
	company, err := requireText(in.CompanyName, "Nom de l'entreprise")
	if err != nil {
		return s.State(), err
	}
	if in.DefaultTaxRate < 0 || in.DefaultTaxRate > 100 {
		return s.State(), fmt.Errorf("le taux de taxe doit être compris entre 0 et 100 %%")
	}
	if in.Decimals < 0 || in.Decimals > 2 {
		return s.State(), fmt.Errorf("le nombre de décimales doit valoir 0, 1 ou 2")
	}

	settings := models.DefaultSettings()
	settings.CompanyName = company
	settings.LegalForm = trim(in.LegalForm)
	settings.TaxID, settings.RCCM = trim(in.TaxID), trim(in.RCCM)
	settings.Address, settings.City = trim(in.Address), trim(in.City)
	settings.Country = defaultString(in.Country, settings.Country)
	settings.Phone, settings.Email = trim(in.Phone), trim(in.Email)
	settings.Currency = defaultString(in.Currency, settings.Currency)
	settings.CurrencySymbol = defaultString(in.CurrencySymbol, settings.Currency)
	settings.Decimals = in.Decimals
	settings.DefaultTaxRate = in.DefaultTaxRate
	settings.PricesIncludeTax = in.PricesIncludeTax
	settings.AutoBackup = in.AutoBackup
	settings.BackupsToKeep = maxInt(in.BackupsToKeep, 1)
	settings.Theme = defaultString(in.Theme, "light")
	if err := s.db.SaveSettings(settings); err != nil {
		return s.State(), err
	}

	u, err := s.auth.CreateFirstAdmin(in.Username, in.FullName, in.Password)
	if err != nil {
		return s.State(), err
	}
	if in.SeedCategories {
		now := time.Now()
		for _, c := range models.DefaultCategories {
			_ = s.db.Categories.Insert(models.Category{
				ID: util.NewID("cat"), Name: c.Name, Description: c.Description,
				Color: c.Color, CreatedAt: now, UpdatedAt: now,
			})
		}
	}
	if _, err := s.auth.Login(in.Username, in.Password); err != nil {
		return s.State(), err
	}
	s.log(u, "SETUP", "user", u.ID,
		"Premier démarrage : « %s », monnaie %s, taxe %.2f %%, administrateur « %s »",
		company, settings.CurrencySymbol, settings.DefaultTaxRate, u.Username)
	return s.State(), nil
}

// Login ouvre une session.
func (s *Session) Login(username, password string) (State, error) {
	pub, err := s.auth.Login(username, password)
	if err != nil {
		return s.State(), err
	}
	if u, err := s.auth.Get(pub.ID); err == nil {
		s.log(u, "LOGIN", "user", u.ID, "Connexion de « %s »", u.Username)
	}
	return s.State(), nil
}

// Logout ferme la session.
func (s *Session) Logout() State {
	if u, err := s.auth.Current(); err == nil {
		s.log(u, "LOGOUT", "user", u.ID, "Déconnexion de « %s »", u.Username)
	}
	s.auth.Logout()
	return s.State()
}

// Touch prolonge la session sur interaction de l'utilisateur et signale son
// expiration éventuelle.
func (s *Session) Touch() State { return s.State() }

// ChangePassword modifie le mot de passe du compte connecté. Accessible même
// lorsque le compte est marqué « mot de passe à changer » : c'est justement la
// seule action alors permise.
func (s *Session) ChangePassword(oldPwd, newPwd string) (State, error) {
	if oldPwd == newPwd {
		return s.State(), fmt.Errorf("le nouveau mot de passe doit être différent de l'ancien")
	}
	if err := s.auth.ChangePassword(oldPwd, newPwd); err != nil {
		return s.State(), err
	}
	if u, err := s.auth.Current(); err == nil {
		s.log(u, "PASSWORD", "user", u.ID, "Mot de passe modifié par son titulaire")
	}
	return s.State(), nil
}
