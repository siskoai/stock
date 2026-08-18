package services

import (
	"fmt"
	"strings"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
)

// Config expose les paramètres de l'application : identité de l'entreprise,
// monnaie, taxes, numérotation, sauvegardes. Lecture ouverte à tout compte
// connecté — l'interface a besoin du symbole monétaire et du nom de la boutique
// pour s'afficher —, écriture réservée au rôle Administrateur.
type Config struct{ core }

// NewConfig construit le service des paramètres.
func NewConfig(db *storage.Database, a *auth.Service) *Config {
	return &Config{core{db: db, auth: a}}
}

// Get renvoie les paramètres courants.
func (s *Config) Get() (models.Settings, error) {
	if _, err := s.guard(""); err != nil {
		return models.Settings{}, err
	}
	return s.db.Settings(), nil
}

// CurrencyPreset décrit une monnaie proposée à la configuration.
type CurrencyPreset struct {
	Code     string `json:"code"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
	Label    string `json:"label"`
}

// Presets renvoie les valeurs proposées dans l'écran de paramétrage :
// monnaies, rubriques de charges et modes de règlement.
func (s *Config) Presets() (map[string]any, error) {
	if _, err := s.guard(""); err != nil {
		return nil, err
	}
	return map[string]any{
		"currencies": []CurrencyPreset{
			{"XOF", "FCFA", 0, "Franc CFA (UEMOA)"},
			{"XAF", "FCFA", 0, "Franc CFA (CEMAC)"},
			{"GNF", "GNF", 0, "Franc guinéen"},
			{"MRU", "MRU", 2, "Ouguiya mauritanien"},
			{"MAD", "DH", 2, "Dirham marocain"},
			{"EUR", "€", 2, "Euro"},
			{"USD", "$", 2, "Dollar américain"},
		},
		"expenseCategories": models.ExpenseCategories,
		"paymentMethods": []map[string]string{
			{"value": string(models.PayCash), "label": paymentLabel(models.PayCash)},
			{"value": string(models.PayMobile), "label": paymentLabel(models.PayMobile)},
			{"value": string(models.PayTransfer), "label": paymentLabel(models.PayTransfer)},
			{"value": string(models.PayCheck), "label": paymentLabel(models.PayCheck)},
			{"value": string(models.PayCredit), "label": paymentLabel(models.PayCredit)},
		},
	}, nil
}

// Save enregistre les paramètres après validation.
//
// Les compteurs de numérotation ne sont jamais repris de l'interface : les
// abaisser redistribuerait des numéros déjà attribués. Ils gardent leur valeur
// courante, sauf remise à zéro explicite en début d'exercice (ResetCounters).
func (s *Config) Save(in models.Settings) (models.Settings, error) {
	u, err := s.guard("settings")
	if err != nil {
		return models.Settings{}, err
	}
	current := s.db.Settings()

	name, err := requireText(in.CompanyName, "Nom de l'entreprise")
	if err != nil {
		return models.Settings{}, err
	}
	if in.DefaultTaxRate < 0 || in.DefaultTaxRate > 100 {
		return models.Settings{}, fmt.Errorf("le taux de taxe par défaut doit être compris entre 0 et 100 %%")
	}
	if in.Decimals < 0 || in.Decimals > 2 {
		return models.Settings{}, fmt.Errorf("le nombre de décimales doit valoir 0, 1 ou 2")
	}
	if in.FiscalYearStartMonth < 1 || in.FiscalYearStartMonth > 12 {
		return models.Settings{}, fmt.Errorf("le mois de début d'exercice doit être compris entre 1 et 12")
	}
	if in.SessionTimeoutMin < 5 || in.SessionTimeoutMin > 720 {
		return models.Settings{}, fmt.Errorf("le délai d'inactivité doit être compris entre 5 et 720 minutes")
	}
	if in.BackupsToKeep < 1 || in.BackupsToKeep > 500 {
		return models.Settings{}, fmt.Errorf("le nombre de sauvegardes à conserver doit être compris entre 1 et 500")
	}
	prefix, err := requireText(in.InvoicePrefix, "Préfixe des factures")
	if err != nil {
		return models.Settings{}, err
	}
	purchasePrefix, err := requireText(in.PurchasePrefix, "Préfixe des bons d'entrée")
	if err != nil {
		return models.Settings{}, err
	}
	if in.LogoDataURL != "" && !strings.HasPrefix(in.LogoDataURL, "data:image/") {
		return models.Settings{}, fmt.Errorf("le logo doit être une image")
	}
	// Le logo est embarqué dans settings.json et dans chaque PDF : au-delà de
	// quelques centaines de kilo-octets, il alourdit tout sans rien apporter.
	if len(in.LogoDataURL) > 512*1024 {
		return models.Settings{}, fmt.Errorf("le logo est trop lourd (max. 380 Ko environ) : réduisez sa taille")
	}

	next := in
	next.ID = "settings"
	next.CompanyName = name
	next.InvoicePrefix = strings.ToUpper(prefix)
	next.PurchasePrefix = strings.ToUpper(purchasePrefix)
	next.InvoiceCounter = current.InvoiceCounter
	next.PurchaseCounter = current.PurchaseCounter
	next.MovementCounter = current.MovementCounter
	next.Currency = defaultString(in.Currency, current.Currency)
	next.CurrencySymbol = defaultString(in.CurrencySymbol, next.Currency)
	next.Theme = defaultString(in.Theme, "light")

	if err := s.db.SaveSettings(next); err != nil {
		return models.Settings{}, err
	}
	s.auth.SetTimeout(next.SessionTimeoutMin)
	s.log(u, "UPDATE", "settings", "settings", "Paramètres modifiés — %s, %s, TVA %.2f %%",
		next.CompanyName, next.CurrencySymbol, next.DefaultTaxRate)
	return next, nil
}

// ResetCounters remet la numérotation à zéro, typiquement au changement
// d'exercice. Refusé s'il existe déjà des documents de l'année en cours : deux
// factures ne peuvent pas porter le même numéro.
func (s *Config) ResetCounters() (models.Settings, error) {
	u, err := s.guard("settings")
	if err != nil {
		return models.Settings{}, err
	}
	next := s.db.Settings()
	year := fmt.Sprintf("-%d-", timeNow().Year())
	for _, inv := range s.db.Invoices.All() {
		if strings.Contains(inv.Number, year) {
			return models.Settings{}, fmt.Errorf(
				"des documents portent déjà un numéro de l'année en cours (%s) : la remise à zéro créerait des doublons", inv.Number)
		}
	}
	next.InvoiceCounter, next.PurchaseCounter = 0, 0
	if err := s.db.SaveSettings(next); err != nil {
		return models.Settings{}, err
	}
	s.log(u, "UPDATE", "settings", "settings", "Numérotation des factures et bons d'entrée remise à zéro")
	return next, nil
}

// DataLocation renvoie le chemin du dossier de données, affiché dans l'écran
// des paramètres : l'utilisateur doit savoir où vivent ses données.
func (s *Config) DataLocation() (map[string]string, error) {
	if _, err := s.guard(""); err != nil {
		return nil, err
	}
	return map[string]string{
		"root":    s.db.Dir,
		"data":    s.db.Dir + "/data",
		"backups": s.db.BackupDir(),
		"exports": s.db.Dir + "/exports",
	}, nil
}
