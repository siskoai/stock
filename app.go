package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"comptoir/internal/auth"
	"comptoir/internal/services"
	"comptoir/internal/storage"
)

// App porte le cycle de vie de l'application et les quelques opérations qui
// exigent le système hôte : sélecteurs de fichiers, ouverture de dossier,
// redémarrage. Toute la logique métier reste dans internal/services ; cette
// couche ne fait que relier Wails au reste.
type App struct {
	ctx context.Context
	db  *storage.Database
	sec *auth.Service
}

// NewApp construit la couche applicative.
func NewApp(db *storage.Database, sec *auth.Service) *App {
	return &App{db: db, sec: sec}
}

// startup est appelé par Wails une fois la fenêtre prête.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Sauvegarde automatique quotidienne : elle a lieu au premier démarrage du
	// jour, avant toute saisie, pour que l'archive reflète la veille complète.
	if done, err := services.AutoBackup(a.db); err != nil {
		runtime.LogErrorf(ctx, "sauvegarde automatique impossible : %v", err)
	} else if done {
		runtime.LogInfo(ctx, "sauvegarde automatique du jour effectuée")
	}
}

// shutdown est appelé à la fermeture de la fenêtre.
func (a *App) shutdown(ctx context.Context) {
	a.sec.Logout()
}

// ---------------------------------------------------------------------------
// Fichiers
// ---------------------------------------------------------------------------

// SaveFile propose l'enregistrement d'un document produit par l'application
// (PDF ou CSV) via le sélecteur natif. Renvoie le chemin retenu, ou une chaîne
// vide si l'utilisateur a annulé, une annulation n'est pas une erreur.
func (a *App) SaveFile(name string, content []byte) (string, error) {
	if _, err := a.sec.Require(""); err != nil {
		return "", err
	}
	if len(content) == 0 {
		return "", fmt.Errorf("document vide")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Enregistrer le document",
		DefaultFilename: filepath.Base(name),
		DefaultDirectory: func() string {
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, "Documents")
			}
			return a.db.Dir
		}(),
		Filters:                    filtersFor(name),
		CanCreateDirectories:       true,
		ShowHiddenFiles:            false,
		TreatPackagesAsDirectories: false,
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // annulé
	}
	if err := os.WriteFile(path, content, 0o640); err != nil {
		return "", fmt.Errorf("écriture du fichier : %w", err)
	}
	return path, nil
}

func filtersFor(name string) []runtime.FileFilter {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return []runtime.FileFilter{{DisplayName: "Document PDF (*.pdf)", Pattern: "*.pdf"}}
	case ".csv":
		return []runtime.FileFilter{{DisplayName: "Tableur (*.csv)", Pattern: "*.csv"}}
	case ".zip":
		return []runtime.FileFilter{{DisplayName: "Archive Comptoir (*.zip)", Pattern: "*.zip"}}
	default:
		return nil
	}
}

// PickBackupArchive ouvre un sélecteur de fichier pour choisir une archive à
// restaurer, par exemple sur une clé USB.
func (a *App) PickBackupArchive() (string, error) {
	if _, err := a.sec.Require("backup"); err != nil {
		return "", err
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choisir une sauvegarde Comptoir",
		DefaultDirectory: a.db.BackupDir(),
		Filters:          []runtime.FileFilter{{DisplayName: "Archive Comptoir (*.zip)", Pattern: "*.zip"}},
	})
}

// PickLogo ouvre un sélecteur d'image et renvoie le logo encodé en data URL,
// prêt à être enregistré dans les paramètres et embarqué dans les PDF.
func (a *App) PickLogo() (string, error) {
	if _, err := a.sec.Require("settings"); err != nil {
		return "", err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choisir le logo de l'entreprise",
		Filters: []runtime.FileFilter{{DisplayName: "Image (*.png, *.jpg)", Pattern: "*.png;*.jpg;*.jpeg"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("lecture de l'image : %w", err)
	}
	if len(raw) > 380*1024 {
		return "", fmt.Errorf("cette image pèse %d Ko : choisissez-en une de moins de 380 Ko", len(raw)/1024)
	}
	mime := "image/png"
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".jpg" || ext == ".jpeg" {
		mime = "image/jpeg"
	}
	return "data:" + mime + ";base64," + encodeBase64(raw), nil
}

// encodeBase64 encode des octets pour une data URL.
func encodeBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// PickCatalogFile ouvre un sélecteur de fichier et renvoie le contenu d'un
// tableur exporté en CSV, prêt à être analysé par l'import de catalogue.
//
// Le fichier est lu ici plutôt que côté interface : le frontend n'a pas accès
// au disque, et un chemin qui transiterait par lui serait une adresse à laquelle
// il pourrait faire lire n'importe quoi.
func (a *App) PickCatalogFile() (string, error) {
	if _, err := a.sec.Require("catalog"); err != nil {
		return "", err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choisir le fichier du catalogue",
		Filters: []runtime.FileFilter{
			{DisplayName: "Tableur exporté (*.csv, *.txt)", Pattern: "*.csv;*.txt"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("lecture du fichier : %w", err)
	}
	// Un catalogue de plusieurs milliers d'articles reste très en dessous ;
	// au-delà, c'est que le fichier choisi n'en est pas un.
	if len(raw) > 8<<20 {
		return "", fmt.Errorf("ce fichier pèse %d Mo : ce n'est probablement pas un catalogue", len(raw)/(1<<20))
	}
	if !utf8.Valid(raw) {
		// Excel sous Windows enregistre encore en Windows-1252 : la conversion
		// évite un fichier truffé de caractères de remplacement.
		return decodeWindows1252(raw), nil
	}
	return string(raw), nil
}

// decodeWindows1252 convertit un texte hérité vers UTF-8. Les octets 0x80-0x9F
// portent en Windows-1252 des caractères que Latin-1 laisse vides, dont les
// guillemets et l'apostrophe typographiques, courants dans les désignations.
func decodeWindows1252(raw []byte) string {
	var high = [32]rune{
		'\u20AC', 0, '\u201A', '\u0192', '\u201E', '\u2026', '\u2020', '\u2021',
		'\u02C6', '\u2030', '\u0160', '\u2039', '\u0152', 0, '\u017D', 0,
		0, '\u2018', '\u2019', '\u201C', '\u201D', '\u2022', '\u2013', '\u2014',
		'\u02DC', '\u2122', '\u0161', '\u203A', '\u0153', 0, '\u017E', '\u0178',
	}
	var sb strings.Builder
	sb.Grow(len(raw))
	for _, b := range raw {
		switch {
		case b < 0x80:
			sb.WriteByte(b)
		case b < 0xA0:
			if r := high[b-0x80]; r != 0 {
				sb.WriteRune(r)
			} else {
				sb.WriteRune('\uFFFD')
			}
		default:
			sb.WriteRune(rune(b))
		}
	}
	return sb.String()
}

// OpenDataFolder ouvre le dossier de données dans l'explorateur du système.
func (a *App) OpenDataFolder(which string) error {
	if _, err := a.sec.Require(""); err != nil {
		return err
	}
	target := a.db.Dir
	switch which {
	case "backups":
		target = a.db.BackupDir()
	case "exports":
		target = filepath.Join(a.db.Dir, "exports")
	}
	// Le dossier peut ne pas exister si rien n'y a encore été déposé.
	if err := os.MkdirAll(target, 0o750); err != nil {
		return fmt.Errorf("ouverture du dossier : %w", err)
	}
	return revelerDansExplorateur(target)
}

// ---------------------------------------------------------------------------
// Fenêtre
// ---------------------------------------------------------------------------

// Quit ferme l'application. Appelée après une restauration : les collections en
// mémoire ne correspondent plus aux fichiers, seul un redémarrage les réaligne.
func (a *App) Quit() {
	runtime.Quit(a.ctx)
}
