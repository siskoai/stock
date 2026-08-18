package services

import (
	"fmt"
	"time"

	"comptoir/internal/auth"
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
	}
	if u, err := s.auth.Current(); err == nil {
		pub := u.Sanitized()
		st.Authenticated = true
		st.User = &pub
		st.Scopes = auth.ScopesFor(u.Role)
	}
	return st
}

// Setup crée le compte administrateur au premier lancement et amorce le
// catalogue avec les catégories par défaut.
func (s *Session) Setup(username, fullName, password string) (State, error) {
	u, err := s.auth.CreateFirstAdmin(username, fullName, password)
	if err != nil {
		return s.State(), err
	}
	now := time.Now()
	for _, c := range models.DefaultCategories {
		_ = s.db.Categories.Insert(models.Category{
			ID: util.NewID("cat"), Name: c.Name, Description: c.Description,
			Color: c.Color, CreatedAt: now, UpdatedAt: now,
		})
	}
	if _, err := s.auth.Login(username, password); err != nil {
		return s.State(), err
	}
	s.log(u, "SETUP", "user", u.ID, "Premier démarrage : compte administrateur « %s » créé", u.Username)
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
