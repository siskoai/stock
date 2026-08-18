package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"comptoir/internal/models"
)

// Database regroupe toutes les collections de l'application ainsi que les
// paramètres. C'est le point d'entrée unique de la couche de données : aucun
// service n'ouvre de fichier directement.
type Database struct {
	Dir string

	Users      *Collection[models.User]
	Categories *Collection[models.Category]
	Products   *Collection[models.Product]
	Parties    *Collection[models.Party]
	Movements  *Collection[models.Movement]
	Invoices   *Collection[models.Invoice]
	Purchases  *Collection[models.Purchase]
	Expenses   *Collection[models.Expense]
	Audit      *Collection[models.AuditEntry]

	settingsPath string
	settings     models.Settings
}

type metaFile struct {
	SchemaVersion int       `json:"schemaVersion"`
	App           string    `json:"app"`
	CreatedAt     time.Time `json:"createdAt"`
	LastOpenedAt  time.Time `json:"lastOpenedAt"`
}

// DataDir renvoie le répertoire de données de l'application.
// Windows : %APPDATA%\Comptoir, macOS : ~/Library/Application Support/Comptoir
// Un fichier portable.txt placé à côté de l'exécutable bascule en mode
// portable : les données vivent alors dans .\data (clé USB, dossier partagé).
func DataDir() (string, error) {
	if exe, err := os.Executable(); err == nil {
		root := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(root, "portable.txt")); err == nil {
			return filepath.Join(root, "data"), nil
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("répertoire de configuration introuvable : %w", err)
	}
	return filepath.Join(base, "Comptoir"), nil
}

// Open ouvre (ou initialise) la base dans le répertoire indiqué.
func Open(dir string) (*Database, error) {
	dataDir := filepath.Join(dir, "data")
	// 0750 : accessible à l'utilisateur courant et à son groupe uniquement.
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("création du répertoire de données : %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "backups"), 0o750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "exports"), 0o750); err != nil {
		return nil, err
	}

	db := &Database{Dir: dir, settingsPath: filepath.Join(dataDir, "settings.json")}

	var err error
	if db.Users, err = OpenCollection[models.User](dataDir, "users"); err != nil {
		return nil, err
	}
	if db.Categories, err = OpenCollection[models.Category](dataDir, "categories"); err != nil {
		return nil, err
	}
	if db.Products, err = OpenCollection[models.Product](dataDir, "products"); err != nil {
		return nil, err
	}
	if db.Parties, err = OpenCollection[models.Party](dataDir, "parties"); err != nil {
		return nil, err
	}
	if db.Movements, err = OpenCollection[models.Movement](dataDir, "movements"); err != nil {
		return nil, err
	}
	if db.Invoices, err = OpenCollection[models.Invoice](dataDir, "invoices"); err != nil {
		return nil, err
	}
	if db.Purchases, err = OpenCollection[models.Purchase](dataDir, "purchases"); err != nil {
		return nil, err
	}
	if db.Expenses, err = OpenCollection[models.Expense](dataDir, "expenses"); err != nil {
		return nil, err
	}
	if db.Audit, err = OpenCollection[models.AuditEntry](dataDir, "audit"); err != nil {
		return nil, err
	}

	if err := db.loadSettings(); err != nil {
		return nil, err
	}
	if err := db.writeMeta(dataDir); err != nil {
		return nil, err
	}
	return db, nil
}

// schemaVersion renvoie la version de schéma des données présentes.
func (db *Database) schemaVersion() (int, error) {
	var meta metaFile
	if err := ReadJSON(filepath.Join(db.Dir, "data", "meta.json"), &meta); err != nil {
		return models.SchemaVersion, nil // base neuve : elle est au format courant
	}
	if meta.SchemaVersion <= 0 {
		return 1, nil
	}
	return meta.SchemaVersion, nil
}

// setSchemaVersion inscrit la version atteinte après une étape de migration.
func (db *Database) setSchemaVersion(version int) error {
	path := filepath.Join(db.Dir, "data", "meta.json")
	var meta metaFile
	_ = ReadJSON(path, &meta)
	meta.SchemaVersion = version
	meta.App = "Comptoir"
	meta.LastOpenedAt = time.Now()
	return WriteJSONAtomic(path, meta)
}

func (db *Database) writeMeta(dataDir string) error {
	path := filepath.Join(dataDir, "meta.json")
	meta := metaFile{SchemaVersion: models.SchemaVersion, App: "Comptoir", CreatedAt: time.Now()}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &meta)
		if meta.SchemaVersion > models.SchemaVersion {
			return fmt.Errorf(
				"ces données ont été créées par une version plus récente de Comptoir (schéma %d, cette version gère %d)",
				meta.SchemaVersion, models.SchemaVersion)
		}
	}
	meta.SchemaVersion = models.SchemaVersion
	meta.LastOpenedAt = time.Now()
	return WriteJSONAtomic(path, meta)
}

// ---------------------------------------------------------------------------
// Paramètres
// ---------------------------------------------------------------------------

func (db *Database) loadSettings() error {
	err := ReadJSON(db.settingsPath, &db.settings)
	if errors.Is(err, os.ErrNotExist) {
		db.settings = models.DefaultSettings()
		return WriteJSONAtomic(db.settingsPath, db.settings)
	}
	if err != nil {
		return fmt.Errorf("lecture des paramètres : %w", err)
	}
	if db.settings.ID == "" {
		db.settings.ID = "settings"
	}
	return nil
}

// Settings renvoie une copie des paramètres courants.
func (db *Database) Settings() models.Settings { return db.settings }

// SaveSettings écrit les paramètres et met à jour le cache.
func (db *Database) SaveSettings(s models.Settings) error {
	s.ID = "settings"
	s.UpdatedAt = time.Now()
	if err := WriteJSONAtomic(db.settingsPath, s); err != nil {
		return err
	}
	db.settings = s
	return nil
}

// NextInvoiceNumber réserve et renvoie le prochain numéro de facture.
// Le compteur est incrémenté et persisté avant d'être rendu, afin qu'un numéro
// ne puisse jamais être attribué deux fois. Si le document n'est finalement pas
// écrit, l'appelant rend le numéro avec ReleaseInvoiceNumber : la numérotation
// ne doit comporter ni doublon ni trou.
func (db *Database) NextInvoiceNumber() (string, error) {
	s := db.settings
	s.InvoiceCounter++
	if err := db.SaveSettings(s); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%04d", s.InvoicePrefix, time.Now().Year(), s.InvoiceCounter), nil
}

// ReleaseInvoiceNumber rend le dernier numéro réservé si le document n'a pas
// pu être créé. Sans effet si un autre numéro a été attribué entre-temps.
func (db *Database) ReleaseInvoiceNumber(number string) {
	s := db.settings
	if s.InvoiceCounter <= 0 || number != fmt.Sprintf("%s-%d-%04d", s.InvoicePrefix, time.Now().Year(), s.InvoiceCounter) {
		return
	}
	s.InvoiceCounter--
	_ = db.SaveSettings(s)
}

// NextPurchaseNumber réserve et renvoie le prochain numéro de bon d'entrée.
func (db *Database) NextPurchaseNumber() (string, error) {
	s := db.settings
	s.PurchaseCounter++
	if err := db.SaveSettings(s); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%04d", s.PurchasePrefix, time.Now().Year(), s.PurchaseCounter), nil
}

// ReleasePurchaseNumber rend le dernier numéro de bon d'entrée réservé.
func (db *Database) ReleasePurchaseNumber(number string) {
	s := db.settings
	if s.PurchaseCounter <= 0 || number != fmt.Sprintf("%s-%d-%04d", s.PurchasePrefix, time.Now().Year(), s.PurchaseCounter) {
		return
	}
	s.PurchaseCounter--
	_ = db.SaveSettings(s)
}

// NextMovementRefs réserve n références de mouvement consécutives, au format
// documenté MVT-2026-000123. Un compteur persisté, plutôt qu'un horodatage à la
// seconde, garantit l'unicité même quand un document sort dix lignes d'un coup.
func (db *Database) NextMovementRefs(n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	s := db.settings
	first := s.MovementCounter + 1
	s.MovementCounter += n
	if err := db.SaveSettings(s); err != nil {
		return nil, err
	}
	year := time.Now().Year()
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("MVT-%d-%06d", year, first+i)
	}
	return out, nil
}
