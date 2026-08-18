package pdfgen

import (
	"fmt"
	"strings"
)

// AmountInWords convertit un montant en centièmes vers sa formulation
// française, telle qu'attendue sur une facture :
// « deux cent quinze mille trois cents francs CFA ».
//
// L'orthographe suit la règle classique (rectifications de 1990 non
// appliquées) : « vingt » et « cent » prennent un s au pluriel s'ils terminent
// le nombre, « mille » est invariable, les dizaines composées sont reliées par
// un trait d'union sauf « et un » / « et onze ».
func AmountInWords(amount int64, currency string, decimals int) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}
	units := amount / 100
	cents := amount % 100

	out := integerToFrench(units) + " " + currencyWords(currency, units)
	if decimals > 0 && cents > 0 {
		out += " et " + integerToFrench(cents) + " centime"
		if cents > 1 {
			out += "s"
		}
	}
	if negative {
		out = "moins " + out
	}
	// La majuscule est posée une seule fois, à la fin : la poser plus tôt
	// laisserait « moins Cinq mille francs CFA ».
	return capitalize(strings.TrimSpace(out))
}

func currencyWords(symbol string, amount int64) string {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "FCFA", "CFA", "XOF", "XAF":
		// « zéro franc », « un franc », « deux francs » : l'accord se fait au
		// pluriel à partir de deux.
		if amount > 1 {
			return "francs CFA"
		}
		return "franc CFA"
	case "€", "EUR":
		if amount > 1 {
			return "euros"
		}
		return "euro"
	case "$", "USD":
		if amount > 1 {
			return "dollars"
		}
		return "dollar"
	case "":
		return ""
	default:
		return symbol
	}
}

var smallNumbers = [...]string{
	"zéro", "un", "deux", "trois", "quatre", "cinq", "six", "sept", "huit", "neuf",
	"dix", "onze", "douze", "treize", "quatorze", "quinze", "seize",
	"dix-sept", "dix-huit", "dix-neuf",
}

var tens = [...]string{
	"", "", "vingt", "trente", "quarante", "cinquante", "soixante",
	"soixante", "quatre-vingt", "quatre-vingt",
}

// integerToFrench écrit un entier positif en toutes lettres.
func integerToFrench(n int64) string {
	if n == 0 {
		return "zéro"
	}
	if n < 0 {
		return "moins " + integerToFrench(-n)
	}

	scales := []struct {
		value int64
		one   string
		many  string
	}{
		{1_000_000_000_000, "mille milliards", "mille milliards"},
		{1_000_000_000, "un milliard", "milliards"},
		{1_000_000, "un million", "millions"},
		{1_000, "mille", "mille"},
	}

	for _, sc := range scales {
		if n >= sc.value {
			count := n / sc.value
			rest := n % sc.value

			var part string
			switch {
			case sc.value == 1000 && count == 1:
				part = "mille" // « mille », jamais « un mille »
			case sc.value == 1000:
				part = integerToFrench(count) + " mille"
			case count == 1:
				part = sc.one
			default:
				part = integerToFrench(count) + " " + sc.many
			}
			if rest == 0 {
				return part
			}
			return part + " " + integerToFrench(rest)
		}
	}
	return below1000(n)
}

func below1000(n int64) string {
	if n >= 100 {
		hundreds := n / 100
		rest := n % 100
		var part string
		if hundreds == 1 {
			part = "cent"
		} else {
			part = smallNumbers[hundreds] + " cent"
		}
		if rest == 0 {
			// « cent » prend un s quand il termine le nombre et est multiplié.
			if hundreds > 1 {
				part += "s"
			}
			return part
		}
		return part + " " + below100(rest)
	}
	return below100(n)
}

func below100(n int64) string {
	if n < 20 {
		return smallNumbers[n]
	}
	ten := n / 10
	unit := n % 10

	// 70–79 et 90–99 se construisent sur soixante / quatre-vingt + 10..19.
	if ten == 7 || ten == 9 {
		base := tens[ten]
		remainder := smallNumbers[10+unit]
		if ten == 7 && unit == 1 {
			return base + " et onze"
		}
		return base + "-" + remainder
	}

	base := tens[ten]
	switch {
	case unit == 0:
		if ten == 8 {
			return base + "s" // quatre-vingts
		}
		return base
	case unit == 1 && ten != 8:
		return base + " et un"
	default:
		return base + "-" + smallNumbers[unit]
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// FormatQuantity formate une quantité avec son unité, au singulier ou pluriel.
func FormatQuantity(qty int, unit string) string {
	if unit == "" {
		unit = "pièce"
	}
	if qty > 1 && !strings.HasSuffix(unit, "s") {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", qty, unit)
}
