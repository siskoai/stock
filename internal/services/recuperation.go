package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Récupération d'un accès administrateur perdu.
//
// Un mot de passe n'est jamais conservé en clair : s'il est oublié et qu'aucun
// autre administrateur n'existe, plus personne n'ouvre la boutique. Un
// commerçant se retrouverait alors avec ses données sur son disque, lisibles,
// et un logiciel qui refuse de démarrer. C'est inacceptable.
//
// La reprise passe par un fichier déposé dans le dossier de données. Ce choix
// n'affaiblit pas le modèle de menace : quiconque peut créer ce fichier peut
// déjà lire l'intégralité des ventes, des clients et des marges, puisque les
// données sont du JSON en clair. La protection contre un accès au disque relève
// du chiffrement du système, pas de l'application. Voir docs/SECURITE.md.
//
// Ce que la procédure garantit en revanche : elle est tracée dans le journal
// d'audit, elle ne rend l'accès qu'une seule fois, et le mot de passe obtenu ne
// permet rien d'autre que d'en choisir un nouveau.

const (
	// FichierReprise est déposé par l'utilisateur pour demander la reprise.
	FichierReprise = "REINITIALISER-MOT-DE-PASSE.txt"

	// FichierMotDePasse reçoit le mot de passe provisoire produit.
	FichierMotDePasse = "MOT-DE-PASSE-PROVISOIRE.txt"
)

// Reprise décrit ce qui s'est produit au démarrage.
type Reprise struct {
	Effectuee bool   `json:"effectuee"`
	Compte    string `json:"compte"`
	Fichier   string `json:"fichier"`
}

// RepriseAdministrateur applique la procédure si le fichier de demande est
// présent. Appelée une fois au démarrage, avant toute session.
//
// Le fichier peut nommer le compte à reprendre. Vide, c'est le premier
// administrateur actif qui est retenu.
func RepriseAdministrateur(db *storage.Database, sec *auth.Service) (Reprise, error) {
	demande := filepath.Join(db.Dir, FichierReprise)
	contenu, err := os.ReadFile(demande)
	if os.IsNotExist(err) {
		return Reprise{}, nil
	}
	if err != nil {
		return Reprise{}, fmt.Errorf("lecture de la demande de reprise : %w", err)
	}
	// Le fichier est retiré quoi qu'il arrive ensuite : une demande ne doit pas
	// pouvoir rejouer à chaque démarrage.
	defer func() { _ = os.Remove(demande) }()

	souhaite := strings.ToLower(strings.TrimSpace(string(contenu)))
	cible, err := choisirCompte(sec, souhaite)
	if err != nil {
		return Reprise{}, err
	}

	provisoire, err := sec.ResetPassword(cible.ID, "")
	if err != nil {
		return Reprise{}, err
	}

	chemin := filepath.Join(db.Dir, FichierMotDePasse)
	message := fmt.Sprintf(`Comptoir, reprise d'accès du %s

Compte             : %s
Mot de passe       : %s

Ce mot de passe ne sert qu'une fois : il ouvre la session, et rien d'autre.
Comptoir vous demandera d'en choisir un nouveau immédiatement.

Supprimez ce fichier dès que vous avez retenu le mot de passe. Toute personne
qui le lit peut ouvrir votre boutique.

Cette reprise est inscrite dans le journal d'audit, à la date ci-dessus.
`, time.Now().Format("02/01/2006 à 15h04"), cible.Username, provisoire)

	if err := os.WriteFile(chemin, []byte(message), 0o600); err != nil {
		return Reprise{}, fmt.Errorf("écriture du mot de passe provisoire : %w", err)
	}

	_ = db.Audit.Insert(models.AuditEntry{
		ID: util.NewID("aud"), At: time.Now(),
		UserID: cible.ID, UserName: cible.FullName,
		Action: "REPRISE", Entity: "user", EntityID: cible.ID,
		Details: fmt.Sprintf(
			"Reprise d'accès au démarrage : mot de passe du compte « %s » réinitialisé depuis le fichier %s",
			cible.Username, FichierReprise),
	})

	return Reprise{Effectuee: true, Compte: cible.Username, Fichier: chemin}, nil
}

// choisirCompte retient le compte nommé, ou à défaut le premier administrateur
// actif. La reprise ne vise que des administrateurs : pour un vendeur, il suffit
// de demander à son responsable.
func choisirCompte(sec *auth.Service, souhaite string) (models.User, error) {
	comptes := sec.List()
	if souhaite != "" {
		for _, u := range comptes {
			if strings.EqualFold(u.Username, souhaite) {
				if u.Role != models.RoleAdmin {
					return models.User{}, fmt.Errorf(
						"le compte « %s » n'est pas administrateur : la reprise ne s'applique qu'aux administrateurs", souhaite)
				}
				return u, nil
			}
		}
		return models.User{}, fmt.Errorf("aucun compte nommé « %s » sur ce poste", souhaite)
	}
	for _, u := range comptes {
		if u.Role == models.RoleAdmin && u.Active {
			return u, nil
		}
	}
	// Un administrateur désactivé vaut mieux que rien : c'est précisément le
	// cas où plus personne ne peut ouvrir la boutique.
	for _, u := range comptes {
		if u.Role == models.RoleAdmin {
			return u, nil
		}
	}
	return models.User{}, fmt.Errorf("aucun compte administrateur sur ce poste")
}

// InstructionsReprise décrit la marche à suivre, pour que l'écran de connexion
// puisse l'afficher sans que l'utilisateur ait à chercher dans un manuel.
type InstructionsReprise struct {
	Dossier  string `json:"dossier"`
	Fichier  string `json:"fichier"`
	Resultat string `json:"resultat"`
}

// Instructions renvoie le chemin exact où déposer le fichier de reprise.
// Accessible sans session : c'est justement quand on ne peut plus se connecter
// qu'on en a besoin.
func (s *Session) Instructions() InstructionsReprise {
	return InstructionsReprise{
		Dossier:  s.db.Dir,
		Fichier:  FichierReprise,
		Resultat: FichierMotDePasse,
	}
}
