package services

import (
	"testing"

	"comptoir/internal/models"
)

// Les montants sont en centièmes : 1 000 correspond à 10 unités monétaires.

func TestComputeTotals_LigneSimple(t *testing.T) {
	lines := []models.DocLine{{Quantity: 3, UnitPrice: 10000, TaxRate: 18}}
	out, ht, tax, total := computeTotals(lines, 0, false)

	if ht != 30000 {
		t.Errorf("sous-total HT = %d, attendu 30000", ht)
	}
	if tax != 5400 {
		t.Errorf("taxes = %d, attendu 5400 (18 %% de 30000)", tax)
	}
	if total != 35400 {
		t.Errorf("total = %d, attendu 35400", total)
	}
	if out[0].LineTTC != 35400 {
		t.Errorf("total de ligne = %d, attendu 35400", out[0].LineTTC)
	}
}

func TestComputeTotals_RemiseDeLigne(t *testing.T) {
	lines := []models.DocLine{{Quantity: 2, UnitPrice: 50000, Discount: 10000, TaxRate: 18}}
	out, ht, _, total := computeTotals(lines, 0, false)

	if ht != 90000 {
		t.Errorf("sous-total HT = %d, attendu 90000 (100000 − 10000)", ht)
	}
	if out[0].TaxAmount != 16200 {
		t.Errorf("taxes = %d, attendu 16200 : la taxe porte sur le prix effectivement payé", out[0].TaxAmount)
	}
	if total != 106200 {
		t.Errorf("total = %d, attendu 106200", total)
	}
}

func TestComputeTotals_RemiseSuperieureAuMontant(t *testing.T) {
	// Une remise plus grande que la ligne ne doit jamais produire un montant
	// négatif : elle est plafonnée.
	lines := []models.DocLine{{Quantity: 1, UnitPrice: 5000, Discount: 900000, TaxRate: 18}}
	_, ht, tax, total := computeTotals(lines, 0, false)

	if ht != 0 || tax != 0 || total != 0 {
		t.Errorf("HT=%d taxes=%d total=%d, tout devrait être à zéro", ht, tax, total)
	}
}

func TestComputeTotals_RemiseGlobaleRepartieAuProrata(t *testing.T) {
	lines := []models.DocLine{
		{Quantity: 1, UnitPrice: 70000, TaxRate: 18},
		{Quantity: 1, UnitPrice: 30000, TaxRate: 18},
	}
	out, ht, tax, total := computeTotals(lines, 10000, false)

	if ht != 100000 {
		t.Fatalf("sous-total HT = %d, attendu 100000", ht)
	}
	// La remise de 10 000 réduit la base taxable : 90 000 × 18 % = 16 200.
	if tax != 16200 {
		t.Errorf("taxes = %d, attendu 16200", tax)
	}
	if total != 106200 {
		t.Errorf("total = %d, attendu 106200 (100000 − 10000 + 16200)", total)
	}
	// 70 % de la remise sur la première ligne, 30 % sur la seconde.
	if out[0].TaxAmount != 11340 {
		t.Errorf("taxe ligne 1 = %d, attendu 11340 (63000 × 18 %%)", out[0].TaxAmount)
	}
	if out[1].TaxAmount != 4860 {
		t.Errorf("taxe ligne 2 = %d, attendu 4860 (27000 × 18 %%)", out[1].TaxAmount)
	}
}

func TestComputeTotals_ReliquatDArrondiSurLaDerniereLigne(t *testing.T) {
	// Trois lignes égales et une remise indivisible par trois : la somme des
	// remises de ligne doit valoir exactement la remise globale, sans perte.
	lines := []models.DocLine{
		{Quantity: 1, UnitPrice: 10000, TaxRate: 0},
		{Quantity: 1, UnitPrice: 10000, TaxRate: 0},
		{Quantity: 1, UnitPrice: 10000, TaxRate: 0},
	}
	out, ht, _, total := computeTotals(lines, 100, false)

	var bases int64
	for _, l := range out {
		bases += l.LineTTC
	}
	if bases != ht-100 {
		t.Errorf("somme des lignes = %d, attendu %d : le reliquat d'arrondi s'est perdu", bases, ht-100)
	}
	if total != 29900 {
		t.Errorf("total = %d, attendu 29900", total)
	}
}

func TestComputeTotals_PrixSaisisTTC(t *testing.T) {
	// 11 800 TTC à 18 % correspondent à 10 000 HT.
	lines := []models.DocLine{{Quantity: 1, UnitPrice: 1180000, TaxRate: 18}}
	_, ht, tax, total := computeTotals(lines, 0, true)

	if ht != 1000000 {
		t.Errorf("base HT extraite = %d, attendu 1000000", ht)
	}
	if tax != 180000 {
		t.Errorf("taxes = %d, attendu 180000", tax)
	}
	if total != 1180000 {
		t.Errorf("total = %d, attendu 1180000 : le prix affiché doit être retrouvé à l'unité près", total)
	}
}

func TestWeightedAverageCost(t *testing.T) {
	cases := []struct {
		nom              string
		oldQty           int
		oldCost          int64
		newQty           int
		newCost, attendu int64
	}{
		{"stock vide : le coût est celui de l'entrée", 0, 0, 10, 50000, 50000},
		{"stock négatif ignoré", -5, 30000, 10, 50000, 50000},
		{"entrée nulle : le coût ne bouge pas", 10, 30000, 0, 50000, 30000},
		{"moyenne simple", 10, 30000, 10, 50000, 40000},
		{"moyenne pondérée", 30, 20000, 10, 60000, 30000},
		{"arrondi au plus proche", 3, 10000, 1, 10001, 10000},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			got := weightedAverageCost(c.oldQty, c.oldCost, c.newQty, c.newCost)
			if got != c.attendu {
				t.Errorf("coût moyen = %d, attendu %d", got, c.attendu)
			}
		})
	}
}

func TestShare_TotalNul(t *testing.T) {
	if got := share(1000, 50, 0); got != 0 {
		t.Errorf("share avec un total nul = %d, attendu 0 (et surtout pas une division par zéro)", got)
	}
}

func TestPct_ArrondiCommercial(t *testing.T) {
	if got := pct(1005, 50); got != 503 {
		t.Errorf("pct(1005, 50) = %d, attendu 503 : l'arrondi doit aller au plus proche, pas vers le bas", got)
	}
}
