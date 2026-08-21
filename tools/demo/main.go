// Commande demo : construit une boutique de démonstration complète.
//
// Elle sert à produire les captures du guide d'utilisation. Les données passent
// par les mêmes services que l'application : les stocks, les marges et les
// totaux sont donc exacts et cohérents entre eux, ce qu'un jeu de données écrit
// à la main ne garantirait jamais.
//
//	go run ./tools/demo -dir /chemin/vers/donnees
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/services"
	"comptoir/internal/storage"
)

func main() {
	dir := flag.String("dir", "", "répertoire de données à créer")
	flag.Parse()
	if *dir == "" {
		log.Fatal("indiquez -dir")
	}
	if err := os.RemoveAll(*dir); err != nil {
		log.Fatal(err)
	}
	if err := construire(*dir); err != nil {
		log.Fatalf("construction de la démonstration : %v", err)
	}
	fmt.Println("boutique de démonstration prête dans", *dir)
}

func construire(dir string) error {
	db, err := storage.Open(dir)
	if err != nil {
		return err
	}
	// Un coût bcrypt minimal : ce jeu de données est jetable, il n'a pas à
	// coûter une seconde de calcul par compte.
	auth.Cost = 4
	sec := auth.New(db.Users, 480)

	session := services.NewSession(db, sec)
	if _, err := session.Setup(services.SetupInput{
		Username: "aissata", FullName: "Aïssata Traoré", Password: "demo2026a",
		CompanyName: "Sahel Informatique", LegalForm: "SARL",
		TaxID: "MLI-2019-4471", RCCM: "BKO-2019-B-1182",
		Address: "Avenue de la Nation, immeuble Kanté", City: "Bamako", Country: "Mali",
		Phone: "+223 20 22 45 90", Email: "contact@sahel-informatique.ml",
		Currency: "XOF", CurrencySymbol: "FCFA", Decimals: 0,
		DefaultTaxRate: 18, SeedCategories: true,
		AutoBackup: true, BackupsToKeep: 30, Theme: "light",
	}); err != nil {
		return fmt.Errorf("configuration : %w", err)
	}
	if _, err := sec.CreateUser("ousmane", "Ousmane Coulibaly", "demo2026a", models.RoleSeller); err != nil {
		return err
	}
	if _, err := sec.CreateUser("fanta", "Fanta Sidibé", "demo2026a", models.RoleManager); err != nil {
		return err
	}

	catalogue := services.NewCatalog(db, sec)
	achats := services.NewPurchases(db, sec)
	ventes := services.NewSales(db, sec)
	stock := services.NewStock(db, sec)
	charges := services.NewExpenses(db, sec)

	// --- Tiers --------------------------------------------------------------
	fournisseurs := map[string]string{}
	for _, f := range []struct{ nom, ville, tel string }{
		{"Techno Import SARL", "Bamako", "+223 20 21 11 40"},
		{"Dakar Digital", "Dakar", "+221 33 849 20 00"},
		{"Guangzhou Star Trading", "Canton", "+86 20 3888 1122"},
	} {
		p, err := catalogue.SaveParty(services.PartyInput{
			Type: models.PartySupplier, Name: f.nom, City: f.ville, Phone: f.tel, Active: true,
		})
		if err != nil {
			return err
		}
		fournisseurs[f.nom] = p.ID
	}

	clients := map[string]string{}
	for _, c := range []struct{ nom, societe, tel, nif string }{
		{"Cabinet Diallo & Associés", "Cabinet Diallo", "+223 76 12 44 08", "MLI-2016-2290"},
		{"Lycée Askia Mohamed", "", "+223 20 29 31 07", "MLI-2004-0091"},
		{"Pharmacie du Fleuve", "", "+223 66 55 12 90", "MLI-2018-3320"},
		{"Mariam Keïta", "", "+223 78 90 21 33", ""},
		{"Bureau Conseil Sanogo", "BCS", "+223 44 21 09 76", "MLI-2021-5510"},
	} {
		p, err := catalogue.SaveParty(services.PartyInput{
			Type: models.PartyCustomer, Name: c.nom, Company: c.societe,
			Phone: c.tel, TaxID: c.nif, City: "Bamako", Active: true,
		})
		if err != nil {
			return err
		}
		clients[c.nom] = p.ID
	}

	// --- Catalogue ----------------------------------------------------------
	cats := map[string]string{}
	liste, err := catalogue.ListCategories()
	if err != nil {
		return err
	}
	for _, c := range liste {
		cats[c.Name] = c.ID
	}

	type article struct {
		nom, sku, cat, marque, modele, emplacement string
		achat, vente                               int64
		seuil                                      int
	}
	articles := []article{
		{"Ordinateur portable HP 250 G9", "PC-HP-250", "Ordinateurs portables", "HP", "250 G9", "Rayon A1", 27500000, 39500000, 3},
		{"Ordinateur portable Lenovo V15", "PC-LEN-V15", "Ordinateurs portables", "Lenovo", "V15 G4", "Rayon A1", 24000000, 34500000, 3},
		{"MacBook Air 13 M2", "PC-APP-MBA", "Ordinateurs portables", "Apple", "Air M2", "Coffre", 78000000, 99000000, 1},
		{"Unité centrale Dell OptiPlex", "PC-DEL-OPT", "Ordinateurs de bureau", "Dell", "OptiPlex 3000", "Rayon A3", 31000000, 43000000, 2},
		{"Écran Samsung 24 pouces", "ECR-SAM-24", "Écrans", "Samsung", "S33GC", "Rayon B1", 8500000, 12500000, 4},
		{"Écran Dell 27 pouces", "ECR-DEL-27", "Écrans", "Dell", "P2723D", "Rayon B1", 14500000, 20000000, 2},
		{"Vidéoprojecteur Epson EB-E01", "ECR-EPS-E01", "Écrans", "Epson", "EB-E01", "Rayon B3", 19500000, 27500000, 1},
		{"Imprimante HP LaserJet M211d", "IMP-HP-M211", "Imprimantes", "HP", "M211d", "Rayon C1", 12000000, 17500000, 2},
		{"Imprimante Canon PIXMA G3420", "IMP-CAN-G34", "Imprimantes", "Canon", "G3420", "Rayon C1", 9800000, 14500000, 2},
		{"Multifonction Epson L3250", "IMP-EPS-L32", "Imprimantes", "Epson", "L3250", "Rayon C2", 13500000, 19500000, 2},
		{"Toner HP 106A noir", "CON-HP-106A", "Consommables", "HP", "106A", "Rayon D1", 3200000, 4900000, 8},
		{"Cartouche Canon GI-490 noir", "CON-CAN-490", "Consommables", "Canon", "GI-490", "Rayon D1", 850000, 1500000, 12},
		{"Rame papier A4 80g", "CON-PAP-A4", "Consommables", "Navigator", "A4 80g", "Rayon D2", 250000, 450000, 20},
		{"Barrette RAM 8 Go DDR4", "COM-RAM-8G", "Composants", "Kingston", "DDR4 3200", "Tiroir E1", 1800000, 3000000, 6},
		{"Disque SSD 512 Go", "COM-SSD-512", "Composants", "Crucial", "BX500", "Tiroir E1", 2400000, 3900000, 5},
		{"Alimentation ATX 500W", "COM-ALI-500", "Composants", "Cooler Master", "MWE 500", "Tiroir E2", 1600000, 2700000, 3},
		{"Routeur TP-Link Archer C6", "RES-TPL-C6", "Réseau", "TP-Link", "Archer C6", "Rayon F1", 1900000, 3200000, 5},
		{"Switch 8 ports Gigabit", "RES-TPL-SW8", "Réseau", "TP-Link", "TL-SG108", "Rayon F1", 1400000, 2400000, 4},
		{"Câble réseau RJ45 3 m", "RES-CAB-3M", "Réseau", "", "Cat 6", "Rayon F2", 90000, 200000, 30},
		{"Clavier + souris sans fil", "ACC-LOG-MK2", "Accessoires", "Logitech", "MK270", "Rayon G1", 1100000, 1900000, 8},
		{"Onduleur 650 VA", "ACC-ULT-650", "Accessoires", "Ultra Power", "UP-650", "Rayon G2", 2800000, 4500000, 4},
		{"Sacoche 15 pouces", "ACC-SAC-15", "Accessoires", "", "Nylon", "Rayon G3", 450000, 900000, 10},
	}

	ids := map[string]string{}
	for _, a := range articles {
		p, err := catalogue.SaveProduct(services.ProductInput{
			Name: a.nom, SKU: a.sku, CategoryID: cats[a.cat],
			Brand: a.marque, Model: a.modele, Location: a.emplacement,
			Unit: "pièce", PurchasePrice: 0, SalePrice: a.vente,
			MinStock: a.seuil, WarrantyMonths: 12, Active: true,
		})
		if err != nil {
			return fmt.Errorf("article %q : %w", a.nom, err)
		}
		ids[a.sku] = p.ID
	}

	// --- Réceptions fournisseur, il y a deux et un mois ---------------------
	maintenant := time.Now()
	receptions := []struct {
		fournisseur string
		jours       int
		reference   string
		frais       int64
		lignes      map[string]struct {
			qte  int
			cout int64
		}
	}{
		{"Techno Import SARL", 62, "TI-2026-0418", 12000000, map[string]struct {
			qte  int
			cout int64
		}{
			"PC-HP-250": {22, 27500000}, "PC-LEN-V15": {18, 24000000},
			"ECR-SAM-24": {40, 8500000}, "IMP-HP-M211": {20, 12000000},
		}},
		{"Dakar Digital", 41, "DD-8871", 4500000, map[string]struct {
			qte  int
			cout int64
		}{
			"CON-HP-106A": {70, 3200000}, "CON-CAN-490": {120, 850000},
			"CON-PAP-A4": {180, 250000}, "COM-RAM-8G": {60, 1800000},
			"COM-SSD-512": {48, 2400000},
		}},
		{"Guangzhou Star Trading", 24, "GZ-2026-119", 21000000, map[string]struct {
			qte  int
			cout int64
		}{
			"RES-TPL-C6": {50, 1900000}, "RES-TPL-SW8": {40, 1400000},
			"RES-CAB-3M": {220, 90000}, "ACC-LOG-MK2": {75, 1100000},
			"ACC-ULT-650": {34, 2800000}, "ACC-SAC-15": {90, 450000},
		}},
		{"Techno Import SARL", 9, "TI-2026-0533", 8000000, map[string]struct {
			qte  int
			cout int64
		}{
			"PC-DEL-OPT": {14, 31000000}, "ECR-DEL-27": {18, 14500000},
			"IMP-EPS-L32": {14, 13500000}, "IMP-CAN-G34": {12, 9800000},
			"PC-APP-MBA": {4, 78000000}, "ECR-EPS-E01": {6, 19500000},
			"COM-ALI-500": {22, 1600000},
		}},
	}
	for _, r := range receptions {
		var lignes []services.PurchaseLineInput
		for sku, l := range r.lignes {
			lignes = append(lignes, services.PurchaseLineInput{
				ProductID: ids[sku], Quantity: l.qte, UnitCost: l.cout, TaxRate: taux(0),
			})
		}
		if _, err := achats.CreatePurchase(services.PurchaseInput{
			SupplierID: fournisseurs[r.fournisseur], Reference: r.reference,
			Date: jour(maintenant, -r.jours), OtherCosts: r.frais, Lines: lignes,
		}); err != nil {
			return fmt.Errorf("réception %s : %w", r.reference, err)
		}
	}

	// --- Ventes réparties sur deux mois -------------------------------------
	tirage := rand.New(rand.NewSource(20260818))
	vendables := []string{
		"PC-HP-250", "PC-LEN-V15", "ECR-SAM-24", "IMP-HP-M211", "CON-HP-106A",
		"CON-CAN-490", "CON-PAP-A4", "COM-RAM-8G", "COM-SSD-512", "RES-TPL-C6",
		"RES-CAB-3M", "ACC-LOG-MK2", "ACC-ULT-650", "ACC-SAC-15", "RES-TPL-SW8",
		"IMP-CAN-G34", "ECR-DEL-27", "COM-ALI-500",
	}
	nomsClients := []string{
		"Cabinet Diallo & Associés", "Lycée Askia Mohamed", "Pharmacie du Fleuve",
		"Mariam Keïta", "Bureau Conseil Sanogo",
	}
	reglements := []models.PaymentMethod{
		models.PayCash, models.PayCash, models.PayCash,
		models.PayMobile, models.PayMobile, models.PayTransfer, models.PayCheck,
	}

	for j := 58; j >= 0; j-- {
		date := maintenant.AddDate(0, 0, -j)
		if date.Weekday() == time.Sunday {
			continue // le magasin est fermé
		}
		iso := date.Format("2006-01-02")
		nb := 1 + tirage.Intn(3)
		for v := 0; v < nb; v++ {
			var lignes []services.InvoiceLineInput
			for l := 0; l < 1+tirage.Intn(3); l++ {
				sku := vendables[tirage.Intn(len(vendables))]
				lignes = append(lignes, services.InvoiceLineInput{
					ProductID: ids[sku], Quantity: 1 + tirage.Intn(3),
				})
			}
			in := services.InvoiceInput{
				Date: iso, Lines: lignes,
				PaymentMethod: reglements[tirage.Intn(len(reglements))],
			}
			// Deux ventes sur trois se font au comptoir, sans fiche client.
			if tirage.Intn(3) == 0 {
				in.CustomerID = clients[nomsClients[tirage.Intn(len(nomsClients))]]
			}
			facture, err := ventes.CreateInvoice(in)
			if err != nil {
				continue // stock épuisé sur cet article : la vente suivante prendra le relais
			}
			// Règlement : comptant le plus souvent, partiel parfois, à crédit rarement.
			switch tirage.Intn(10) {
			case 0, 1:
				if _, err := ventes.RegisterPayment(services.PaymentInput{
					InvoiceID: facture.ID, Amount: facture.Total / 2,
				}); err != nil {
					return err
				}
			case 2:
				// laissée impayée
			default:
				if _, err := ventes.RegisterPayment(services.PaymentInput{
					InvoiceID: facture.ID, Amount: facture.Total,
				}); err != nil {
					return err
				}
			}
		}
	}

	// --- Ventes à crédit, à divers stades de retard --------------------------
	// Elles peuplent l'écran des créances : sans elles, le classement par
	// ancienneté n'aurait rien à montrer.
	credits := []struct {
		client   string
		sku      string
		quantite int
		jours    int  // échéance, en jours par rapport à aujourd'hui
		acompte  bool // un versement partiel a déjà été fait
	}{
		{"Cabinet Diallo & Associés", "PC-HP-250", 2, 12, false},
		{"Lycée Askia Mohamed", "IMP-EPS-L32", 3, -8, true},
		{"Pharmacie du Fleuve", "PC-LEN-V15", 1, -34, false},
		{"Bureau Conseil Sanogo", "CON-HP-106A", 6, -67, true},
		{"Mariam Keïta", "ACC-ULT-650", 2, -103, false},
	}
	for i, c := range credits {
		facture, err := ventes.CreateInvoice(services.InvoiceInput{
			CustomerID: clients[c.client], PaymentMethod: models.PayCredit,
			Date:    jour(maintenant, -(20 + i*9)),
			DueDate: jour(maintenant, c.jours),
			Lines:   []services.InvoiceLineInput{{ProductID: ids[c.sku], Quantity: c.quantite}},
			Notes:   "Enlèvement immédiat, règlement convenu à l'échéance.",
		})
		if err != nil {
			continue // stock épuisé sur cet article
		}
		if c.acompte {
			if _, err := ventes.RegisterPayment(services.PaymentInput{
				InvoiceID: facture.ID, Amount: facture.Total / 3,
				Method: models.PayMobile, Note: "acompte à l'enlèvement",
			}); err != nil {
				return err
			}
		}
	}

	// --- Un devis en cours ---------------------------------------------------
	if _, err := ventes.CreateInvoice(services.InvoiceInput{
		Draft: true, Date: jour(maintenant, -2),
		CustomerID: clients["Lycée Askia Mohamed"],
		Notes:      "Équipement de la salle informatique. Validité 30 jours.",
		Lines: []services.InvoiceLineInput{
			{ProductID: ids["PC-DEL-OPT"], Quantity: 4},
			{ProductID: ids["ECR-DEL-27"], Quantity: 4},
			{ProductID: ids["RES-TPL-SW8"], Quantity: 1},
			{ProductID: ids["RES-CAB-3M"], Quantity: 10},
		},
	}); err != nil {
		return fmt.Errorf("devis : %w", err)
	}

	// --- Aléas du magasin ----------------------------------------------------
	if _, err := stock.DeclareDefective(services.MovementInput{
		ProductID: ids["ECR-SAM-24"], Quantity: 1, Date: jour(maintenant, -12),
		Reason: "Dalle fêlée constatée au déballage", Notes: "Carton n° 4 de la réception TI-2026-0418",
	}); err != nil {
		return err
	}
	if _, err := stock.ReturnFromCustomer(services.MovementInput{
		ProductID: ids["ACC-LOG-MK2"], Quantity: 1, Date: jour(maintenant, -6),
		PartyID: clients["Mariam Keïta"], Restock: true,
		Reason: "Erreur de modèle, échange demandé",
	}); err != nil {
		return err
	}
	if _, err := stock.DeclareDefective(services.MovementInput{
		ProductID: ids["ACC-ULT-650"], Quantity: 2, Date: jour(maintenant, -4),
		Reason: "Ne tient pas la charge",
	}); err != nil {
		return err
	}
	if _, err := stock.ScrapDefective(services.MovementInput{
		ProductID: ids["ACC-ULT-650"], Quantity: 1, Date: jour(maintenant, -3),
		Reason: "Batterie hors service, non réparable",
	}); err != nil {
		return err
	}
	if _, err := stock.AdjustInventory(services.InventoryInput{
		ProductID: ids["CON-PAP-A4"], CountedSound: compteur(db, ids["CON-PAP-A4"]) - 2,
		Reason: "Inventaire mensuel, écart constaté sur le rayon D2", Date: jour(maintenant, -1),
	}); err != nil {
		return err
	}

	// --- Charges d'exploitation ----------------------------------------------
	for _, mois := range []int{1, 0} {
		base := time.Date(maintenant.Year(), maintenant.Month()-time.Month(mois), 3, 10, 0, 0, 0, time.Local)
		for _, c := range []struct {
			rubrique, libelle, beneficiaire string
			montant                         int64
		}{
			{"Loyer", "Loyer du magasin", "SCI Kanté", 25000000},
			{"Salaires", "Salaires de l'équipe", "", 42000000},
			{"Électricité", "Facture EDM", "EDM SA", 6800000},
			{"Internet", "Abonnement fibre", "Orange Mali", 3500000},
			{"Transport", "Livraisons et déplacements", "", 2400000},
			{"Impôts et taxes", "Patente trimestrielle", "Direction des impôts", 4500000},
		} {
			if _, err := charges.SaveExpense(services.ExpenseInput{
				Date: base.Format("2006-01-02"), Category: c.rubrique, Label: c.libelle,
				Amount: c.montant, Beneficiary: c.beneficiaire,
				PaymentMethod: models.PayTransfer,
			}); err != nil {
				return err
			}
		}
	}

	config := services.NewConfig(db, sec)
	reglages := db.Settings()
	reglages.InvoiceTerms = "Marchandise vendue ne peut être ni reprise ni échangée, sauf défaut constaté sous 7 jours. Garantie constructeur selon les conditions du fabricant."
	reglages.InvoiceFooter = "Merci de votre confiance."
	reglages.LogoDataURL = logoDemo
	if _, err := config.Save(reglages); err != nil {
		return err
	}

	fmt.Printf("  %d articles, %d factures, %d mouvements, %d charges\n",
		db.Products.Count(), db.Invoices.Count(), db.Movements.Count(), db.Expenses.Count())
	return nil
}

func taux(v float64) *float64 { return &v }

func jour(base time.Time, decalage int) string {
	return base.AddDate(0, 0, decalage).Format("2006-01-02")
}

func compteur(db *storage.Database, id string) int {
	p, err := db.Products.Get(id)
	if err != nil {
		return 0
	}
	return p.Quantity
}
