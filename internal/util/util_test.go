package util

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"  Imprimante   LASER  ": "imprimante laser",
		"Écran 24\" Full-HD":     "ecran 24 full hd",
		"HP-LaserJet/M404":       "hp laserjet m404",
		"Café & Thé":             "cafe the",
		"":                       "",
	}
	for in, attendu := range cases {
		if got := Slug(in); got != attendu {
			t.Errorf("Slug(%q) = %q, attendu %q", in, got, attendu)
		}
	}
}

func TestContains_InsensibleALaCasseEtAuxAccents(t *testing.T) {
	if !Contains("Imprimante Écran", "ecran") {
		t.Error("« ecran » devrait trouver « Écran »")
	}
	if !Contains("HP LaserJet M404", "laserjet m404") {
		t.Error("la recherche multi-mots devrait aboutir")
	}
	if !Contains("n'importe quoi", "") {
		t.Error("une recherche vide doit tout retenir")
	}
	if Contains("Clavier", "souris") {
		t.Error("faux positif")
	}
}

func TestNewID_UniqueEtTriableChronologiquement(t *testing.T) {
	const n = 2000
	ids := make([]string, n)
	seen := make(map[string]bool, n)
	for i := range ids {
		ids[i] = NewID("prd")
		if seen[ids[i]] {
			t.Fatalf("identifiant en double : %s", ids[i])
		}
		seen[ids[i]] = true
		if !strings.HasPrefix(ids[i], "prd_") {
			t.Fatalf("préfixe manquant : %s", ids[i])
		}
	}
	// Deux identifiants créés dans l'ordre gardent cet ordre en comparaison de
	// chaînes : c'est ce qui rend les listes stables sans index.
	before := NewID("prd")
	time.Sleep(2 * time.Millisecond)
	after := NewID("prd")
	if before >= after {
		t.Errorf("l'ordre chronologique n'est pas respecté : %s >= %s", before, after)
	}
	if !sort.StringsAreSorted([]string{before, after}) {
		t.Error("les identifiants ne se trient pas dans l'ordre de création")
	}
}

func TestTempPassword_RespecteLaPolitique(t *testing.T) {
	for i := 0; i < 200; i++ {
		pwd, err := TempPassword()
		if err != nil {
			t.Fatalf("génération : %v", err)
		}
		if len(pwd) != 10 {
			t.Fatalf("longueur = %d, attendu 10 : %q", len(pwd), pwd)
		}
		var lettre, chiffre bool
		for _, r := range pwd {
			switch {
			case r >= '0' && r <= '9':
				chiffre = true
			case r >= 'A' && r <= 'Z':
				lettre = true
			default:
				t.Fatalf("caractère inattendu %q dans %q", r, pwd)
			}
		}
		if !lettre || !chiffre {
			t.Fatalf("%q ne satisfait pas la politique (lettre et chiffre obligatoires)", pwd)
		}
		if strings.ContainsAny(pwd, "ILOU01") {
			t.Fatalf("%q contient un caractère ambigu à l'oral", pwd)
		}
	}
}

func TestPeriodes(t *testing.T) {
	ref := time.Date(2026, time.August, 18, 14, 30, 0, 0, time.Local)

	if got := StartOfDay(ref); got.Hour() != 0 || got.Day() != 18 {
		t.Errorf("StartOfDay = %v", got)
	}
	if got := EndOfDay(ref); got.Day() != 18 || got.Hour() != 23 {
		t.Errorf("EndOfDay = %v", got)
	}
	if got := StartOfMonth(ref); got.Day() != 1 || got.Month() != time.August {
		t.Errorf("StartOfMonth = %v", got)
	}
	if got := StartOfQuarter(ref); got.Month() != time.July {
		t.Errorf("StartOfQuarter = %v, attendu juillet", got)
	}
	if got := StartOfSemester(ref); got.Month() != time.July {
		t.Errorf("StartOfSemester = %v, attendu juillet", got)
	}
	if got := StartOfYear(ref); got.Month() != time.January || got.Year() != 2026 {
		t.Errorf("StartOfYear = %v", got)
	}
	if MonthLabel(ref) != "août 2026" {
		t.Errorf("MonthLabel = %q, attendu « août 2026 »", MonthLabel(ref))
	}
}

func TestInRange_BornesIncluses(t *testing.T) {
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, time.March, 31, 23, 59, 59, 0, time.Local)

	if !InRange(from, from, to) || !InRange(to, from, to) {
		t.Error("les bornes doivent être incluses")
	}
	if InRange(from.AddDate(0, 0, -1), from, to) {
		t.Error("une date antérieure ne doit pas être retenue")
	}
}
