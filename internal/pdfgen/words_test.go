package pdfgen

import (
	"strings"
	"testing"
)

func TestIntegerToFrench(t *testing.T) {
	cases := map[int64]string{
		0:          "zéro",
		1:          "un",
		16:         "seize",
		17:         "dix-sept",
		20:         "vingt",
		21:         "vingt et un",
		31:         "trente et un",
		71:         "soixante et onze",
		75:         "soixante-quinze",
		80:         "quatre-vingts",
		81:         "quatre-vingt-un",
		91:         "quatre-vingt-onze",
		99:         "quatre-vingt-dix-neuf",
		100:        "cent",
		101:        "cent un",
		180:        "cent quatre-vingts",
		200:        "deux cents",
		215:        "deux cent quinze",
		1000:       "mille",
		1001:       "mille un",
		2000:       "deux mille",
		215300:     "deux cent quinze mille trois cents",
		1000000:    "un million",
		2000001:    "deux millions un",
		1000000000: "un milliard",
	}
	for n, attendu := range cases {
		if got := integerToFrench(n); got != attendu {
			t.Errorf("integerToFrench(%d) = %q, attendu %q", n, got, attendu)
		}
	}
}

func TestAmountInWords(t *testing.T) {
	cases := []struct {
		amount   int64
		devise   string
		decimals int
		attendu  string
	}{
		{21530000, "FCFA", 0, "Deux cent quinze mille trois cents francs CFA"},
		{100, "FCFA", 0, "Un franc CFA"},
		{0, "FCFA", 0, "Zéro franc CFA"},
		{150050, "EUR", 2, "Mille cinq cents euros et cinquante centimes"},
		{100001, "EUR", 2, "Mille euros et un centime"},
		{-500000, "FCFA", 0, "Moins cinq mille francs CFA"},
	}
	for _, c := range cases {
		if got := AmountInWords(c.amount, c.devise, c.decimals); got != c.attendu {
			t.Errorf("AmountInWords(%d, %q, %d) = %q, attendu %q",
				c.amount, c.devise, c.decimals, got, c.attendu)
		}
	}
}

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		amount   int64
		decimals int
		attendu  string
	}{
		{1500000, 0, "15 000"},
		{100, 0, "1"},
		{0, 0, "0"},
		{123456789, 0, "1 234 567"},
		{150050, 2, "1 500,50"},
		{-1500000, 0, "-15 000"},
	}
	for _, c := range cases {
		if got := FormatMoney(c.amount, c.decimals); got != c.attendu {
			t.Errorf("FormatMoney(%d, %d) = %q, attendu %q", c.amount, c.decimals, got, c.attendu)
		}
	}
}

func TestFormatQuantity(t *testing.T) {
	cases := []struct {
		qty     int
		unit    string
		attendu string
	}{
		{1, "pièce", "1 pièce"},
		{3, "pièce", "3 pièces"},
		{2, "colis", "2 colis"},
		{5, "", "5 pièces"},
	}
	for _, c := range cases {
		if got := FormatQuantity(c.qty, c.unit); got != c.attendu {
			t.Errorf("FormatQuantity(%d, %q) = %q, attendu %q", c.qty, c.unit, got, c.attendu)
		}
	}
}

// Le logo du commerçant doit apparaître sur ses documents, et un logo abîmé ne
// doit jamais empêcher d'imprimer une facture.
func TestLogoDeLaBoutique(t *testing.T) {
	// PNG 1x1 valide, encodé en base64.
	const pixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

	cas := []struct {
		nom         string
		dataURL     string
		attendu     bool
		commentaire string
	}{
		{"logo valide", "data:image/png;base64," + pixel, true, "doit être dessiné"},
		{"aucun logo", "", false, "en-tête sans logo"},
		{"base64 illisible", "data:image/png;base64,pas-du-base64", false, "ignoré sans planter"},
		{"data URL tronquée", "data:image/png;base64", false, "ignoré sans planter"},
		{"encodage non géré", "data:image/png,%89PNG", false, "ignoré sans planter"},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			settings := reglagesAvecLogo(c.dataURL)
			d := newDoc(settings)
			if got := d.registerLogo(); got != c.attendu {
				t.Errorf("registerLogo() = %v, attendu %v (%s)", got, c.attendu, c.commentaire)
			}
			// Dans tous les cas, le document doit se produire.
			data, err := Invoice(factureMinimale(), settings)
			if err != nil {
				t.Fatalf("génération de la facture : %v", err)
			}
			if len(data) < 1000 || string(data[:4]) != "%PDF" {
				t.Errorf("document invalide (%d octets)", len(data))
			}
		})
	}
}

// Un logo présent doit alourdir le document : c'est la preuve qu'il y est.
func TestLogoPresentDansLeDocument(t *testing.T) {
	const pixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

	sans, err := Invoice(factureMinimale(), reglagesAvecLogo(""))
	if err != nil {
		t.Fatal(err)
	}
	avec, err := Invoice(factureMinimale(), reglagesAvecLogo("data:image/png;base64,"+pixel))
	if err != nil {
		t.Fatal(err)
	}
	if len(avec) <= len(sans) {
		t.Errorf("document avec logo = %d octets, sans logo = %d : l'image n'a pas été incluse",
			len(avec), len(sans))
	}
}

// La désignation est la colonne qu'on lit : elle ne doit jamais être réduite à
// quelques lettres parce que les colonnes chiffrées ont pris toute la place.
func TestResolveWidths_PlancherDesColonnesLibres(t *testing.T) {
	cas := []struct {
		nom      string
		colonnes []ReportColumn
		libreIdx int
	}{
		{
			"colonnes imposées gourmandes",
			[]ReportColumn{
				{Header: "Référence", Width: 30}, {Header: "Désignation"},
				{Header: "Catégorie", Width: 34}, {Header: "Stock", Width: 16},
				{Header: "Déf.", Width: 13}, {Header: "PV", Width: 24},
				{Header: "Coût", Width: 24}, {Header: "Valeur", Width: 26},
			}, 1,
		},
		{
			"imposées au-delà de la page",
			[]ReportColumn{
				{Header: "A", Width: 90}, {Header: "Désignation"},
				{Header: "B", Width: 90}, {Header: "C", Width: 40},
			}, 1,
		},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			w := resolveWidths(c.colonnes)
			if w[c.libreIdx] < largeurLibreMin {
				t.Errorf("colonne libre = %.1f mm, plancher attendu %.0f mm", w[c.libreIdx], largeurLibreMin)
			}
			var total float64
			for _, x := range w {
				total += x
			}
			if total < 179 || total > 181 {
				t.Errorf("largeur totale = %.1f mm, attendu 180", total)
			}
		})
	}
}

func TestResolveWidths_ToutImpose(t *testing.T) {
	w := resolveWidths([]ReportColumn{{Width: 100}, {Width: 100}})
	var total float64
	for _, x := range w {
		total += x
	}
	if total < 179 || total > 181 {
		t.Errorf("des colonnes toutes imposées doivent être ramenées à 180 mm, obtenu %.1f", total)
	}
}

// La troncature mesure le texte : une chaîne étroite qui tient ne doit pas
// être coupée, une chaîne large qui déborde doit l'être.
func TestAjuster(t *testing.T) {
	d := newDoc(reglagesAvecLogo(""))
	d.pdf.AddPage()
	d.pdf.SetFont("Helvetica", "", 8.5)

	if got := d.ajuster("Clavier", 50); got != "Clavier" {
		t.Errorf("un texte court a été coupé : %q", got)
	}
	long := "Ordinateur portable HP 250 G9 avec sacoche et souris"
	court := d.ajuster(long, 40)
	if court == long {
		t.Error("un texte trop long n'a pas été raccourci")
	}
	if !strings.HasSuffix(court, "…") {
		t.Errorf("la coupure ne se signale pas : %q", court)
	}
	if d.pdf.GetStringWidth(d.text(court)) > 40 {
		t.Errorf("le texte raccourci déborde encore : %q", court)
	}
	// Les caractères larges tiennent moins que les étroits, à nombre égal.
	etroit := d.ajuster(strings.Repeat("i", 60), 30)
	large := d.ajuster(strings.Repeat("W", 60), 30)
	if len([]rune(etroit)) <= len([]rune(large)) {
		t.Errorf("la mesure ignore la largeur des lettres : %d i contre %d W",
			len([]rune(etroit)), len([]rune(large)))
	}
}
