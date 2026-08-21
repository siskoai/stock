package pdfgen

import (
	"strings"
	"time"

	"comptoir/internal/models"
)

// Le générateur de rapports reçoit une description neutre, des indicateurs et
// des tableaux déjà formatés, plutôt que les structures métier. Le paquet
// pdfgen reste ainsi indépendant du paquet services : la mise en page ne
// change pas quand un calcul change, et l'inverse est vrai aussi.

// ReportKPI est un indicateur mis en avant en tête de rapport.
type ReportKPI struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Hint  string `json:"hint"`
}

// ReportColumn décrit une colonne de tableau.
type ReportColumn struct {
	Header string  `json:"header"`
	Width  float64 `json:"width"` // en millimètres ; 0 = réparti automatiquement
	Align  string  `json:"align"` // L, C ou R
}

// ReportTable est un tableau de rapport, avec une ligne de total optionnelle.
type ReportTable struct {
	Title    string         `json:"title"`
	Note     string         `json:"note"`
	Columns  []ReportColumn `json:"columns"`
	Rows     [][]string     `json:"rows"`
	TotalRow []string       `json:"totalRow"`
}

// ReportDoc est la description complète d'un rapport imprimable.
type ReportDoc struct {
	Title    string        `json:"title"`
	Subtitle string        `json:"subtitle"`
	Period   string        `json:"period"`
	KPIs     []ReportKPI   `json:"kpis"`
	Tables   []ReportTable `json:"tables"`
	Notes    []string      `json:"notes"`
}

// Report rend un rapport paginé au format A4 portrait.
func Report(rd ReportDoc, settings models.Settings) ([]byte, error) {
	d := newDoc(settings)
	p := d.pdf

	d.footer("", "Édité le "+time.Now().Format("02/01/2006 à 15h04")+" depuis Comptoir, document interne.")
	p.AddPage()
	d.header(rd.Title, rd.Period, time.Now(), "")

	if rd.Subtitle != "" {
		p.SetX(15)
		p.SetFont("Helvetica", "", 9.5)
		d.setColor(mutedRGB)
		p.MultiCell(180, 4.5, d.text(rd.Subtitle), "", "L", false)
		p.Ln(2)
	}

	// --- Bandeau d'indicateurs --------------------------------------------
	if len(rd.KPIs) > 0 {
		perRow := 4
		cellW := 180.0 / float64(perRow)
		for i := 0; i < len(rd.KPIs); i += perRow {
			end := minI(i+perRow, len(rd.KPIs))
			row := rd.KPIs[i:end]
			startY := p.GetY()

			for j, k := range row {
				x := 15 + float64(j)*cellW
				d.setFill(washRGB)
				p.Rect(x, startY, cellW-2, 18, "F")

				p.SetXY(x+3, startY+2.5)
				p.SetFont("Helvetica", "B", 6.5)
				d.setColor(mutedRGB)
				p.CellFormat(cellW-8, 3.5, d.text(strings.ToUpper(k.Label)), "", 2, "L", false, 0, "")

				p.SetFont("Helvetica", "B", 12)
				d.setColor(inkRGB)
				p.CellFormat(cellW-8, 6.5, d.text(k.Value), "", 2, "L", false, 0, "")

				if k.Hint != "" {
					p.SetFont("Helvetica", "", 6.5)
					d.setColor(mutedRGB)
					p.CellFormat(cellW-8, 3.5, d.text(k.Hint), "", 2, "L", false, 0, "")
				}
			}
			p.SetY(startY + 21)
		}
		p.Ln(2)
	}

	// --- Tableaux ----------------------------------------------------------
	for _, t := range rd.Tables {
		if p.GetY() > 235 {
			p.AddPage()
		}
		if t.Title != "" {
			p.SetX(15)
			p.SetFont("Helvetica", "B", 10.5)
			d.setColor(inkRGB)
			p.CellFormat(180, 7, d.text(t.Title), "", 1, "L", false, 0, "")
		}
		if t.Note != "" {
			p.SetX(15)
			p.SetFont("Helvetica", "", 7.5)
			d.setColor(mutedRGB)
			p.MultiCell(180, 3.8, d.text(t.Note), "", "L", false)
			p.Ln(1)
		}

		widths := resolveWidths(t.Columns)

		drawHead := func() {
			p.SetX(15)
			d.setFill(brandRGB)
			p.SetTextColor(255, 255, 255)
			p.SetFont("Helvetica", "B", 7)
			for i, c := range t.Columns {
				entete := d.ajuster(strings.ToUpper(c.Header), widths[i])
				p.CellFormat(widths[i], 6.5, d.text(entete), "", 0, align(c.Align), true, 0, "")
			}
			p.Ln(-1)
		}
		drawHead()

		p.SetFont("Helvetica", "", 8.5)
		d.setDraw(ruleRGB)
		p.SetLineWidth(0.15)
		for idx, row := range t.Rows {
			if p.GetY() > 262 {
				p.AddPage()
				drawHead()
				p.SetFont("Helvetica", "", 8.5)
			}
			fill := idx%2 == 1
			if fill {
				d.setFill(washRGB)
			}
			p.SetX(15)
			d.setColor(inkRGB)
			for i := range t.Columns {
				value := ""
				if i < len(row) {
					value = row[i]
				}
				p.CellFormat(widths[i], 5.8, d.text(d.ajuster(value, widths[i])), "", 0, align(t.Columns[i].Align), fill, 0, "")
			}
			p.Ln(-1)
			p.Line(15, p.GetY(), 195, p.GetY())
		}

		if len(t.TotalRow) > 0 {
			p.SetX(15)
			d.setFill([3]int{232, 236, 233})
			p.SetFont("Helvetica", "B", 9)
			d.setColor(inkRGB)
			for i := range t.Columns {
				value := ""
				if i < len(t.TotalRow) {
					value = t.TotalRow[i]
				}
				p.CellFormat(widths[i], 7, d.text(d.ajuster(value, widths[i])), "", 0, align(t.Columns[i].Align), true, 0, "")
			}
			p.Ln(-1)
		}
		p.Ln(6)
	}

	// --- Notes de lecture --------------------------------------------------
	if len(rd.Notes) > 0 {
		if p.GetY() > 245 {
			p.AddPage()
		}
		p.SetX(15)
		p.SetFont("Helvetica", "B", 7.5)
		d.setColor(mutedRGB)
		p.CellFormat(180, 5, d.text("NOTES DE LECTURE"), "", 1, "L", false, 0, "")
		p.SetFont("Helvetica", "", 7.5)
		for _, n := range rd.Notes {
			p.SetX(15)
			p.MultiCell(180, 3.6, d.text("- "+n), "", "L", false)
		}
	}

	return output(p)
}

// largeurLibreMin est la place minimale laissée à une colonne sans largeur
// imposée. En dessous, elle ne contient plus assez de caractères pour dire
// quoi que ce soit, et c'est en général la colonne de désignation.
const largeurLibreMin = 42.0

// resolveWidths répartit les 180 mm de la zone de texte entre les colonnes.
//
// Les colonnes sans largeur imposée se partagent ce qui reste, avec un plancher.
// Si les largeurs imposées ne laissent pas ce plancher, ce sont elles qui sont
// réduites au prorata : mieux vaut un montant un peu à l'étroit qu'une
// désignation réduite à sept lettres.
func resolveWidths(cols []ReportColumn) []float64 {
	const total = 180.0
	widths := make([]float64, len(cols))
	var imposees float64
	var libres int
	for i, c := range cols {
		if c.Width > 0 {
			widths[i] = c.Width
			imposees += c.Width
		} else {
			libres++
		}
	}
	if libres == 0 {
		// Tout est imposé : on ajuste à la zone de texte pour éviter un
		// débordement silencieux hors de la page.
		if imposees > 0 && imposees != total {
			facteur := total / imposees
			for i := range widths {
				widths[i] *= facteur
			}
		}
		return widths
	}

	besoin := float64(libres) * largeurLibreMin
	if reste := total - imposees; reste < besoin {
		// Les colonnes imposées prennent trop de place : on les resserre juste
		// assez pour dégager le plancher des colonnes libres.
		disponible := total - besoin
		if disponible < 0 {
			disponible = 0
		}
		facteur := disponible / imposees
		for i, c := range cols {
			if c.Width > 0 {
				widths[i] = c.Width * facteur
			}
		}
		imposees = disponible
	}
	each := (total - imposees) / float64(libres)
	for i := range widths {
		if widths[i] == 0 {
			widths[i] = each
		}
	}
	return widths
}

func align(a string) string {
	switch strings.ToUpper(a) {
	case "R", "C":
		return strings.ToUpper(a)
	default:
		return "L"
	}
}

// ajuster raccourcit un texte pour qu'il tienne dans une colonne.
//
// La largeur est mesurée par le moteur, avec la police en cours, plutôt
// qu'estimée à partir d'un nombre de caractères : « Imprimante » et « WWWWWWWWWW »
// comptent dix lettres et n'occupent pas la même place. Une estimation trop
// prudente coupe des textes qui tenaient ; une estimation trop généreuse les
// laisse déborder sur la colonne voisine.
func (d *doc) ajuster(texte string, largeurMM float64) string {
	const marge = 2.0 // les deux filets de la cellule
	dispo := largeurMM - marge
	if dispo <= 0 || texte == "" {
		return texte
	}
	rendu := d.text(texte)
	if d.pdf.GetStringWidth(rendu) <= dispo {
		return texte
	}
	points := "…"
	largeurPoints := d.pdf.GetStringWidth(d.text(points))
	lettres := []rune(texte)
	for len(lettres) > 0 {
		lettres = lettres[:len(lettres)-1]
		essai := strings.TrimRight(string(lettres), " ")
		if d.pdf.GetStringWidth(d.text(essai))+largeurPoints <= dispo {
			return essai + points
		}
	}
	return points
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}
