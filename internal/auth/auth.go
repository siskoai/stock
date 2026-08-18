// Package auth gère l'authentification locale et le contrôle d'accès.
//
// Modèle de menace retenu (application 100 % locale, sans réseau) :
//   - protéger les données contre un utilisateur non autorisé qui s'assoit
//     devant le poste ;
//   - empêcher un vendeur de modifier les prix d'achat, les charges ou les
//     comptes ;
//   - garder une trace de qui a fait quoi.
//
// Ce que ce modèle ne prétend pas faire : résister à quelqu'un qui a un accès
// administrateur au système de fichiers. Les fichiers de données sont du JSON
// lisible, par choix (inspection et récupération manuelles possibles). Le
// chiffrement du disque relève du système (BitLocker sous Windows) et c'est la
// bonne couche pour le faire — voir docs/SECURITE.md.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

var (
	ErrInvalidCredentials = errors.New("identifiant ou mot de passe incorrect")
	ErrAccountDisabled    = errors.New("ce compte est désactivé")
	ErrNotAuthenticated   = errors.New("session expirée, veuillez vous reconnecter")
	ErrForbidden          = errors.New("votre rôle ne permet pas cette action")
	ErrWeakPassword       = errors.New("le mot de passe doit contenir au moins 8 caractères, dont une lettre et un chiffre")
	ErrLockedOut          = errors.New("trop de tentatives échouées, réessayez dans une minute")
	ErrMustChangePassword = errors.New("vous devez définir un nouveau mot de passe avant de continuer")
)

// Cost est le nombre de tours bcrypt appliqué aux mots de passe. Douze tours
// représentent environ 250 ms sur une machine de bureau courante : assez lent
// pour rendre une attaque par dictionnaire hors ligne coûteuse, assez rapide
// pour que la connexion reste instantanée à l'usage.
//
// C'est une variable et non une constante pour deux raisons : un poste très
// lent peut avoir besoin de l'abaisser, et une suite de tests n'a pas à passer
// l'essentiel de son temps à hacher. Elle se règle au démarrage, jamais en
// cours d'exécution. Les valeurs hors bornes sont ramenées au défaut.
var Cost = 12

const (
	maxFailedAttempts = 5
	lockoutDuration   = time.Minute
)

// Service porte l'état de session de l'application. Une seule session active à
// la fois : c'est une application de poste, pas un serveur.
type Service struct {
	mu    sync.RWMutex
	users *storage.Collection[models.User]

	current  *models.User
	lastSeen time.Time
	timeout  time.Duration
	failures map[string]*failureState
}

type failureState struct {
	count      int
	lockedTill time.Time
}

// New construit le service d'authentification.
func New(users *storage.Collection[models.User], timeoutMinutes int) *Service {
	if timeoutMinutes <= 0 {
		timeoutMinutes = 60
	}
	return &Service{
		users:    users,
		timeout:  time.Duration(timeoutMinutes) * time.Minute,
		failures: map[string]*failureState{},
	}
}

// SetTimeout met à jour le délai d'inactivité après changement de paramètres.
func (s *Service) SetTimeout(minutes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if minutes > 0 {
		s.timeout = time.Duration(minutes) * time.Minute
	}
}

// HasUsers indique si au moins un compte existe : sert à afficher l'écran de
// premier démarrage plutôt que l'écran de connexion.
func (s *Service) HasUsers() bool { return s.users.Count() > 0 }

// ---------------------------------------------------------------------------
// Mots de passe
// ---------------------------------------------------------------------------

// ValidatePassword applique la politique de mot de passe.
func ValidatePassword(pwd string) error {
	if len([]rune(pwd)) < 8 {
		return ErrWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range pwd {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}

// HashPassword produit un hash bcrypt salé.
//
// Le coût est inscrit dans le hash produit : abaisser Cost n'affaiblit pas les
// mots de passe déjà enregistrés, et les remonter ne renforce que les suivants.
func HashPassword(pwd string) (string, error) {
	cost := Cost
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = 12
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pwd), cost)
	if err != nil {
		return "", fmt.Errorf("chiffrement du mot de passe : %w", err)
	}
	return string(h), nil
}

// ---------------------------------------------------------------------------
// Connexion
// ---------------------------------------------------------------------------

// CreateFirstAdmin crée le compte administrateur initial. Refusé si des comptes
// existent déjà : cette porte ne peut pas servir d'escalade de privilèges.
func (s *Service) CreateFirstAdmin(username, fullName, password string) (models.User, error) {
	if s.users.Count() > 0 {
		return models.User{}, errors.New("un compte existe déjà sur ce poste")
	}
	return s.createUser(username, fullName, password, models.RoleAdmin)
}

// CreateUser ajoute un compte. Le contrôle de rôle appartient à l'appelant.
func (s *Service) CreateUser(username, fullName, password string, role models.Role) (models.User, error) {
	return s.createUser(username, fullName, password, role)
}

func (s *Service) createUser(username, fullName, password string, role models.Role) (models.User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(username) < 3 {
		return models.User{}, errors.New("l'identifiant doit contenir au moins 3 caractères")
	}
	if strings.TrimSpace(fullName) == "" {
		return models.User{}, errors.New("le nom complet est obligatoire")
	}
	if err := ValidatePassword(password); err != nil {
		return models.User{}, err
	}
	if _, err := s.users.FindOne(func(u models.User) bool { return u.Username == username }); err == nil {
		return models.User{}, fmt.Errorf("l'identifiant « %s » est déjà pris", username)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return models.User{}, err
	}
	now := time.Now()
	u := models.User{
		ID:           util.NewID("usr"),
		Username:     username,
		FullName:     strings.TrimSpace(fullName),
		Role:         role,
		PasswordHash: hash,
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Insert(u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

// Login vérifie les identifiants et ouvre une session.
func (s *Service) Login(username, password string) (models.PublicUser, error) {
	username = strings.ToLower(strings.TrimSpace(username))

	if err := s.checkLockout(username); err != nil {
		return models.PublicUser{}, err
	}

	u, err := s.users.FindOne(func(u models.User) bool { return u.Username == username })
	if err != nil {
		// Comparaison factice pour que le temps de réponse ne révèle pas
		// l'existence du compte.
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$12$aaaaaaaaaaaaaaaaaaaaaeZ7yQ7g4rJLKR8vXqB9pJ.YCtQZP0Wnq"), []byte(password))
		s.recordFailure(username)
		return models.PublicUser{}, ErrInvalidCredentials
	}
	if !u.Active {
		return models.PublicUser{}, ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		s.recordFailure(username)
		return models.PublicUser{}, ErrInvalidCredentials
	}

	now := time.Now()
	u.LastLogin = &now
	u.UpdatedAt = now
	if err := s.users.Update(u); err != nil {
		return models.PublicUser{}, err
	}

	s.mu.Lock()
	s.current = &u
	s.lastSeen = now
	delete(s.failures, username)
	s.mu.Unlock()

	return u.Sanitized(), nil
}

func (s *Service) checkLockout(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.failures[username]
	if !ok {
		return nil
	}
	if time.Now().Before(st.lockedTill) {
		return ErrLockedOut
	}
	if !st.lockedTill.IsZero() {
		// La fenêtre de blocage est passée : on repart de zéro.
		delete(s.failures, username)
	}
	return nil
}

func (s *Service) recordFailure(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.failures[username]
	if !ok {
		st = &failureState{}
		s.failures[username] = st
	}
	st.count++
	if st.count >= maxFailedAttempts {
		st.lockedTill = time.Now().Add(lockoutDuration)
		st.count = 0
	}
}

// Logout ferme la session courante.
func (s *Service) Logout() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = nil
}

// Current renvoie l'utilisateur connecté, en vérifiant le délai d'inactivité.
// Toute méthode exposée à l'interface passe par ici avant d'agir.
func (s *Service) Current() (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return models.User{}, ErrNotAuthenticated
	}
	if time.Since(s.lastSeen) > s.timeout {
		s.current = nil
		return models.User{}, ErrNotAuthenticated
	}
	s.lastSeen = time.Now() // toute activité repousse l'expiration
	return *s.current, nil
}

// Refresh prolonge la session sans effet de bord ; appelé par l'interface sur
// interaction utilisateur.
func (s *Service) Refresh() error {
	_, err := s.Current()
	return err
}

// ---------------------------------------------------------------------------
// Contrôle d'accès
// ---------------------------------------------------------------------------

// permissions décrit, pour chaque rôle, les domaines accessibles en écriture.
// La lecture du catalogue et du stock est ouverte à tous les rôles connectés.
var permissions = map[models.Role]map[string]bool{
	models.RoleAdmin: {
		"catalog": true, "stock": true, "sales": true, "purchases": true,
		"expenses": true, "finance": true, "users": true, "settings": true,
		"backup": true, "delete": true,
	},
	models.RoleManager: {
		"catalog": true, "stock": true, "sales": true, "purchases": true,
		"expenses": true, "finance": true, "backup": true, "delete": true,
	},
	models.RoleSeller: {
		"sales": true,
	},
}

// Require vérifie qu'une session valide existe et que le rôle couvre le domaine
// demandé. Renvoie l'utilisateur pour l'attribution et le journal d'audit.
//
// Ce contrôle est appliqué côté Go, pas seulement dans l'interface : masquer un
// bouton n'est pas une protection, refuser l'appel en est une.
func (s *Service) Require(scope string) (models.User, error) {
	u, err := s.Current()
	if err != nil {
		return models.User{}, err
	}
	// Un mot de passe réinitialisé par un administrateur ne donne accès à rien
	// tant qu'il n'a pas été remplacé : sinon le mot de passe provisoire, qui
	// transite verbalement, resterait une clé utilisable indéfiniment.
	if u.MustChangePwd {
		return models.User{}, ErrMustChangePassword
	}
	if scope == "" {
		return u, nil // lecture simple : être connecté suffit
	}
	if permissions[u.Role][scope] {
		return u, nil
	}
	return models.User{}, ErrForbidden
}

// Can indique si le rôle couvre un domaine ; utilisé pour piloter l'affichage.
func Can(role models.Role, scope string) bool { return permissions[role][scope] }

// ScopesFor renvoie les domaines autorisés d'un rôle, transmis à l'interface au
// moment de la connexion pour qu'elle n'affiche que ce qui est réellement
// utilisable.
func ScopesFor(role models.Role) []string {
	out := []string{}
	for scope, ok := range permissions[role] {
		if ok {
			out = append(out, scope)
		}
	}
	return out
}

// ChangePassword modifie le mot de passe de l'utilisateur connecté après
// vérification de l'ancien.
func (s *Service) ChangePassword(oldPwd, newPwd string) error {
	u, err := s.Current()
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPwd)); err != nil {
		return errors.New("le mot de passe actuel est incorrect")
	}
	if err := ValidatePassword(newPwd); err != nil {
		return err
	}
	hash, err := HashPassword(newPwd)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	u.MustChangePwd = false
	u.UpdatedAt = time.Now()
	if err := s.users.Update(u); err != nil {
		return err
	}
	s.mu.Lock()
	s.current = &u
	s.mu.Unlock()
	return nil
}

// ResetPassword permet à un administrateur de redéfinir le mot de passe d'un
// autre compte. Le contrôle de rôle appartient à l'appelant.
//
// Un mot de passe vide fait générer un mot de passe provisoire, renvoyé en
// clair une seule fois pour être transmis de vive voix. Dans les deux cas le
// compte est marqué MustChangePwd : le mot de passe reçu ne sert qu'à ouvrir
// la session pendant laquelle il sera remplacé.
func (s *Service) ResetPassword(userID, newPwd string) (string, error) {
	generated := ""
	if strings.TrimSpace(newPwd) == "" {
		pwd, err := util.TempPassword()
		if err != nil {
			return "", err
		}
		newPwd, generated = pwd, pwd
	}
	if err := ValidatePassword(newPwd); err != nil {
		return "", err
	}
	u, err := s.users.Get(userID)
	if err != nil {
		return "", err
	}
	hash, err := HashPassword(newPwd)
	if err != nil {
		return "", err
	}
	u.PasswordHash = hash
	u.MustChangePwd = true
	u.UpdatedAt = time.Now()
	if err := s.users.Update(u); err != nil {
		return "", err
	}
	return generated, nil
}

// SetActive active ou désactive un compte. Le contrôle de rôle appartient à
// l'appelant. Un compte désactivé ne peut plus ouvrir de session.
func (s *Service) SetActive(userID string, active bool) (models.User, error) {
	u, err := s.users.Get(userID)
	if err != nil {
		return models.User{}, err
	}
	u.Active = active
	u.UpdatedAt = time.Now()
	if err := s.users.Update(u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

// UpdateProfile modifie le nom et le rôle d'un compte. Le contrôle de rôle
// appartient à l'appelant.
func (s *Service) UpdateProfile(userID, fullName string, role models.Role) (models.User, error) {
	u, err := s.users.Get(userID)
	if err != nil {
		return models.User{}, err
	}
	if strings.TrimSpace(fullName) == "" {
		return models.User{}, errors.New("le nom complet est obligatoire")
	}
	if _, ok := permissions[role]; !ok {
		return models.User{}, errors.New("rôle inconnu")
	}
	u.FullName = strings.TrimSpace(fullName)
	u.Role = role
	u.UpdatedAt = time.Now()
	if err := s.users.Update(u); err != nil {
		return models.User{}, err
	}
	// Le rôle de l'utilisateur connecté a pu changer : la session en mémoire
	// doit refléter le nouveau rôle immédiatement, sans reconnexion.
	s.mu.Lock()
	if s.current != nil && s.current.ID == u.ID {
		s.current = &u
	}
	s.mu.Unlock()
	return u, nil
}

// List renvoie tous les comptes, triés par identifiant. Le contrôle de rôle
// appartient à l'appelant.
func (s *Service) List() []models.User { return s.users.All() }

// Get renvoie un compte par identifiant.
func (s *Service) Get(userID string) (models.User, error) { return s.users.Get(userID) }
