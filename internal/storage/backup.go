package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupInfo décrit une archive de sauvegarde présente sur le disque.
type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}

// BackupDir renvoie le répertoire des sauvegardes.
func (db *Database) BackupDir() string { return filepath.Join(db.Dir, "backups") }

// Backup crée une archive ZIP horodatée de l'intégralité du répertoire data/.
// L'étiquette permet de distinguer les sauvegardes automatiques ("auto") des
// sauvegardes manuelles ("manuel") ou d'avant-restauration ("avant-restauration").
func (db *Database) Backup(label string) (BackupInfo, error) {
	if label == "" {
		label = "manuel"
	}
	stamp := time.Now().Format("2006-01-02_15h04m05")
	name := fmt.Sprintf("comptoir_%s_%s.zip", stamp, sanitize(label))
	dest := filepath.Join(db.BackupDir(), name)

	if err := zipDir(filepath.Join(db.Dir, "data"), dest); err != nil {
		return BackupInfo{}, fmt.Errorf("création de la sauvegarde : %w", err)
	}
	st, err := os.Stat(dest)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Name: name, Path: dest, SizeBytes: st.Size(), CreatedAt: st.ModTime()}, nil
}

// ListBackups renvoie les sauvegardes existantes, de la plus récente à la plus
// ancienne.
func (db *Database) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(db.BackupDir())
	if err != nil {
		return nil, err
	}
	out := make([]BackupInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{
			Name:      e.Name(),
			Path:      filepath.Join(db.BackupDir(), e.Name()),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// PruneBackups conserve les `keep` sauvegardes les plus récentes et supprime
// les autres. Renvoie le nombre d'archives supprimées.
func (db *Database) PruneBackups(keep int) (int, error) {
	if keep <= 0 {
		return 0, nil
	}
	list, err := db.ListBackups()
	if err != nil {
		return 0, err
	}
	removed := 0
	for i := keep; i < len(list); i++ {
		if err := os.Remove(list[i].Path); err == nil {
			removed++
		}
	}
	return removed, nil
}

// Restore remplace le contenu de data/ par celui d'une archive.
// Une sauvegarde de sécurité est créée avant toute écriture. L'appelant doit
// redémarrer l'application ensuite : les collections en mémoire pointent encore
// sur les anciennes données.
func (db *Database) Restore(archivePath string) error {
	if _, err := db.Backup("avant-restauration"); err != nil {
		return fmt.Errorf("sauvegarde de sécurité impossible, restauration annulée : %w", err)
	}
	dataDir := filepath.Join(db.Dir, "data")
	staging := filepath.Join(db.Dir, ".restore-tmp")
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o750); err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := unzipTo(archivePath, staging); err != nil {
		return fmt.Errorf("archive illisible : %w", err)
	}
	// L'archive doit contenir au minimum le fichier des produits : garde-fou
	// contre la restauration d'un ZIP quelconque.
	if _, err := os.Stat(filepath.Join(staging, "products.json")); err != nil {
		return fmt.Errorf("cette archive n'est pas une sauvegarde Comptoir valide")
	}

	old := dataDir + ".old"
	_ = os.RemoveAll(old)
	if err := os.Rename(dataDir, old); err != nil {
		return err
	}
	if err := os.Rename(staging, dataDir); err != nil {
		_ = os.Rename(old, dataDir) // retour à l'état initial
		return err
	}
	_ = os.RemoveAll(old)
	return nil
}

// ---------------------------------------------------------------------------

func sanitize(s string) string {
	repl := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", "*", "", "?", "", "\"", "")
	return repl.Replace(s)
}

// safeArchivePath rejette les chemins absolus, les remontées « .. » et les
// noms d'unité Windows. Le format ZIP impose « / » comme séparateur : tout
// antislash dans un nom d'entrée est déjà le signe d'une archive malformée.
func safeArchivePath(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	if len(name) > 1 && name[1] == ':' {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func zipDir(src, dest string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
}

func unzipTo(archive, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Protection « Zip Slip ». Une entrée qui remonte hors du répertoire de
		// destination est refusée, et non pas ramenée de force à l'intérieur :
		// une archive dont le contenu ne correspond pas à ce qu'il annonce n'a
		// pas à être restaurée en silence.
		if !safeArchivePath(f.Name) {
			return fmt.Errorf("entrée d'archive suspecte : %s", f.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("entrée d'archive suspecte : %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
		if err != nil {
			rc.Close()
			return err
		}
		// Limite de taille : protège contre une archive « zip bomb ».
		if _, err := io.Copy(out, io.LimitReader(rc, 256<<20)); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}
