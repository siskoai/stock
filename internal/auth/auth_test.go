package auth

import (
	"errors"
	"testing"
	"time"

	"comptoir/internal/models"
	"comptoir/internal/storage"
)

func newService(t *testing.T) *Service {
	t.Helper()
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	return New(db.Users, 60)
}

func TestValidatePassword(t *testing.T) {
	refuses := []string{"", "court1", "quesdeslettres", "12345678", "élève12"[:6]}
	for _, pwd := range refuses {
		if err := ValidatePassword(pwd); !errors.Is(err, ErrWeakPassword) {
			t.Errorf("ValidatePassword(%q) = %v, attendu ErrWeakPassword", pwd, err)
		}
	}
	for _, pwd := range []string{"motdepasse1", "Ab3456789", "élève2026"} {
		if err := ValidatePassword(pwd); err != nil {
			t.Errorf("ValidatePassword(%q) = %v, attendu nil", pwd, err)
		}
	}
}

func TestCreateFirstAdmin_UneSeuleFois(t *testing.T) {
	s := newService(t)
	if _, err := s.CreateFirstAdmin("patron", "La Patronne", "motdepasse1"); err != nil {
		t.Fatalf("création : %v", err)
	}
	if _, err := s.CreateFirstAdmin("intrus", "L'Intrus", "motdepasse1"); err == nil {
		t.Error("la création du premier administrateur ne doit pas pouvoir servir d'escalade de privilèges")
	}
}

func TestCreateUser_Validations(t *testing.T) {
	s := newService(t)
	if _, err := s.CreateUser("ab", "Trop court", "motdepasse1", models.RoleSeller); err == nil {
		t.Error("un identifiant de moins de 3 caractères aurait dû être refusé")
	}
	if _, err := s.CreateUser("valide", "  ", "motdepasse1", models.RoleSeller); err == nil {
		t.Error("un nom complet vide aurait dû être refusé")
	}
	if _, err := s.CreateUser("valide", "Nom", "faible", models.RoleSeller); err == nil {
		t.Error("un mot de passe faible aurait dû être refusé")
	}
	if _, err := s.CreateUser("Vendeur", "Le Vendeur", "motdepasse1", models.RoleSeller); err != nil {
		t.Fatalf("création : %v", err)
	}
	// L'identifiant est normalisé en minuscules : « Vendeur » et « vendeur »
	// sont le même compte.
	if _, err := s.CreateUser("vendeur", "Un autre", "motdepasse1", models.RoleSeller); err == nil {
		t.Error("un identifiant déjà pris, à la casse près, aurait dû être refusé")
	}
}

func TestLogin(t *testing.T) {
	s := newService(t)
	if _, err := s.CreateFirstAdmin("patron", "La Patronne", "motdepasse1"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Login("patron", "mauvais1234"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("mauvais mot de passe : %v, attendu ErrInvalidCredentials", err)
	}
	if _, err := s.Login("inconnu", "motdepasse1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("compte inconnu : %v, le message ne doit pas révéler que le compte n'existe pas", err)
	}
	pub, err := s.Login("PATRON", "motdepasse1")
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	if pub.Username != "patron" {
		t.Errorf("identifiant = %q, attendu « patron »", pub.Username)
	}
	if pub.LastLogin == nil {
		t.Error("la dernière connexion n'est pas enregistrée")
	}
	if u, err := s.Current(); err != nil || u.Username != "patron" {
		t.Errorf("session courante = %+v, erreur %v", u, err)
	}
}

func TestLogin_CompteDesactive(t *testing.T) {
	s := newService(t)
	u, _ := s.CreateFirstAdmin("patron", "La Patronne", "motdepasse1")
	if _, err := s.SetActive(u.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login("patron", "motdepasse1"); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("erreur = %v, attendu ErrAccountDisabled", err)
	}
}

func TestVerrouillageApresEchecsRepetes(t *testing.T) {
	s := newService(t)
	if _, err := s.CreateFirstAdmin("patron", "La Patronne", "motdepasse1"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxFailedAttempts; i++ {
		if _, err := s.Login("patron", "mauvais1234"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("tentative %d : %v", i+1, err)
		}
	}
	// Même le bon mot de passe est refusé pendant la fenêtre de blocage.
	if _, err := s.Login("patron", "motdepasse1"); !errors.Is(err, ErrLockedOut) {
		t.Errorf("erreur = %v, attendu ErrLockedOut", err)
	}
}

func TestSession_Expiration(t *testing.T) {
	s := newService(t)
	if _, err := s.CreateFirstAdmin("patron", "La Patronne", "motdepasse1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login("patron", "motdepasse1"); err != nil {
		t.Fatal(err)
	}
	s.SetTimeout(1)

	// On recule la dernière activité au-delà du délai.
	s.mu.Lock()
	s.lastSeen = time.Now().Add(-2 * time.Minute)
	s.mu.Unlock()

	if _, err := s.Current(); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("erreur = %v, attendu ErrNotAuthenticated", err)
	}
	if _, err := s.Require("sales"); !errors.Is(err, ErrNotAuthenticated) {
		t.Errorf("Require après expiration = %v", err)
	}
}

func TestPermissions(t *testing.T) {
	cases := []struct {
		role   models.Role
		scope  string
		permis bool
	}{
		{models.RoleAdmin, "users", true},
		{models.RoleAdmin, "settings", true},
		{models.RoleManager, "finance", true},
		{models.RoleManager, "users", false},
		{models.RoleManager, "settings", false},
		{models.RoleSeller, "sales", true},
		{models.RoleSeller, "finance", false},
		{models.RoleSeller, "stock", false},
		{models.RoleSeller, "delete", false},
	}
	for _, c := range cases {
		if got := Can(c.role, c.scope); got != c.permis {
			t.Errorf("Can(%s, %q) = %v, attendu %v", c.role, c.scope, got, c.permis)
		}
	}
	if len(ScopesFor(models.RoleSeller)) != 1 {
		t.Errorf("le vendeur devrait avoir un seul domaine : %v", ScopesFor(models.RoleSeller))
	}
}

func TestChangePassword(t *testing.T) {
	s := newService(t)
	if _, err := s.CreateFirstAdmin("patron", "La Patronne", "motdepasse1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login("patron", "motdepasse1"); err != nil {
		t.Fatal(err)
	}

	if err := s.ChangePassword("mauvais1234", "nouveaupass9"); err == nil {
		t.Error("un ancien mot de passe erroné aurait dû être refusé")
	}
	if err := s.ChangePassword("motdepasse1", "faible"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("erreur = %v, attendu ErrWeakPassword", err)
	}
	if err := s.ChangePassword("motdepasse1", "nouveaupass9"); err != nil {
		t.Fatalf("changement : %v", err)
	}

	s.Logout()
	if _, err := s.Login("patron", "motdepasse1"); err == nil {
		t.Error("l'ancien mot de passe fonctionne encore")
	}
	if _, err := s.Login("patron", "nouveaupass9"); err != nil {
		t.Errorf("le nouveau mot de passe est refusé : %v", err)
	}
}

func TestResetPassword_MotDePasseProvisoire(t *testing.T) {
	s := newService(t)
	admin, _ := s.CreateFirstAdmin("patron", "La Patronne", "motdepasse1")
	u, _ := s.CreateUser("vendeur", "Le Vendeur", "motdepasse1", models.RoleSeller)
	_ = admin

	provisoire, err := s.ResetPassword(u.ID, "")
	if err != nil {
		t.Fatalf("réinitialisation : %v", err)
	}
	if err := ValidatePassword(provisoire); err != nil {
		t.Errorf("le mot de passe provisoire ne respecte pas la politique : %v", err)
	}
	if _, err := s.Login("vendeur", provisoire); err != nil {
		t.Fatalf("connexion avec le mot de passe provisoire : %v", err)
	}
	// Tant qu'il n'est pas remplacé, il n'ouvre rien.
	if _, err := s.Require("sales"); !errors.Is(err, ErrMustChangePassword) {
		t.Errorf("erreur = %v, attendu ErrMustChangePassword", err)
	}
	if err := s.ChangePassword(provisoire, "definitif2026"); err != nil {
		t.Fatalf("changement : %v", err)
	}
	if _, err := s.Require("sales"); err != nil {
		t.Errorf("après changement, l'accès doit être rendu : %v", err)
	}
}
