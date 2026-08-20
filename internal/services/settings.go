package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
)

// Config expose les paramètres de l'application : identité de l'entreprise,
// monnaie, taxes, numérotation, sauvegardes. Lecture ouverte à tout compte
// connecté, l'interface a besoin du symbole monétaire et du nom de la boutique
// pour s'afficher -, écriture réservée au rôle Administrateur.
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

// Currencies est la liste proposée au paramétrage comme au premier démarrage.
// Elle vit ici, en un seul endroit : deux listes divergeraient.
var Currencies = []CurrencyPreset{
	{"XOF", "FCFA", 0, "Franc CFA (UEMOA)"},
	{"XAF", "FCFA", 0, "Franc CFA (CEMAC)"},
	{"GNF", "GNF", 0, "Franc guinéen"},
	{"MRU", "MRU", 2, "Ouguiya mauritanien"},
	{"MAD", "DH", 2, "Dirham marocain"},
	{"EUR", "€", 2, "Euro"},
	{"USD", "$", 2, "Dollar américain"},
}

// Presets renvoie les valeurs proposées dans l'écran de paramétrage :
// monnaies, rubriques de charges et modes de règlement.
func (s *Config) Presets() (map[string]any, error) {
	if _, err := s.guard(""); err != nil {
		return nil, err
	}
	return map[string]any{
		"currencies":        Currencies,
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
	s.log(u, "UPDATE", "settings", "settings", "Paramètres modifiés, %s, %s, TVA %.2f %%",
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

// EffacementInput porte la confirmation d'un effacement total.
type EffacementInput struct {
	// Confirmation doit reproduire le nom de l'entreprise, à la casse près.
	// Taper un nom entier laisse le temps de changer d'avis, ce qu'une case à
	// cocher ne fait pas.
	Confirmation string `json:"confirmation"`

	// GarderUneSauvegarde conserve une dernière archive avant l'effacement.
	// À décocher seulement pour céder ou mettre au rebut le poste : il ne
	// resterait alors plus rien à récupérer.
	GarderUneSauvegarde bool `json:"garderUneSauvegarde"`
}

// Effacement rend compte de ce qui a été supprimé.
type Effacement struct {
	Sauvegarde string `json:"sauvegarde,omitempty"`
}

// EraseAllData efface l'intégralité des données de ce poste.
//
// L'opération est irréversible et laisse le logiciel dans l'état d'un premier
// démarrage. Elle existe pour deux situations : repartir d'une base propre après
// une période d'essai, et céder ou mettre au rebut l'ordinateur sans y laisser
// le fichier de ses clients.
//
// L'application doit être redémarrée ensuite : les collections chargées en
// mémoire ne correspondent plus à rien.
func (s *Config) EraseAllData(in EffacementInput) (Effacement, error) {
	u, err := s.guard("settings")
	if err != nil {
		return Effacement{}, err
	}
	attendu := trim(s.db.Settings().CompanyName)
	if !strings.EqualFold(trim(in.Confirmation), attendu) {
		return Effacement{}, fmt.Errorf(
			"pour confirmer, saisissez exactement le nom de l'entreprise : « %s »", attendu)
	}

	resultat := Effacement{}
	if in.GarderUneSauvegarde {
		info, err := s.db.Backup("avant-effacement")
		if err != nil {
			return Effacement{}, fmt.Errorf(
				"la sauvegarde préalable a échoué, l'effacement est annulé : %w", err)
		}
		resultat.Sauvegarde = info.Path
	}

	// La trace est écrite avant la suppression : elle disparaîtra avec le
	// reste, mais elle figurera dans l'archive que l'on vient de prendre.
	s.log(u, "ERASE", "settings", "settings",
		"Effacement de toutes les données du poste demandé par « %s », sauvegarde conservée : %t",
		u.Username, in.GarderUneSauvegarde)

	if err := os.RemoveAll(filepath.Join(s.db.Dir, "data")); err != nil {
		return Effacement{}, fmt.Errorf("suppression des données : %w", err)
	}
	if !in.GarderUneSauvegarde {
		if err := os.RemoveAll(s.db.BackupDir()); err != nil {
			return Effacement{}, fmt.Errorf("suppression des sauvegardes : %w", err)
		}
		if err := os.RemoveAll(filepath.Join(s.db.Dir, "exports")); err != nil {
			return Effacement{}, fmt.Errorf("suppression des exports : %w", err)
		}
	}
	s.auth.Logout()
	return resultat, nil
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
