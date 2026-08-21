package pdfgen

import (
	"fmt"
	"strings"
	"time"

	"comptoir/internal/models"
)

// Lettre de relance.
//
// Le ton est celui d'un commerçant qui veut être payé et garder son client :
// factuel, sans menace, et surtout précis. Un client relancé conteste rarement
// un tableau qui nomme chaque facture, sa date et son solde ; il conteste
// souvent un total qu'on lui annonce sans détail.

// RelanceLigne est une facture rappelée dans la lettre.
type RelanceLigne struct {
	Numero   string
	Date     time.Time
	Echeance *time.Time
	Total    int64
	Regle    int64
	Solde    int64
	Retard   int // jours, zéro si l'échéance n'est pas passée
}

// RelanceDoc décrit la lettre à produire.
type RelanceDoc struct {
	Client    string
	Societe   string
	Adresse   string
	Ville     string
	Telephone string

	Lignes   []RelanceLigne
	Total    int64
	EnRetard int64
}

// ReminderLetter rend une lettre de relance.
func ReminderLetter(rd RelanceDoc, settings models.Settings) ([]byte, error) {
	d := newDoc(settings)
	p := d.pdf

	d.footer(settings.InvoiceTerms,
		"Si votre règlement nous est parvenu entre-temps, considérez ce courrier comme sans objet.")
	p.AddPage()
	d.header("Relevé de compte", "Relance du "+time.Now().Format("02/01/2006"), time.Now(), "")

	// --- Destinataire --------------------------------------------------------
	y := p.GetY()
	d.setFill(washRGB)
	p.Rect(110, y, 85, 30, "F")
	p.SetXY(112, y+3.5)
	p.SetFont("Helvetica", "B", 7.5)
	d.setColor(mutedRGB)
	p.CellFormat(81, 4, d.text("À L'ATTENTION DE"), "", 2, "L", false, 0, "")
	p.SetFont("Helvetica", "B", 10.5)
	d.setColor(inkRGB)
	p.CellFormat(81, 5.5, d.text(rd.Client), "", 2, "L", false, 0, "")
	p.SetFont("Helvetica", "", 8.5)
	d.setColor(mutedRGB)
	for _, ligne := range []string{rd.Societe, rd.Adresse, rd.Ville, rd.Telephone} {
		if strings.TrimSpace(ligne) != "" {
			p.CellFormat(81, 4, d.text(ligne), "", 2, "L", false, 0, "")
		}
	}

	p.SetXY(15, y+36)

	// --- Corps ---------------------------------------------------------------
	p.SetFont("Helvetica", "", 10)
	d.setColor(inkRGB)
	intro := fmt.Sprintf(
		"Madame, Monsieur,\n\nSauf erreur de notre part, notre compte présente à ce jour un solde de %s en notre faveur. "+
			"Vous en trouverez le détail ci-dessous.",
		d.money(rd.Total))
	if rd.EnRetard > 0 {
		intro += fmt.Sprintf(
			" Sur ce montant, %s correspond à des échéances déjà dépassées.", d.money(rd.EnRetard))
	}
	p.MultiCell(180, 5, d.text(intro), "", "L", false)
	p.Ln(4)

	// --- Tableau -------------------------------------------------------------
	largeurs := []float64{32, 26, 26, 32, 30, 34}
	entetes := []string{"FACTURE", "DATE", "ÉCHÉANCE", "TOTAL", "RÉGLÉ", "RESTE DÛ"}
	alignements := []string{"L", "L", "L", "R", "R", "R"}

	p.SetFont("Helvetica", "B", 7)
	d.setFill(brandRGB)
	p.SetTextColor(255, 255, 255)
	for i, entete := range entetes {
		p.CellFormat(largeurs[i], 6.5, d.text(entete), "", 0, alignements[i], true, 0, "")
	}
	p.Ln(-1)

	p.SetFont("Helvetica", "", 8.5)
	d.setDraw(ruleRGB)
	p.SetLineWidth(0.15)
	for i, l := range rd.Lignes {
		if p.GetY() > 245 {
			p.AddPage()
		}
		fond := i%2 == 1
		if fond {
			d.setFill(washRGB)
		}
		echeance := "non convenue"
		if l.Echeance != nil {
			echeance = l.Echeance.Format("02/01/2006")
		}
		d.setColor(inkRGB)
		p.CellFormat(largeurs[0], 5.8, d.text(l.Numero), "", 0, "L", fond, 0, "")
		p.CellFormat(largeurs[1], 5.8, d.text(l.Date.Format("02/01/2006")), "", 0, "L", fond, 0, "")
		if l.Retard > 0 {
			d.setColor(accentRGB)
		}
		p.CellFormat(largeurs[2], 5.8, d.text(echeance), "", 0, "L", fond, 0, "")
		d.setColor(inkRGB)
		p.CellFormat(largeurs[3], 5.8, d.money(l.Total), "", 0, "R", fond, 0, "")
		p.CellFormat(largeurs[4], 5.8, d.money(l.Regle), "", 0, "R", fond, 0, "")
		p.SetFont("Helvetica", "B", 8.5)
		p.CellFormat(largeurs[5], 5.8, d.money(l.Solde), "", 0, "R", fond, 0, "")
		p.SetFont("Helvetica", "", 8.5)
		p.Ln(-1)
		p.Line(15, p.GetY(), 195, p.GetY())
	}

	// --- Total ---------------------------------------------------------------
	p.SetFont("Helvetica", "B", 10)
	d.setFill(accentWashRGB)
	d.setColor(accentRGB)
	p.CellFormat(largeurs[0]+largeurs[1]+largeurs[2]+largeurs[3]+largeurs[4], 8,
		d.text("TOTAL RESTANT DÛ"), "", 0, "R", true, 0, "")
	p.CellFormat(largeurs[5], 8, d.money(rd.Total), "", 1, "R", true, 0, "")
	p.Ln(6)

	// --- Formule -------------------------------------------------------------
	p.SetFont("Helvetica", "", 10)
	d.setColor(inkRGB)
	p.MultiCell(180, 5, d.text(
		"Nous vous remercions de bien vouloir procéder au règlement de cette somme dans les meilleurs délais, "+
			"ou de nous contacter si un point vous paraît devoir être discuté.\n\n"+
			"Nous vous prions d'agréer, Madame, Monsieur, l'expression de nos salutations distinguées."),
		"", "L", false)
	p.Ln(8)

	p.SetFont("Helvetica", "B", 9.5)
	p.CellFormat(180, 5, d.text(orDefault(settings.CompanyName, "La direction")), "", 1, "R", false, 0, "")
	if settings.Phone != "" {
		p.SetFont("Helvetica", "", 8.5)
		d.setColor(mutedRGB)
		p.CellFormat(180, 4.5, d.text(settings.Phone), "", 1, "R", false, 0, "")
	}

	return output(p)
}
