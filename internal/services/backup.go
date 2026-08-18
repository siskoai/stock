package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/storage"
)

// timeNow est indirecté pour rester testable.
var timeNow = time.Now

// Backups expose les sauvegardes et la restauration. Réservé aux rôles qui
// portent le domaine « backup » (administrateur et gérant).
type Backups struct{ core }

// NewBackups construit le service de sauvegarde.
func NewBackups(db *storage.Database, a *auth.Service) *Backups {
	return &Backups{core{db: db, auth: a}}
}

// List renvoie les archives présentes, de la plus récente à la plus ancienne.
func (s *Backups) List() ([]storage.BackupInfo, error) {
	if _, err := s.guard("backup"); err != nil {
		return nil, err
	}
	return s.db.ListBackups()
}

// Create déclenche une sauvegarde manuelle et applique la rétention.
func (s *Backups) Create(label string) (storage.BackupInfo, error) {
	u, err := s.guard("backup")
	if err != nil {
		return storage.BackupInfo{}, err
	}
	info, err := s.db.Backup(defaultString(label, "manuel"))
	if err != nil {
		return storage.BackupInfo{}, err
	}
	if keep := s.db.Settings().BackupsToKeep; keep > 0 {
		if removed, err := s.db.PruneBackups(keep); err == nil && removed > 0 {
			s.log(u, "PRUNE", "backup", "", "%d sauvegarde(s) ancienne(s) supprimée(s), %d conservée(s)", removed, keep)
		}
	}
	s.log(u, "BACKUP", "backup", info.Name, "Sauvegarde « %s » créée (%d octets)", info.Name, info.SizeBytes)
	return info, nil
}

// Delete supprime une archive.
func (s *Backups) Delete(name string) error {
	u, err := s.guard("backup")
	if err != nil {
		return err
	}
	path, err := s.resolve(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("suppression de la sauvegarde : %w", err)
	}
	s.log(u, "DELETE", "backup", name, "Sauvegarde « %s » supprimée", name)
	return nil
}

// Restore remplace les données par le contenu d'une archive.
//
// L'application doit être redémarrée ensuite : les collections chargées en
// mémoire pointent encore sur les anciennes données. L'appelant (couche Wails)
// se charge de prévenir puis de relancer.
func (s *Backups) Restore(name string) error {
	u, err := s.guard("backup")
	if err != nil {
		return err
	}
	path, err := s.resolve(name)
	if err != nil {
		return err
	}
	if err := s.db.Restore(path); err != nil {
		return err
	}
	s.log(u, "RESTORE", "backup", name, "Données restaurées depuis « %s »", name)
	return nil
}

// RestoreFromPath restaure une archive choisie hors du dossier de sauvegardes
// (clé USB, dossier partagé). Le chemin vient d'un sélecteur de fichier natif.
func (s *Backups) RestoreFromPath(path string) error {
	u, err := s.guard("backup")
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return fmt.Errorf("une sauvegarde Comptoir est une archive .zip")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("fichier introuvable : %s", path)
	}
	if err := s.db.Restore(path); err != nil {
		return err
	}
	s.log(u, "RESTORE", "backup", filepath.Base(path), "Données restaurées depuis « %s »", path)
	return nil
}

// AutoBackup effectue la sauvegarde automatique quotidienne si elle est activée
// et qu'aucune n'a encore eu lieu aujourd'hui.
//
// C'est une fonction de paquet et non une méthode de service : elle s'exécute
// au démarrage, avant toute session, et n'a donc rien à faire dans l'API que
// l'interface peut appeler.
func AutoBackup(db *storage.Database) (bool, error) {
	settings := db.Settings()
	if !settings.AutoBackup {
		return false, nil
	}
	list, err := db.ListBackups()
	if err != nil {
		return false, err
	}
	today := timeNow().Format("2006-01-02")
	for _, b := range list {
		if strings.HasPrefix(b.Name, "comptoir_"+today) {
			return false, nil
		}
	}
	if _, err := db.Backup("auto"); err != nil {
		return false, err
	}
	if settings.BackupsToKeep > 0 {
		_, _ = db.PruneBackups(settings.BackupsToKeep)
	}
	return true, nil
}

// resolve valide qu'un nom d'archive désigne bien un fichier du dossier de
// sauvegardes : un nom venu de l'interface ne doit pas pouvoir désigner un
// fichier arbitraire du disque.
func (s *Backups) resolve(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("nom de sauvegarde invalide")
	}
	path := filepath.Join(s.db.BackupDir(), name)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("sauvegarde introuvable : %s", name)
	}
	return path, nil
}
