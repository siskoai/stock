package services

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"comptoir/internal/models"
	"comptoir/internal/util"
)

// Import de catalogue depuis un tableur.
//
// Reprendre un stock existant à la main, article par article, est le premier
// obstacle à l'adoption du logiciel. L'import lit le fichier que le commerçant
// a déjà — une liste exportée d'Excel — plutôt que d'exiger un format précis :
// les colonnes sont reconnues par leur intitulé, dans n'importe quel ordre, et
// celles qu'il ne comprend pas sont ignorées.
//
// L'analyse et l'application partagent le même code : ce que l'aperçu annonce
// est exactement ce que la confirmation exécute.

// ImportAction décrit ce qu'une ligne du fichier va produire.
type ImportAction string

const (
	ImportCreate ImportAction = "CREATE"
	ImportUpdate ImportAction = "UPDATE"
	ImportSkip   ImportAction = "SKIP" // ligne écartée : le message dit pourquoi
)

// ImportRow est le compte rendu d'une ligne.
type ImportRow struct {
	Line     int          `json:"line"` // numéro dans le fichier, en-tête comprise
	SKU      string       `json:"sku"`
	Name     string       `json:"name"`
	Action   ImportAction `json:"action"`
	Message  string       `json:"message"`
	Quantity int          `json:"quantity"`
}

// ImportReport résume un import, réel ou simulé.
type ImportReport struct {
	Applied           bool        `json:"applied"`
	Rows              []ImportRow `json:"rows"`
	Created           int         `json:"created"`
	Updated           int         `json:"updated"`
	Skipped           int         `json:"skipped"`
	CategoriesCreated []string    `json:"categoriesCreated"`
	Columns           []string    `json:"columns"` // colonnes reconnues
	Ignored           []string    `json:"ignored"` // colonnes non reconnues
}

// champs reconnus, par intitulé normalisé. Plusieurs libellés mènent au même
// champ : le fichier vient d'un tableur tenu à la main, pas d'une API.
var importAliases = map[string][]string{
	"name":     {"designation", "designation article", "libelle", "produit", "article", "nom", "description courte"},
	"sku":      {"reference", "ref", "sku", "code", "code article"},
	"barcode":  {"code barre", "code barres", "codebarre", "ean", "gencod"},
	"category": {"categorie", "famille", "rayon"},
	"brand":    {"marque"},
	"model":    {"modele"},
	"unit":     {"unite", "conditionnement"},
	"purchase": {"cout", "cout achat", "cout moyen", "prix achat", "prix d achat", "achat", "pa"},
	"sale":     {"prix", "prix vente", "prix de vente", "prix unitaire", "pv", "tarif"},
	"quantity": {"stock", "quantite", "qte", "stock initial", "quantite en stock"},
	"minStock": {"seuil", "seuil alerte", "seuil d alerte", "stock mini", "minimum"},
	"location": {"emplacement", "rayonnage", "localisation"},
	"warranty": {"garantie", "garantie mois"},
	"notes":    {"description", "notes", "commentaire"},
}

// ImportProducts analyse un fichier CSV et, si apply vaut vrai, l'applique.
//
// Un article dont la référence existe déjà est mis à jour ; les autres sont
// créés. Les quantités ne sont reprises qu'à la création, et par un mouvement
// d'inventaire : un import ne doit pas pouvoir modifier un stock en silence.
func (s *Catalog) ImportProducts(content string, apply bool) (ImportReport, error) {
	u, err := s.guard("catalog")
	if err != nil {
		return ImportReport{}, err
	}
	records, header, err := parseSheet(content)
	if err != nil {
		return ImportReport{}, err
	}
	columns, ignored := mapColumns(header)
	if _, ok := columns["name"]; !ok {
		return ImportReport{}, fmt.Errorf(
			"aucune colonne de désignation trouvée : la première ligne doit nommer les colonnes (« Désignation », « Prix de vente », …)")
	}

	settings := s.db.Settings()
	report := ImportReport{Applied: apply, Ignored: ignored}
	for key := range columns {
		report.Columns = append(report.Columns, key)
	}
	sort.Strings(report.Columns)

	// Index des références et des catégories existantes, pour éviter une
	// recherche linéaire par ligne sur un fichier de plusieurs centaines.
	existing := map[string]models.Product{}
	for _, p := range s.db.Products.All() {
		if p.SKU != "" {
			existing[strings.ToUpper(p.SKU)] = p
		}
	}
	categories := map[string]models.Category{}
	for _, c := range s.db.Categories.All() {
		categories[util.Slug(c.Name)] = c
	}
	seenSKU := map[string]int{} // référence → ligne, pour repérer les doublons

	for i, record := range records {
		line := i + 2 // en-tête comprise
		get := func(key string) string {
			idx, ok := columns[key]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		name := get("name")
		if name == "" {
			// Une ligne vide en fin de fichier est courante ; on ne la signale
			// que si elle contient autre chose.
			if strings.TrimSpace(strings.Join(record, "")) == "" {
				continue
			}
			report.Rows = append(report.Rows, ImportRow{
				Line: line, Action: ImportSkip, Message: "désignation absente",
			})
			report.Skipped++
			continue
		}

		sku := strings.ToUpper(get("sku"))
		if sku != "" {
			if before, dup := seenSKU[sku]; dup {
				report.Rows = append(report.Rows, ImportRow{
					Line: line, SKU: sku, Name: name, Action: ImportSkip,
					Message: fmt.Sprintf("référence déjà présente ligne %d", before),
				})
				report.Skipped++
				continue
			}
			seenSKU[sku] = line
		}

		sale, err := parseSheetMoney(get("sale"), settings.Decimals)
		if err != nil {
			report.Rows = append(report.Rows, ImportRow{
				Line: line, SKU: sku, Name: name, Action: ImportSkip,
				Message: "prix de vente illisible : " + get("sale"),
			})
			report.Skipped++
			continue
		}
		purchase, err := parseSheetMoney(get("purchase"), settings.Decimals)
		if err != nil {
			report.Rows = append(report.Rows, ImportRow{
				Line: line, SKU: sku, Name: name, Action: ImportSkip,
				Message: "coût d'achat illisible : " + get("purchase"),
			})
			report.Skipped++
			continue
		}
		quantity := parseSheetInt(get("quantity"))
		minStock := parseSheetInt(get("minStock"))
		warranty := parseSheetInt(get("warranty"))

		// Catégorie : rapprochée par son nom, créée si elle manque.
		categoryID := ""
		if label := get("category"); label != "" {
			key := util.Slug(label)
			if c, ok := categories[key]; ok {
				categoryID = c.ID
			} else {
				now := time.Now()
				c := models.Category{
					ID: util.NewID("cat"), Name: label, Color: "blue",
					Description: "Créée à l'import du catalogue",
					CreatedAt:   now, UpdatedAt: now,
				}
				if apply {
					if err := s.db.Categories.Insert(c); err != nil {
						return report, err
					}
				}
				categories[key] = c
				categoryID = c.ID
				report.CategoriesCreated = append(report.CategoriesCreated, label)
			}
		}

		input := ProductInput{
			SKU: sku, Name: name, Barcode: get("barcode"), CategoryID: categoryID,
			Brand: get("brand"), Model: get("model"), Description: get("notes"),
			Unit: get("unit"), PurchasePrice: purchase, SalePrice: sale,
			MinStock: minStock, Location: get("location"), WarrantyMonths: warranty,
			Active: true,
		}

		if current, found := existing[sku]; sku != "" && found {
			input.ID = current.ID
			// Un import ne redéfinit pas le coût moyen : il est le résultat des
			// réceptions enregistrées, pas une donnée à recopier.
			input.PurchasePrice = current.PurchasePrice
			row := ImportRow{
				Line: line, SKU: sku, Name: name, Action: ImportUpdate,
				Message: "fiche existante mise à jour",
			}
			if apply {
				if _, err := s.db.Products.Get(current.ID); err == nil {
					if _, err := s.SaveProduct(input); err != nil {
						row.Action, row.Message = ImportSkip, err.Error()
						report.Skipped++
						report.Rows = append(report.Rows, row)
						continue
					}
				}
			}
			report.Updated++
			report.Rows = append(report.Rows, row)
			continue
		}

		input.InitialQuantity = quantity
		row := ImportRow{
			Line: line, SKU: sku, Name: name, Action: ImportCreate,
			Quantity: quantity, Message: "nouvelle fiche",
		}
		if apply {
			created, err := s.SaveProduct(input)
			if err != nil {
				row.Action, row.Message = ImportSkip, err.Error()
				report.Skipped++
				report.Rows = append(report.Rows, row)
				continue
			}
			row.SKU = created.SKU
			existing[strings.ToUpper(created.SKU)] = created
		}
		report.Created++
		report.Rows = append(report.Rows, row)
	}

	if apply {
		s.log(u, "IMPORT", "product", "", "Import du catalogue : %d création(s), %d mise(s) à jour, %d ligne(s) écartée(s)",
			report.Created, report.Updated, report.Skipped)
	}
	return report, nil
}

// ImportTemplate renvoie un fichier d'exemple : les colonnes attendues et une
// ligne remplie. C'est plus court à expliquer qu'une documentation.
func (s *Catalog) ImportTemplate() (File, error) {
	if _, err := s.guard("catalog"); err != nil {
		return File{}, err
	}
	sh := newSheet(s.db.Settings().Decimals)
	sh.row("Désignation", "Référence", "Catégorie", "Marque", "Modèle",
		"Code-barres", "Unité", "Coût d'achat", "Prix de vente", "Stock", "Seuil d'alerte",
		"Emplacement", "Garantie (mois)")
	sh.row("Imprimante HP LaserJet M404", "IMP-HP-M404", "Imprimantes", "HP", "M404dn",
		"0195161234567", "pièce", "185000", "245000", "4", "2", "Rayon B1", "12")
	sh.row("Câble HDMI 2 m", "", "Accessoires", "", "",
		"", "pièce", "1500", "3000", "40", "10", "Rayon A3", "")
	f := sh.file("modele_import_catalogue")
	return f, nil
}

// ---------------------------------------------------------------------------
// Lecture du fichier
// ---------------------------------------------------------------------------

// parseSheet lit un CSV en devinant son séparateur. Un fichier issu d'Excel
// francophone utilise le point-virgule ; un fichier issu d'un outil anglophone,
// la virgule. Choisir le mauvais donnerait une seule colonne.
func parseSheet(content string) (records [][]string, header []string, err error) {
	content = strings.TrimPrefix(content, "\ufeff")
	if strings.TrimSpace(content) == "" {
		return nil, nil, fmt.Errorf("le fichier est vide")
	}
	firstLine := content
	if i := strings.IndexAny(content, "\r\n"); i > 0 {
		firstLine = content[:i]
	}
	comma := ','
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		comma = ';'
	} else if strings.Count(firstLine, "\t") > strings.Count(firstLine, ",") {
		comma = '\t'
	}

	r := csv.NewReader(strings.NewReader(content))
	r.Comma = comma
	r.FieldsPerRecord = -1 // les lignes courtes sont tolérées
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	all, err := r.ReadAll()
	if err != nil && !strings.Contains(err.Error(), "wrong number of fields") {
		return nil, nil, fmt.Errorf("fichier illisible : %w", err)
	}
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("le fichier ne contient aucune ligne")
	}
	if len(all) == 1 {
		return nil, nil, fmt.Errorf("le fichier ne contient qu'une ligne d'en-tête, aucun article")
	}
	return all[1:], all[0], nil
}

// mapColumns rapproche les intitulés du fichier des champs connus.
func mapColumns(header []string) (columns map[string]int, ignored []string) {
	columns = map[string]int{}
	for i, raw := range header {
		label := util.Slug(raw)
		if label == "" {
			continue
		}
		matched := ""
		for field, aliases := range importAliases {
			for _, alias := range aliases {
				if label == alias {
					matched = field
					break
				}
			}
			if matched != "" {
				break
			}
		}
		// Rapprochement approché : « prix de vente ht » retrouve « prix de vente ».
		if matched == "" {
			for field, aliases := range importAliases {
				for _, alias := range aliases {
					if len(alias) >= 4 && strings.Contains(label, alias) {
						matched = field
						break
					}
				}
				if matched != "" {
					break
				}
			}
		}
		if matched == "" {
			ignored = append(ignored, strings.TrimSpace(raw))
			continue
		}
		if _, taken := columns[matched]; !taken {
			columns[matched] = i
		}
	}
	return columns, ignored
}

// parseSheetMoney lit un montant tel qu'un tableur l'écrit : « 1 500 »,
// « 1500,50 », « 1 500,50 FCFA ». Renvoie des centièmes.
func parseSheetMoney(raw string, decimals int) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r == ',', r == '.', r == '-':
			return r
		default:
			return -1 // espaces, insécables, symboles monétaires
		}
	}, raw)
	if cleaned == "" || cleaned == "-" {
		return 0, fmt.Errorf("montant illisible")
	}

	negative := strings.HasPrefix(cleaned, "-")
	cleaned = strings.TrimPrefix(cleaned, "-")

	// Le dernier séparateur est décimal, sauf s'il est suivi de trois chiffres
	// dans une monnaie sans décimale : « 1.500 » vaut alors mille cinq cents.
	sep := strings.LastIndexAny(cleaned, ",.")
	integer, fraction := cleaned, ""
	if sep >= 0 {
		tail := cleaned[sep+1:]
		if !(len(tail) == 3 && decimals == 0) {
			integer, fraction = cleaned[:sep], tail
		}
	}
	integer = strings.NewReplacer(",", "", ".", "").Replace(integer)
	fraction = strings.NewReplacer(",", "", ".", "").Replace(fraction)

	units, err := strconv.ParseInt(orZero(integer), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("montant illisible")
	}
	cents := int64(0)
	if decimals > 0 && fraction != "" {
		padded := (fraction + "00")[:2]
		if c, err := strconv.ParseInt(padded, 10, 64); err == nil {
			cents = c
		}
	}
	total := units*100 + cents
	if negative {
		total = -total
	}
	return total, nil
}

// parseSheetInt lit une quantité ; toute valeur illisible ou négative vaut zéro.
func parseSheetInt(raw string) int {
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.TrimSpace(raw))
	if cleaned == "" {
		return 0
	}
	n, err := strconv.Atoi(cleaned)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func orZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
