package storage

import (
	"fmt"

	"comptoir/internal/models"
)

// migration transforme les données d'une version de schéma vers la suivante.
// From est la version d'origine ; après application, les données sont en
// version From+1.
type migration struct {
	From  int
	Label string
	Apply func(db *Database) error
}

// migrations est la liste ordonnée des transformations connues. Elle est vide
// tant que le format n'a pas changé : la mécanique existe pour que la première
// évolution du schéma n'oblige pas à improviser sur des données de production.
var migrations = []migration{}

// Migrate met les données au niveau de models.SchemaVersion.
//
// Une sauvegarde complète est prise avant la première transformation : une
// migration ratée doit toujours pouvoir être défaite en restaurant l'archive.
// Les données plus récentes que le programme sont refusées à l'ouverture
// (voir writeMeta) : rétrograder silencieusement perdrait des champs.
func Migrate(db *Database) error {
	version, err := db.schemaVersion()
	if err != nil {
		return err
	}
	if version >= models.SchemaVersion {
		return db.repair()
	}

	if _, err := db.Backup(fmt.Sprintf("avant-migration-v%d", version)); err != nil {
		return fmt.Errorf("sauvegarde préalable impossible, migration annulée : %w", err)
	}

	for version < models.SchemaVersion {
		step, ok := findMigration(version)
		if !ok {
			return fmt.Errorf(
				"aucune migration connue du schéma %d vers %d : ouvrez ces données avec la version de Comptoir qui les a créées",
				version, version+1)
		}
		if err := step.Apply(db); err != nil {
			return fmt.Errorf("migration « %s » : %w", step.Label, err)
		}
		version++
		if err := db.setSchemaVersion(version); err != nil {
			return err
		}
	}
	return db.repair()
}

func findMigration(from int) (migration, bool) {
	for _, m := range migrations {
		if m.From == from {
			return m, true
		}
	}
	return migration{}, false
}

// repair réaligne les compteurs sur les données réellement présentes. Sans
// effet dans le cas normal ; utile après une restauration d'archive partielle
// ou une reprise de fichiers modifiés à la main, où un compteur trop bas
// redistribuerait des numéros déjà utilisés.
func (db *Database) repair() error {
	s := db.settings
	changed := false

	if n := db.Movements.Count(); s.MovementCounter < n {
		s.MovementCounter = n
		changed = true
	}
	if n := db.Invoices.Count(); s.InvoiceCounter < n {
		s.InvoiceCounter = n
		changed = true
	}
	if n := db.Purchases.Count(); s.PurchaseCounter < n {
		s.PurchaseCounter = n
		changed = true
	}
	if s.Decimals < 0 || s.Decimals > 2 {
		s.Decimals = 0
		changed = true
	}
	if s.BackupsToKeep <= 0 {
		s.BackupsToKeep = 30
		changed = true
	}
	if s.SessionTimeoutMin <= 0 {
		s.SessionTimeoutMin = 60
		changed = true
	}
	if s.FiscalYearStartMonth < 1 || s.FiscalYearStartMonth > 12 {
		s.FiscalYearStartMonth = 1
		changed = true
	}
	if !changed {
		return nil
	}
	return db.SaveSettings(s)
}
