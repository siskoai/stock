package pdfgen

import (
	"time"

	"comptoir/internal/models"
)

// reglagesAvecLogo produit des paramètres de boutique minimaux, avec ou sans
// logo, pour les tests de rendu.
func reglagesAvecLogo(dataURL string) models.Settings {
	s := models.DefaultSettings()
	s.CompanyName = "Sahel Informatique"
	s.City, s.Country = "Bamako", "Mali"
	s.LogoDataURL = dataURL
	return s
}

// factureMinimale produit une facture d'une ligne, suffisante pour vérifier
// qu'un document se rend.
func factureMinimale() models.Invoice {
	return models.Invoice{
		Number:       "FA-2026-0001",
		Date:         time.Date(2026, time.August, 18, 10, 0, 0, 0, time.Local),
		CustomerName: "Client comptoir",
		Lines: []models.DocLine{{
			ProductName: "Clavier", SKU: "CLA-1", Quantity: 1,
			UnitPrice: 500000, LineHT: 500000, TaxRate: 18,
			TaxAmount: 90000, LineTTC: 590000,
		}},
		SubtotalHT: 500000, TaxTotal: 90000, Total: 590000,
		Status: models.StatusIssued,
	}
}
