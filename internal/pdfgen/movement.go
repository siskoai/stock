package pdfgen

import (
	"strings"

	"comptoir/internal/models"
)

// Bon de mouvement.
//
// Une sortie de stock qui ne passe pas par une facture laisse une trace dans le
// journal, mais rien à faire signer. Ce document comble ce manque : il nomme
// l'article, la quantité, le motif et l'opérateur, et porte deux emplacements de
// signature. C'est la pièce qu'on classe quand de la marchandise quitte le
// magasin sans être vendue, ou qu'un client rapporte un article.

// mouvementTitres donne à chaque type de mouvement le nom que porte son
// justificatif dans un commerce.
var mouvementTitres = map[models.MovementType]string{
	models.MovementOut:            "Bon de sortie",
	models.MovementIn:             "Bon de réception",
	models.MovementReturnCustomer: "Bon de retour client",
	models.MovementReturnSupplier: "Bon de retour fournisseur",
	models.MovementDefect:         "Constat de défaut",
	models.MovementRepair:         "Remise en vente",
	models.MovementScrap:          "Bon de mise au rebut",
	models.MovementAdjust:         "Constat d'inventaire",
}

// mouvementSignatures nomme les deux parties qui signent, selon le sens du
// mouvement. Faire signer « le client » un constat d'inventaire n'aurait aucun
// sens ; les libellés suivent donc l'opération.
var mouvementSignatures = map[models.MovementType][2]string{
	models.MovementOut:            {"Remis par", "Reçu par le client"},
	models.MovementIn:             {"Livré par le fournisseur", "Reçu par"},
	models.MovementReturnCustomer: {"Rapporté par le client", "Repris par"},
	models.MovementReturnSupplier: {"Renvoyé par", "Repris par le fournisseur"},
	models.MovementDefect:         {"Constaté par", "Visa du responsable"},
	models.MovementRepair:         {"Réparé par", "Visa du responsable"},
	models.MovementScrap:          {"Mis au rebut par", "Visa du responsable"},
	models.MovementAdjust:         {"Compté par", "Visa du responsable"},
}

// MovementNote rend le justificatif d'un mouvement de stock.
func MovementNote(m models.Movement, settings models.Settings) ([]byte, error) {
	d := newDoc(settings)
	p := d.pdf

	titre := mouvementTitres[m.Type]
	if titre == "" {
		titre = "Bon de mouvement"
	}

	d.footer("", "Document interne. À conserver comme justificatif du mouvement de stock.")
	p.AddPage()
	d.header(titre, m.Ref, m.Date, "")

	// --- Tiers et document lié ----------------------------------------------
	y := p.GetY()
	d.setFill(washRGB)
	p.Rect(110, y, 85, 26, "F")
	p.SetXY(112, y+3.5)
	p.SetFont("Helvetica", "B", 7.5)
	d.setColor(mutedRGB)
	p.CellFormat(81, 4, d.text(intituleTiers(m.Type)), "", 2, "L", false, 0, "")
	p.SetFont("Helvetica", "B", 10.5)
	d.setColor(inkRGB)
	p.CellFormat(81, 5.5, d.text(orDefault(m.PartyName, "Non renseigné")), "", 2, "L", false, 0, "")
	if m.DocumentNo != "" {
		p.SetFont("Helvetica", "", 8.5)
		d.setColor(mutedRGB)
		p.CellFormat(81, 4, d.text("Document lié : "+m.DocumentNo), "", 2, "L", false, 0, "")
	}

	// --- Colonne de gauche : qui, quand -------------------------------------
	p.SetXY(15, y+3.5)
	p.SetFont("Helvetica", "B", 7.5)
	d.setColor(mutedRGB)
	p.CellFormat(85, 4, d.text("ÉTABLI PAR"), "", 2, "L", false, 0, "")
	p.SetFont("Helvetica", "B", 10.5)
	d.setColor(inkRGB)
	p.CellFormat(85, 5.5, d.text(orDefault(m.UserName, "Non renseigné")), "", 2, "L", false, 0, "")
	p.SetFont("Helvetica", "", 8.5)
	d.setColor(mutedRGB)
	p.CellFormat(85, 4, d.text("Le "+m.Date.Format("02/01/2006 à 15h04")), "", 2, "L", false, 0, "")

	p.SetY(y + 32)

	// --- Article -------------------------------------------------------------
	largeurs := []float64{34, 86, 24, 36}
	entetes := []string{"RÉFÉRENCE", "DÉSIGNATION", "QUANTITÉ", "STOCK APRÈS"}
	alignements := []string{"L", "L", "C", "C"}

	p.SetFont("Helvetica", "B", 7.5)
	d.setFill(washRGB)
	d.setColor(mutedRGB)
	for i, entete := range entetes {
		p.CellFormat(largeurs[i], 7, d.text(entete), "", 0, alignements[i], true, 0, "")
	}
	p.Ln(-1)

	p.SetFont("Helvetica", "", 10)
	d.setColor(inkRGB)
	d.setDraw(ruleRGB)
	p.SetLineWidth(0.2)
	valeurs := []string{
		m.ProductSKU,
		m.ProductName,
		FormatQuantity(m.Quantity, ""),
		FormatQuantity(m.StockAfter, ""),
	}
	for i, valeur := range valeurs {
		p.CellFormat(largeurs[i], 9, d.text(valeur), "B", 0, alignements[i], false, 0, "")
	}
	p.Ln(-1)
	p.Ln(4)

	// --- Motif ---------------------------------------------------------------
	if trimmed := strings.TrimSpace(m.Reason); trimmed != "" {
		p.SetFont("Helvetica", "B", 7.5)
		d.setColor(mutedRGB)
		p.CellFormat(180, 5, d.text("MOTIF"), "", 1, "L", false, 0, "")
		p.SetFont("Helvetica", "", 9.5)
		d.setColor(inkRGB)
		p.MultiCell(180, 4.6, d.text(trimmed), "", "L", false)
		p.Ln(2)
	}
	if trimmed := strings.TrimSpace(m.Notes); trimmed != "" {
		p.SetFont("Helvetica", "B", 7.5)
		d.setColor(mutedRGB)
		p.CellFormat(180, 5, d.text("NOTES"), "", 1, "L", false, 0, "")
		p.SetFont("Helvetica", "", 9.5)
		d.setColor(inkRGB)
		p.MultiCell(180, 4.6, d.text(trimmed), "", "L", false)
		p.Ln(2)
	}

	// --- Valorisation, seulement si elle est renseignée ---------------------
	if m.UnitCost > 0 {
		p.Ln(2)
		d.setFill(washRGB)
		yv := p.GetY()
		p.Rect(110, yv, 85, 16, "F")
		p.SetXY(114, yv+3)
		p.SetFont("Helvetica", "", 9)
		d.setColor(mutedRGB)
		p.CellFormat(45, 5, d.text("Valeur du mouvement"), "", 0, "L", false, 0, "")
		p.SetFont("Helvetica", "B", 11)
		d.setColor(inkRGB)
		p.CellFormat(32, 5, d.money(int64(m.Quantity)*m.UnitCost), "", 1, "R", false, 0, "")
		p.SetY(yv + 20)
	}

	// --- Signatures ----------------------------------------------------------
	d.signatures(m.Type)

	return output(p)
}

// signatures trace les deux emplacements à signer, en bas de page.
func (d *doc) signatures(t models.MovementType) {
	p := d.pdf
	if p.GetY() < 200 {
		p.SetY(200)
	}
	libelles, ok := mouvementSignatures[t]
	if !ok {
		libelles = [2]string{"Établi par", "Visa du responsable"}
	}

	y := p.GetY()
	for i, libelle := range libelles {
		x := 15 + float64(i)*95
		p.SetXY(x, y)
		p.SetFont("Helvetica", "B", 7.5)
		d.setColor(mutedRGB)
		p.CellFormat(85, 5, d.text(strings.ToUpper(libelle)), "", 2, "L", false, 0, "")
		p.SetFont("Helvetica", "", 7)
		p.CellFormat(85, 4, d.text("Nom, date et signature"), "", 2, "L", false, 0, "")
		// Le cadre laisse la place d'écrire à la main.
		d.setDraw(ruleRGB)
		p.SetLineWidth(0.2)
		p.Rect(x, p.GetY()+1, 85, 22, "D")
	}
	p.SetY(y + 32)
}

func intituleTiers(t models.MovementType) string {
	switch t {
	case models.MovementIn, models.MovementReturnSupplier:
		return "FOURNISSEUR"
	case models.MovementOut, models.MovementReturnCustomer:
		return "CLIENT"
	default:
		return "TIERS CONCERNÉ"
	}
}
