// Package pdfgen produit les documents imprimables : factures, bons d'entrée
// et rapports.
//
// La mise en page est écrite à la main plutôt que rendue depuis du HTML : le
// résultat est identique sur tous les postes, ne dépend d'aucun navigateur et
// pèse quelques kilo-octets. Les polices sont les polices standard PDF
// (Helvetica), encodées en CP1252, les accents français passent sans avoir à
// embarquer de fichier de police.
package pdfgen

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"comptoir/internal/brand"
	"comptoir/internal/models"
)

// Palette du document, alignée sur l'interface : vert profond pour le bandeau,
// gris ardoise pour le texte, filets très clairs.
var (
	brandRGB  = [3]int{14, 59, 52}    // #0E3B34
	inkRGB    = [3]int{20, 23, 26}    // #14171A
	mutedRGB  = [3]int{110, 118, 125} // #6E767D
	ruleRGB   = [3]int{224, 227, 224} // #E0E3E0
	washRGB   = [3]int{244, 246, 244} // #F4F6F4
	accentRGB = [3]int{176, 90, 38}   // #B05A26, cuivre, réservé au net à payer
)

// Document est le contexte de rendu partagé par tous les documents.
type doc struct {
	pdf      *fpdf.Fpdf
	tr       func(string) string
	settings models.Settings
}

func newDoc(settings models.Settings) *doc {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 22)
	return &doc{pdf: pdf, tr: pdf.UnicodeTranslatorFromDescriptor("cp1252"), settings: settings}
}

func (d *doc) setColor(c [3]int)    { d.pdf.SetTextColor(c[0], c[1], c[2]) }
func (d *doc) setFill(c [3]int)     { d.pdf.SetFillColor(c[0], c[1], c[2]) }
func (d *doc) setDraw(c [3]int)     { d.pdf.SetDrawColor(c[0], c[1], c[2]) }
func (d *doc) text(s string) string { return d.tr(s) }

// money formate un montant en unités monétaires lisibles, avec séparateur de
// milliers en espace insécable fine, la convention francophone.
func (d *doc) money(amount int64) string {
	return FormatMoney(amount, d.settings.Decimals) + " " + d.settings.CurrencySymbol
}

// FormatMoney convertit un montant en centièmes vers une chaîne lisible.
// 1500000 avec 0 décimale → « 15 000 ». Exporté car les rapports s'en servent.
func FormatMoney(amount int64, decimals int) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	units := amount / 100
	cents := amount % 100

	s := fmt.Sprintf("%d", units)
	var grouped strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			grouped.WriteRune(' ')
		}
		grouped.WriteRune(r)
	}
	out := grouped.String()
	if decimals > 0 {
		out = fmt.Sprintf("%s,%02d", out, cents)
	}
	if neg {
		out = "-" + out
	}
	return out
}

// ---------------------------------------------------------------------------
// En-tête et pied communs
// ---------------------------------------------------------------------------

// header dessine le bandeau d'identité : un aplat vert plein largeur portant le
// nom de l'entreprise, et à droite le type et le numéro du document.
// logoBox est le côté, en millimètres, du carré réservé au logo de la boutique
// dans le bandeau de marque.
const logoBox = 17.0

// registerLogo décode le logo enregistré dans les paramètres et le confie au
// moteur PDF. Renvoie faux si aucun logo n'est configuré, ou si l'image est
// illisible : un logo abîmé ne doit pas empêcher d'imprimer une facture.
func (d *doc) registerLogo() bool {
	raw := d.settings.LogoDataURL
	virgule := strings.Index(raw, ",")
	if raw == "" || virgule < 0 {
		return false
	}
	entete, charge := raw[:virgule], raw[virgule+1:]
	if !strings.Contains(entete, "base64") {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(charge)
	if err != nil || len(data) == 0 {
		return false
	}
	format := "PNG"
	if strings.Contains(entete, "jpeg") || strings.Contains(entete, "jpg") {
		format = "JPG"
	}
	d.pdf.RegisterImageOptionsReader("logo-boutique",
		fpdf.ImageOptions{ImageType: format}, bytes.NewReader(data))
	if d.pdf.Err() {
		// L'erreur est effacée pour que la suite du document se produise
		// normalement, sans le logo.
		d.pdf.ClearError()
		return false
	}
	return true
}

// drawLogo place le logo dans un carré blanc, à l'échelle et centré. Le fond
// blanc est nécessaire : un logo sombre sur le bandeau vert serait illisible,
// et la plupart des logos de commerçants sont fournis sans transparence.
func (d *doc) drawLogo(x, y float64) {
	info := d.pdf.GetImageInfo("logo-boutique")
	if info == nil || info.Width() <= 0 || info.Height() <= 0 {
		return
	}
	d.setFill([3]int{255, 255, 255})
	d.pdf.RoundedRect(x, y, logoBox, logoBox, 2, "1234", "F")

	marge := 1.5
	dispo := logoBox - 2*marge
	echelle := dispo / info.Width()
	if h := dispo / info.Height(); h < echelle {
		echelle = h
	}
	w, h := info.Width()*echelle, info.Height()*echelle
	d.pdf.ImageOptions("logo-boutique",
		x+(logoBox-w)/2, y+(logoBox-h)/2, w, h, false,
		fpdf.ImageOptions{ImageType: "", ReadDpi: false}, 0, "")
}

func (d *doc) header(title, number string, date time.Time, statusNote string) {
	p := d.pdf
	s := d.settings

	// Bandeau de marque.
	d.setFill(brandRGB)
	p.Rect(0, 0, 210, 30, "F")

	// Le texte se décale si un logo occupe la gauche du bandeau.
	texteX := 15.0
	largeur := 110.0
	if d.registerLogo() {
		d.drawLogo(15, 6.5)
		texteX = 15 + logoBox + 5
		largeur = 110 - logoBox - 5
	}

	p.SetXY(texteX, 9)
	p.SetTextColor(255, 255, 255)
	p.SetFont("Helvetica", "B", 15)
	p.CellFormat(largeur, 7, d.text(orDefault(s.CompanyName, "Ma Société")), "", 2, "L", false, 0, "")

	p.SetFont("Helvetica", "", 8)
	p.SetTextColor(214, 226, 222)
	p.CellFormat(largeur, 4.5, d.text(joinNonEmpty(" · ", s.Address, s.City, s.Country)), "", 2, "L", false, 0, "")
	p.CellFormat(largeur, 4.5, d.text(joinNonEmpty(" · ",
		prefixed("Tél. ", s.Phone), s.Email, prefixed("NIF ", s.TaxID), prefixed("RCCM ", s.RCCM))),
		"", 2, "L", false, 0, "")

	// Bloc titre à droite.
	p.SetXY(130, 9)
	p.SetFont("Helvetica", "B", 20)
	p.SetTextColor(255, 255, 255)
	p.CellFormat(65, 9, d.text(strings.ToUpper(title)), "", 2, "R", false, 0, "")
	p.SetFont("Helvetica", "", 9)
	p.SetTextColor(214, 226, 222)
	p.CellFormat(65, 5, d.text(number+"   ·   "+date.Format("02/01/2006")), "", 2, "R", false, 0, "")

	p.SetY(38)

	if statusNote != "" {
		p.SetFont("Helvetica", "B", 9)
		d.setColor(accentRGB)
		p.CellFormat(180, 6, d.text(statusNote), "", 1, "L", false, 0, "")
		p.Ln(2)
	}
}

// footer dessine le pied de page : conditions, mention légale et pagination.
func (d *doc) footer(terms, note string) {
	p := d.pdf
	p.SetFooterFunc(func() {
		p.SetY(-20)
		d.setDraw(ruleRGB)
		p.SetLineWidth(0.2)
		p.Line(15, p.GetY(), 195, p.GetY())
		p.Ln(2)

		p.SetFont("Helvetica", "", 7)
		d.setColor(mutedRGB)
		if terms != "" {
			p.MultiCell(150, 3.2, d.text(terms), "", "L", false)
		}
		if note != "" {
			p.SetFont("Helvetica", "I", 7)
			p.MultiCell(150, 3.2, d.text(note), "", "L", false)
		}
		// Mention de paternité, discrète et constante : le document appartient
		// au commerçant, la mention nomme l'éditeur du logiciel. Elle est prévue
		// par l'article 3 de la licence, voir internal/brand.
		p.SetXY(15, -11)
		p.SetFont("Helvetica", "", 6)
		d.setColor(mutedRGB)
		p.CellFormat(120, 4, d.text(brand.Notice), "", 0, "L", false, 0, "")

		p.SetXY(160, -14)
		p.SetFont("Helvetica", "", 7)
		p.CellFormat(35, 4, d.text(fmt.Sprintf("Page %d/{nb}", p.PageNo())), "", 0, "R", false, 0, "")
	})
	p.AliasNbPages("{nb}")
}

// ---------------------------------------------------------------------------
// Facture
// ---------------------------------------------------------------------------

// Invoice rend une facture de vente et renvoie le PDF sous forme d'octets.
func Invoice(inv models.Invoice, settings models.Settings) ([]byte, error) {
	d := newDoc(settings)
	p := d.pdf

	title := "Facture"
	statusNote := ""
	switch inv.Status {
	case models.StatusDraft:
		title = "Devis"
	case models.StatusCancelled:
		statusNote = "DOCUMENT ANNULÉ, sans valeur commerciale"
	}

	d.footer(settings.InvoiceTerms, settings.InvoiceFooter)
	p.AddPage()
	d.header(title, inv.Number, inv.Date, statusNote)

	// --- Bloc client -------------------------------------------------------
	y := p.GetY()
	d.setFill(washRGB)
	p.Rect(110, y, 85, 30, "F")

	p.SetXY(112, y+3.5)
	p.SetFont("Helvetica", "B", 7.5)
	d.setColor(mutedRGB)
	p.CellFormat(81, 4, d.text("FACTURÉ À"), "", 2, "L", false, 0, "")

	p.SetFont("Helvetica", "B", 10.5)
	d.setColor(inkRGB)
	p.CellFormat(81, 5.5, d.text(inv.CustomerName), "", 2, "L", false, 0, "")

	p.SetFont("Helvetica", "", 8.5)
	d.setColor(mutedRGB)
	for _, line := range []string{inv.CustomerAddress, inv.CustomerPhone, prefixed("NIF ", inv.CustomerTaxID)} {
		if strings.TrimSpace(line) != "" {
			p.CellFormat(81, 4, d.text(line), "", 2, "L", false, 0, "")
		}
	}

	// Colonne de gauche : conditions de la vente.
	p.SetXY(15, y+3.5)
	p.SetFont("Helvetica", "B", 7.5)
	d.setColor(mutedRGB)
	p.CellFormat(90, 4, d.text("CONDITIONS"), "", 2, "L", false, 0, "")
	p.SetFont("Helvetica", "", 8.5)
	d.setColor(inkRGB)
	p.CellFormat(90, 4.5, d.text("Règlement : "+paymentLabel(inv.PaymentMethod)), "", 2, "L", false, 0, "")
	p.CellFormat(90, 4.5, d.text("Statut : "+statusLabel(inv.Status)), "", 2, "L", false, 0, "")
	if inv.UserName != "" {
		p.CellFormat(90, 4.5, d.text("Établie par : "+inv.UserName), "", 2, "L", false, 0, "")
	}

	p.SetY(y + 36)

	// --- Tableau des lignes ------------------------------------------------
	widths := []float64{78, 16, 28, 20, 38}
	headers := []string{"DÉSIGNATION", "QTÉ", "PRIX UNITAIRE", "REMISE", "MONTANT HT"}
	aligns := []string{"L", "C", "R", "R", "R"}

	d.setFill(brandRGB)
	p.SetTextColor(255, 255, 255)
	p.SetFont("Helvetica", "B", 7.5)
	for i, h := range headers {
		p.CellFormat(widths[i], 7, d.text(h), "", 0, aligns[i], true, 0, "")
	}
	p.Ln(-1)

	p.SetFont("Helvetica", "", 9)
	d.setDraw(ruleRGB)
	p.SetLineWidth(0.15)

	for idx, l := range inv.Lines {
		if p.GetY() > 240 {
			p.AddPage()
			d.setFill(brandRGB)
			p.SetTextColor(255, 255, 255)
			p.SetFont("Helvetica", "B", 7.5)
			for i, h := range headers {
				p.CellFormat(widths[i], 7, d.text(h), "", 0, aligns[i], true, 0, "")
			}
			p.Ln(-1)
			p.SetFont("Helvetica", "", 9)
		}

		fill := idx%2 == 1
		if fill {
			d.setFill(washRGB)
		}
		rowY := p.GetY()

		d.setColor(inkRGB)
		p.CellFormat(widths[0], 6.5, d.text(truncate(l.ProductName, 52)), "", 0, "L", fill, 0, "")
		p.CellFormat(widths[1], 6.5, d.text(fmt.Sprintf("%d", l.Quantity)), "", 0, "C", fill, 0, "")
		p.CellFormat(widths[2], 6.5, d.text(d.money(l.UnitPrice)), "", 0, "R", fill, 0, "")
		if l.Discount > 0 {
			p.CellFormat(widths[3], 6.5, d.text("-"+d.money(l.Discount)), "", 0, "R", fill, 0, "")
		} else {
			d.setColor(mutedRGB)
			p.CellFormat(widths[3], 6.5, d.text("-"), "", 0, "R", fill, 0, "")
			d.setColor(inkRGB)
		}
		p.SetFont("Helvetica", "B", 9)
		p.CellFormat(widths[4], 6.5, d.text(d.money(l.LineHT)), "", 1, "R", fill, 0, "")
		p.SetFont("Helvetica", "", 9)

		// Deuxième ligne discrète : référence et taux de TVA.
		sub := joinNonEmpty("   ·   ", prefixed("Réf. ", l.SKU), taxNote(l.TaxRate))
		if sub != "" {
			p.SetFont("Helvetica", "", 7)
			d.setColor(mutedRGB)
			p.SetX(15)
			p.CellFormat(widths[0], 3.8, d.text(sub), "", 1, "L", fill, 0, "")
			p.SetFont("Helvetica", "", 9)
			d.setColor(inkRGB)
		}
		p.Line(15, p.GetY(), 195, p.GetY())
		_ = rowY
	}

	// --- Totaux ------------------------------------------------------------
	p.Ln(4)
	totalsX := 110.0
	labelW, valueW := 48.0, 37.0

	line := func(label, value string, bold bool, color [3]int) {
		p.SetX(totalsX)
		if bold {
			p.SetFont("Helvetica", "B", 9.5)
		} else {
			p.SetFont("Helvetica", "", 9)
		}
		d.setColor(mutedRGB)
		p.CellFormat(labelW, 5.8, d.text(label), "", 0, "L", false, 0, "")
		d.setColor(color)
		p.CellFormat(valueW, 5.8, d.text(value), "", 1, "R", false, 0, "")
	}

	line("Total hors taxe", d.money(inv.SubtotalHT), false, inkRGB)
	if inv.GlobalDiscount > 0 {
		line("Remise commerciale", "-"+d.money(inv.GlobalDiscount), false, inkRGB)
	}
	if inv.TaxTotal > 0 {
		line(fmt.Sprintf("TVA (%s)", taxRatesSummary(inv.Lines)), d.money(inv.TaxTotal), false, inkRGB)
	}

	// Net à payer : la seule ligne qui porte la couleur d'accent.
	p.Ln(1)
	p.SetX(totalsX)
	d.setFill(washRGB)
	p.Rect(totalsX, p.GetY(), labelW+valueW, 10, "F")
	p.SetFont("Helvetica", "B", 11)
	d.setColor(inkRGB)
	p.CellFormat(labelW, 10, d.text("  NET À PAYER"), "", 0, "L", false, 0, "")
	d.setColor(accentRGB)
	p.SetFont("Helvetica", "B", 13)
	p.CellFormat(valueW, 10, d.text(d.money(inv.Total)), "", 1, "R", false, 0, "")
	p.Ln(1)

	if inv.AmountPaid > 0 || inv.Balance > 0 {
		line("Déjà réglé", d.money(inv.AmountPaid), false, inkRGB)
		if inv.Balance > 0 {
			line("Reste dû", d.money(inv.Balance), true, accentRGB)
		}
	}

	// Montant en toutes lettres : exigé sur beaucoup de factures.
	p.SetY(p.GetY() + 6)
	p.SetX(15)
	p.SetFont("Helvetica", "I", 8)
	d.setColor(mutedRGB)
	p.MultiCell(90, 4,
		d.text("Arrêtée la présente facture à la somme de "+
			AmountInWords(inv.Total, settings.CurrencySymbol, settings.Decimals)+"."), "", "L", false)

	if strings.TrimSpace(inv.Notes) != "" {
		p.Ln(3)
		p.SetX(15)
		p.SetFont("Helvetica", "B", 7.5)
		d.setColor(mutedRGB)
		p.CellFormat(90, 4, d.text("OBSERVATIONS"), "", 1, "L", false, 0, "")
		p.SetX(15)
		p.SetFont("Helvetica", "", 8)
		d.setColor(inkRGB)
		p.MultiCell(90, 3.8, d.text(inv.Notes), "", "L", false)
	}

	// Zone de signature.
	sigY := p.GetY() + 10
	if sigY < 240 {
		sigY = maxF(sigY, 235)
	}
	p.SetXY(130, sigY)
	d.setDraw(ruleRGB)
	p.Line(130, sigY, 195, sigY)
	p.SetXY(130, sigY+1)
	p.SetFont("Helvetica", "", 7.5)
	d.setColor(mutedRGB)
	p.CellFormat(65, 4, d.text("Cachet et signature"), "", 0, "C", false, 0, "")

	return output(p)
}

// ---------------------------------------------------------------------------
// Bon d'entrée
// ---------------------------------------------------------------------------

// PurchaseNote rend un bon d'entrée fournisseur.
func PurchaseNote(pu models.Purchase, settings models.Settings) ([]byte, error) {
	d := newDoc(settings)
	p := d.pdf

	statusNote := ""
	if pu.Status == models.StatusCancelled {
		statusNote = "BON D'ENTRÉE ANNULÉ"
	}
	d.footer("", "Document interne, justificatif de réception de marchandise.")
	p.AddPage()
	d.header("Bon d'entrée", pu.Number, pu.Date, statusNote)

	y := p.GetY()
	d.setFill(washRGB)
	p.Rect(110, y, 85, 26, "F")
	p.SetXY(112, y+3.5)
	p.SetFont("Helvetica", "B", 7.5)
	d.setColor(mutedRGB)
	p.CellFormat(81, 4, d.text("FOURNISSEUR"), "", 2, "L", false, 0, "")
	p.SetFont("Helvetica", "B", 10.5)
	d.setColor(inkRGB)
	p.CellFormat(81, 5.5, d.text(orDefault(pu.SupplierName, "Non renseigné")), "", 2, "L", false, 0, "")
	if pu.Reference != "" {
		p.SetFont("Helvetica", "", 8.5)
		d.setColor(mutedRGB)
		p.CellFormat(81, 4, d.text("Réf. fournisseur : "+pu.Reference), "", 2, "L", false, 0, "")
	}
	p.SetY(y + 32)

	widths := []float64{86, 18, 32, 44}
	headers := []string{"DÉSIGNATION", "QTÉ", "COÛT UNITAIRE", "MONTANT HT"}
	aligns := []string{"L", "C", "R", "R"}

	d.setFill(brandRGB)
	p.SetTextColor(255, 255, 255)
	p.SetFont("Helvetica", "B", 7.5)
	for i, h := range headers {
		p.CellFormat(widths[i], 7, d.text(h), "", 0, aligns[i], true, 0, "")
	}
	p.Ln(-1)

	p.SetFont("Helvetica", "", 9)
	d.setDraw(ruleRGB)
	p.SetLineWidth(0.15)
	for idx, l := range pu.Lines {
		fill := idx%2 == 1
		if fill {
			d.setFill(washRGB)
		}
		d.setColor(inkRGB)
		p.CellFormat(widths[0], 6.5, d.text(truncate(l.ProductName, 58)), "", 0, "L", fill, 0, "")
		p.CellFormat(widths[1], 6.5, d.text(fmt.Sprintf("%d", l.Quantity)), "", 0, "C", fill, 0, "")
		p.CellFormat(widths[2], 6.5, d.text(d.money(l.UnitPrice)), "", 0, "R", fill, 0, "")
		p.SetFont("Helvetica", "B", 9)
		p.CellFormat(widths[3], 6.5, d.text(d.money(l.LineHT)), "", 1, "R", fill, 0, "")
		p.SetFont("Helvetica", "", 9)
		p.Line(15, p.GetY(), 195, p.GetY())
	}

	p.Ln(4)
	totalsX, labelW, valueW := 110.0, 48.0, 37.0
	row := func(label, value string, bold bool) {
		p.SetX(totalsX)
		if bold {
			p.SetFont("Helvetica", "B", 10.5)
		} else {
			p.SetFont("Helvetica", "", 9)
		}
		d.setColor(mutedRGB)
		p.CellFormat(labelW, 6, d.text(label), "", 0, "L", false, 0, "")
		d.setColor(inkRGB)
		p.CellFormat(valueW, 6, d.text(value), "", 1, "R", false, 0, "")
	}
	row("Total hors taxe", d.money(pu.SubtotalHT), false)
	if pu.TaxTotal > 0 {
		row("TVA", d.money(pu.TaxTotal), false)
	}
	if pu.OtherCosts > 0 {
		row("Frais annexes", d.money(pu.OtherCosts), false)
	}
	row("Coût total de réception", d.money(pu.Total), true)

	return output(p)
}

// ---------------------------------------------------------------------------

func output(p *fpdf.Fpdf) ([]byte, error) {
	var buf strings.Builder
	if err := p.Output(&stringWriter{&buf}); err != nil {
		return nil, fmt.Errorf("génération du PDF : %w", err)
	}
	return []byte(buf.String()), nil
}

type stringWriter struct{ sb *strings.Builder }

func (w *stringWriter) Write(b []byte) (int, error) { return w.sb.Write(b) }

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func prefixed(prefix, v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return prefix + v
}

func joinNonEmpty(sep string, parts ...string) string {
	out := []string{}
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return strings.Join(out, sep)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func taxNote(rate float64) string {
	if rate <= 0 {
		return ""
	}
	return fmt.Sprintf("TVA %.0f%%", rate)
}

func taxRatesSummary(lines []models.DocLine) string {
	seen := map[float64]bool{}
	rates := []string{}
	for _, l := range lines {
		if l.TaxRate > 0 && !seen[l.TaxRate] {
			seen[l.TaxRate] = true
			rates = append(rates, fmt.Sprintf("%.0f%%", l.TaxRate))
		}
	}
	if len(rates) == 0 {
		return "0%"
	}
	return strings.Join(rates, " + ")
}

func paymentLabel(m models.PaymentMethod) string {
	switch m {
	case models.PayCash:
		return "Espèces"
	case models.PayMobile:
		return "Mobile money"
	case models.PayTransfer:
		return "Virement bancaire"
	case models.PayCheck:
		return "Chèque"
	case models.PayCredit:
		return "Crédit / à terme"
	default:
		return "Non précisé"
	}
}

func statusLabel(s models.DocStatus) string {
	switch s {
	case models.StatusDraft:
		return "Devis"
	case models.StatusIssued:
		return "Émise, non réglée"
	case models.StatusPartial:
		return "Partiellement réglée"
	case models.StatusPaid:
		return "Réglée"
	case models.StatusCancelled:
		return "Annulée"
	default:
		return string(s)
	}
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
