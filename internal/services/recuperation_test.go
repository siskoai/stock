package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"comptoir/internal/auth"
	"comptoir/internal/models"
)

// demander dépose le fichier de reprise, éventuellement nominatif.
func demander(t *testing.T, dir, compte string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FichierReprise), []byte(compte), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRepriseAdministrateur(t *testing.T) {
	s := newSuite(t)
	s.sec.Logout()
	demander(t, s.db.Dir, "")

	reprise, err := RepriseAdministrateur(s.db, s.sec)
	if err != nil {
		t.Fatalf("reprise : %v", err)
	}
	if !reprise.Effectuee || reprise.Compte != "patron" {
		t.Fatalf("reprise = %+v, attendu le compte « patron »", reprise)
	}

	// Le fichier de demande doit disparaître : une reprise ne se rejoue pas.
	if _, err := os.Stat(filepath.Join(s.db.Dir, FichierReprise)); !os.IsNotExist(err) {
		t.Error("la demande de reprise est encore là")
	}

	// Le mot de passe provisoire doit être lisible et fonctionner.
	brut, err := os.ReadFile(reprise.Fichier)
	if err != nil {
		t.Fatalf("lecture du mot de passe provisoire : %v", err)
	}
	var motDePasse string
	for _, ligne := range strings.Split(string(brut), "\n") {
		if strings.HasPrefix(ligne, "Mot de passe") {
			motDePasse = strings.TrimSpace(strings.SplitN(ligne, ":", 2)[1])
		}
	}
	if motDePasse == "" {
		t.Fatalf("aucun mot de passe dans le fichier produit :\n%s", brut)
	}
	if _, err := s.sec.Login("patron", motDePasse); err != nil {
		t.Fatalf("connexion avec le mot de passe repris : %v", err)
	}

	// Il n'ouvre rien tant qu'il n'est pas remplacé.
	if _, err := s.sec.Require("sales"); err == nil {
		t.Error("le mot de passe provisoire donne accès aux ventes")
	}
	if err := s.sec.ChangePassword(motDePasse, "definitif2026"); err != nil {
		t.Fatalf("changement : %v", err)
	}
	if _, err := s.sec.Require("sales"); err != nil {
		t.Errorf("après changement, l'accès doit être rendu : %v", err)
	}

	// La reprise laisse une trace.
	trouve := false
	for _, e := range s.db.Audit.All() {
		if e.Action == "REPRISE" {
			trouve = true
		}
	}
	if !trouve {
		t.Error("la reprise n'est pas inscrite au journal d'audit")
	}
}

func TestRepriseAdministrateur_CompteNomme(t *testing.T) {
	s := newSuite(t)
	if _, err := s.sec.CreateUser("adjoint", "L'Adjoint", "motdepasse1", models.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	s.sec.Logout()
	demander(t, s.db.Dir, "  ADJOINT \n")

	reprise, err := RepriseAdministrateur(s.db, s.sec)
	if err != nil {
		t.Fatalf("reprise : %v", err)
	}
	if reprise.Compte != "adjoint" {
		t.Errorf("compte repris = %q, attendu « adjoint »", reprise.Compte)
	}
}

func TestReprise_RefusHorsAdministrateur(t *testing.T) {
	s := newSuite(t)
	if _, err := s.sec.CreateUser("vendeur", "Le Vendeur", "motdepasse1", models.RoleSeller); err != nil {
		t.Fatal(err)
	}
	s.sec.Logout()
	demander(t, s.db.Dir, "vendeur")

	if _, err := RepriseAdministrateur(s.db, s.sec); err == nil {
		t.Fatal("la reprise d'un compte vendeur aurait dû être refusée")
	}
	// Même refusée, la demande est consommée : elle ne doit pas rejouer.
	if _, err := os.Stat(filepath.Join(s.db.Dir, FichierReprise)); !os.IsNotExist(err) {
		t.Error("la demande refusée est encore là")
	}
}

func TestReprise_SansDemande(t *testing.T) {
	s := newSuite(t)
	reprise, err := RepriseAdministrateur(s.db, s.sec)
	if err != nil {
		t.Fatalf("sans demande, la reprise ne doit rien faire : %v", err)
	}
	if reprise.Effectuee {
		t.Error("une reprise a eu lieu sans demande")
	}
	// La session en cours ne doit pas avoir été perturbée.
	if _, err := s.sec.Require("sales"); err != nil {
		t.Errorf("la session a été cassée : %v", err)
	}
}

func TestInstructionsReprise(t *testing.T) {
	s := newSuite(t)
	session := NewSession(s.db, s.sec)
	session.Logout()

	// Accessibles sans session : c'est le seul moment où elles servent.
	in := session.Instructions()
	if in.Dossier != s.db.Dir {
		t.Errorf("dossier = %q, attendu %q", in.Dossier, s.db.Dir)
	}
	if in.Fichier != FichierReprise || in.Resultat != FichierMotDePasse {
		t.Errorf("noms de fichiers inattendus : %+v", in)
	}
	_ = auth.ErrNotAuthenticated
}
