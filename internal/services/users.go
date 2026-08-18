package services

import (
	"fmt"
	"sort"
	"strings"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Users administre les comptes du poste. Réservé au rôle Administrateur : la
// gestion des comptes est le seul domaine qu'un gérant ne touche pas, sans quoi
// il pourrait s'accorder lui-même des droits.
type Users struct{ core }

// NewUsers construit le service des comptes.
func NewUsers(db *storage.Database, a *auth.Service) *Users {
	return &Users{core{db: db, auth: a}}
}

// UserView est un compte tel qu'affiché dans l'interface, enrichi de son
// activité commerciale.
type UserView struct {
	models.PublicUser
	InvoiceCount int      `json:"invoiceCount"`
	Revenue      int64    `json:"revenue"`
	Scopes       []string `json:"scopes"`
	IsCurrent    bool     `json:"isCurrent"`
}

// UserInput est la charge utile de création ou de modification d'un compte.
type UserInput struct {
	ID       string      `json:"id"`
	Username string      `json:"username"`
	FullName string      `json:"fullName"`
	Password string      `json:"password"` // création uniquement
	Role     models.Role `json:"role"`
}

// RoleInfo décrit un rôle pour l'écran de gestion des comptes.
type RoleInfo struct {
	Role        models.Role `json:"role"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Scopes      []string    `json:"scopes"`
}

// Roles renvoie les rôles disponibles et ce qu'ils autorisent, pour que le
// choix se fasse en connaissance de cause.
func (s *Users) Roles() ([]RoleInfo, error) {
	if _, err := s.guard("users"); err != nil {
		return nil, err
	}
	return []RoleInfo{
		{models.RoleAdmin, "Administrateur",
			"Accès complet, y compris les comptes, les paramètres et les sauvegardes.",
			auth.ScopesFor(models.RoleAdmin)},
		{models.RoleManager, "Gérant",
			"Stock, ventes, achats, charges et rapports financiers. Ne gère pas les comptes.",
			auth.ScopesFor(models.RoleManager)},
		{models.RoleSeller, "Vendeur",
			"Ventes et consultation du stock. Ne voit ni les prix d'achat, ni les marges, ni les charges.",
			auth.ScopesFor(models.RoleSeller)},
	}, nil
}

// List renvoie tous les comptes, l'actif en premier puis par identifiant.
func (s *Users) List() ([]UserView, error) {
	current, err := s.guard("users")
	if err != nil {
		return nil, err
	}
	activity := map[string]*UserView{}
	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) {
			continue
		}
		v, ok := activity[inv.UserID]
		if !ok {
			v = &UserView{}
			activity[inv.UserID] = v
		}
		v.InvoiceCount++
		v.Revenue += inv.SubtotalHT - inv.GlobalDiscount
	}

	out := []UserView{}
	for _, u := range s.auth.List() {
		v := UserView{
			PublicUser: u.Sanitized(),
			Scopes:     auth.ScopesFor(u.Role),
			IsCurrent:  u.ID == current.ID,
		}
		if a, ok := activity[u.ID]; ok {
			v.InvoiceCount, v.Revenue = a.InvoiceCount, a.Revenue
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		return util.Slug(out[i].Username) < util.Slug(out[j].Username)
	})
	return out, nil
}

// Create ajoute un compte.
func (s *Users) Create(in UserInput) (UserView, error) {
	admin, err := s.guard("users")
	if err != nil {
		return UserView{}, err
	}
	if _, ok := roleLabels[in.Role]; !ok {
		return UserView{}, fmt.Errorf("choisissez un rôle pour ce compte")
	}
	u, err := s.auth.CreateUser(in.Username, in.FullName, in.Password, in.Role)
	if err != nil {
		return UserView{}, err
	}
	s.log(admin, "CREATE", "user", u.ID, "Compte « %s » créé avec le rôle %s", u.Username, roleLabels[u.Role])
	return UserView{PublicUser: u.Sanitized(), Scopes: auth.ScopesFor(u.Role)}, nil
}

// Update modifie le nom et le rôle d'un compte.
//
// Un administrateur ne peut pas se retirer à lui-même le droit de gérer les
// comptes, ni retirer le dernier administrateur du poste : personne ne doit
// pouvoir se verrouiller dehors.
func (s *Users) Update(in UserInput) (UserView, error) {
	admin, err := s.guard("users")
	if err != nil {
		return UserView{}, err
	}
	target, err := s.auth.Get(in.ID)
	if err != nil {
		return UserView{}, err
	}
	if _, ok := roleLabels[in.Role]; !ok {
		return UserView{}, fmt.Errorf("choisissez un rôle pour ce compte")
	}
	if target.Role == models.RoleAdmin && in.Role != models.RoleAdmin {
		if err := s.ensureAnotherAdmin(target.ID); err != nil {
			return UserView{}, err
		}
	}
	u, err := s.auth.UpdateProfile(in.ID, in.FullName, in.Role)
	if err != nil {
		return UserView{}, err
	}
	s.log(admin, "UPDATE", "user", u.ID, "Compte « %s » : rôle %s", u.Username, roleLabels[u.Role])
	return UserView{PublicUser: u.Sanitized(), Scopes: auth.ScopesFor(u.Role), IsCurrent: u.ID == admin.ID}, nil
}

// SetActive active ou désactive un compte.
func (s *Users) SetActive(id string, active bool) (UserView, error) {
	admin, err := s.guard("users")
	if err != nil {
		return UserView{}, err
	}
	if !active {
		if id == admin.ID {
			return UserView{}, fmt.Errorf("vous ne pouvez pas désactiver votre propre compte")
		}
		if target, err := s.auth.Get(id); err == nil && target.Role == models.RoleAdmin {
			if err := s.ensureAnotherAdmin(id); err != nil {
				return UserView{}, err
			}
		}
	}
	u, err := s.auth.SetActive(id, active)
	if err != nil {
		return UserView{}, err
	}
	état := "désactivé"
	if active {
		état = "réactivé"
	}
	s.log(admin, "UPDATE", "user", u.ID, "Compte « %s » %s", u.Username, état)
	return UserView{PublicUser: u.Sanitized(), Scopes: auth.ScopesFor(u.Role)}, nil
}

// ResetPassword redéfinit le mot de passe d'un compte. Sans mot de passe
// fourni, un mot de passe provisoire est généré et renvoyé une seule fois : il
// devra être remplacé à la première connexion.
func (s *Users) ResetPassword(id, newPassword string) (string, error) {
	admin, err := s.guard("users")
	if err != nil {
		return "", err
	}
	target, err := s.auth.Get(id)
	if err != nil {
		return "", err
	}
	generated, err := s.auth.ResetPassword(id, newPassword)
	if err != nil {
		return "", err
	}
	s.log(admin, "PASSWORD", "user", id, "Mot de passe du compte « %s » réinitialisé", target.Username)
	return generated, nil
}

// Delete supprime un compte sans activité commerciale. Un compte qui a émis des
// factures est désactivé, jamais supprimé : son nom figure sur des documents.
func (s *Users) Delete(id string) error {
	admin, err := s.guard("users")
	if err != nil {
		return err
	}
	if id == admin.ID {
		return fmt.Errorf("vous ne pouvez pas supprimer votre propre compte")
	}
	target, err := s.auth.Get(id)
	if err != nil {
		return err
	}
	if target.Role == models.RoleAdmin {
		if err := s.ensureAnotherAdmin(id); err != nil {
			return err
		}
	}
	n := len(s.db.Invoices.Find(func(i models.Invoice) bool { return i.UserID == id })) +
		len(s.db.Purchases.Find(func(p models.Purchase) bool { return p.UserID == id }))
	if n > 0 {
		return fmt.Errorf("« %s » figure sur %d document(s) : désactivez ce compte plutôt que de le supprimer",
			target.Username, n)
	}
	if err := s.db.Users.Delete(id); err != nil {
		return err
	}
	s.log(admin, "DELETE", "user", id, "Compte « %s » supprimé", target.Username)
	return nil
}

// ensureAnotherAdmin vérifie qu'un autre administrateur actif subsiste.
func (s *Users) ensureAnotherAdmin(excludeID string) error {
	for _, u := range s.auth.List() {
		if u.ID != excludeID && u.Role == models.RoleAdmin && u.Active {
			return nil
		}
	}
	return fmt.Errorf("ce compte est le dernier administrateur actif : désignez d'abord un autre administrateur")
}

// ---------------------------------------------------------------------------
// Journal d'audit
// ---------------------------------------------------------------------------

// AuditQuery filtre le journal d'audit.
type AuditQuery struct {
	Search string `json:"search"`
	Action string `json:"action"`
	Entity string `json:"entity"`
	UserID string `json:"userId"`
	From   string `json:"from"`
	To     string `json:"to"`
	Limit  int    `json:"limit"`
}

// Audit renvoie le journal d'audit, du plus récent au plus ancien.
func (s *Users) Audit(q AuditQuery) ([]models.AuditEntry, error) {
	if _, err := s.guard("users"); err != nil {
		return nil, err
	}
	from, to, err := parseRange(q.From, q.To)
	if err != nil {
		return nil, err
	}
	out := []models.AuditEntry{}
	for _, e := range s.db.Audit.All() {
		if q.Action != "" && e.Action != q.Action {
			continue
		}
		if q.Entity != "" && e.Entity != q.Entity {
			continue
		}
		if q.UserID != "" && e.UserID != q.UserID {
			continue
		}
		if !util.InRange(e.At, from, to) {
			continue
		}
		if q.Search != "" && !(util.Contains(e.Details, q.Search) ||
			util.Contains(e.UserName, q.Search) || util.Contains(e.Entity, q.Search)) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// AuditActions renvoie les types d'action présents dans le journal, pour
// alimenter le filtre de l'interface.
func (s *Users) AuditActions() ([]string, error) {
	if _, err := s.guard("users"); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, e := range s.db.Audit.All() {
		if !seen[e.Action] {
			seen[e.Action] = true
			out = append(out, e.Action)
		}
	}
	sort.Strings(out)
	return out, nil
}

// ---------------------------------------------------------------------------

var roleLabels = map[models.Role]string{
	models.RoleAdmin:   "Administrateur",
	models.RoleManager: "Gérant",
	models.RoleSeller:  "Vendeur",
}

// RoleLabel traduit un rôle pour l'affichage et les journaux.
func RoleLabel(r models.Role) string {
	if l, ok := roleLabels[r]; ok {
		return l
	}
	return strings.ToUpper(string(r))
}
