// Package util regroupe des aides transverses : identifiants, dates, texte.
package util

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

var encoder = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// NewID génère un identifiant unique, trié chronologiquement et sans caractère
// ambigu (ni I, L, O, U). Format : <préfixe>_<48 bits de temps><40 bits aléatoires>.
// Trié chronologiquement : deux enregistrements créés dans l'ordre gardent cet
// ordre en comparaison de chaînes, ce qui rend les listes stables sans index.
func NewID(prefix string) string {
	var b [11]byte
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint64(b[:8], ms<<16)
	if _, err := rand.Read(b[6:]); err != nil {
		// crypto/rand ne peut échouer que si le système est en très mauvais
		// état ; on dégrade sur l'horloge nanoseconde plutôt que de paniquer.
		binary.BigEndian.PutUint64(b[3:], uint64(time.Now().UnixNano()))
	}
	return prefix + "_" + encoder.EncodeToString(b[:])
}

// TempPassword génère un mot de passe provisoire lisible à l'oral : dix
// caractères de l'alphabet sans ambiguïté (ni I, L, O, U, ni 0/O confondus),
// dont au moins une lettre et un chiffre — la politique de auth.ValidatePassword
// est donc toujours respectée.
func TempPassword() (string, error) {
	const letters = "ABCDEFGHJKMNPQRSTVWXYZ"
	const digits = "23456789"
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("génération du mot de passe provisoire : %w", err)
	}
	out := make([]byte, 10)
	for i, v := range b {
		// Les deux premières positions fixent la présence d'une lettre et d'un
		// chiffre ; le reste est tiré dans l'alphabet complet.
		switch i {
		case 0:
			out[i] = letters[int(v)%len(letters)]
		case 1:
			out[i] = digits[int(v)%len(digits)]
		default:
			all := letters + digits
			out[i] = all[int(v)%len(all)]
		}
	}
	// Le chiffre imposé en position 1 rendrait tous les mots de passe
	// reconnaissables : on mélange avec un tirage indépendant.
	var shuffle [10]byte
	if _, err := rand.Read(shuffle[:]); err != nil {
		return "", fmt.Errorf("génération du mot de passe provisoire : %w", err)
	}
	for i := len(out) - 1; i > 0; i-- {
		j := int(shuffle[i]) % (i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}

// Slug normalise une chaîne pour la comparaison et la recherche : minuscules,
// accents retirés, espaces réduits.
func Slug(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		r = foldAccent(r)
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		default:
			// Ponctuation, espaces et symboles deviennent des séparateurs :
			// « HP-LaserJet/M404 » et « hp laserjet m404 » se rejoignent.
			sb.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// accentFolds mappe les caractères accentués français (et quelques voisins)
// vers leur équivalent ASCII, pour que « imprimante laser » trouve
// « Imprimante Laser » et « écran » trouve « Ecran ».
var accentFolds = map[rune]rune{
	'à': 'a', 'â': 'a', 'ä': 'a', 'á': 'a', 'ã': 'a', 'å': 'a',
	'ç': 'c',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ñ': 'n',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ý': 'y', 'ÿ': 'y',
}

func foldAccent(r rune) rune {
	if folded, ok := accentFolds[r]; ok {
		return folded
	}
	return r
}

// Contains effectue une recherche insensible à la casse et aux accents.
func Contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(Slug(haystack), Slug(needle))
}

// ---------------------------------------------------------------------------
// Périodes
// ---------------------------------------------------------------------------

// StartOfDay renvoie le premier instant du jour de t, dans son fuseau.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// EndOfDay renvoie le dernier instant du jour de t.
func EndOfDay(t time.Time) time.Time {
	return StartOfDay(t).AddDate(0, 0, 1).Add(-time.Nanosecond)
}

// StartOfMonth renvoie le premier instant du mois de t.
func StartOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

// StartOfQuarter renvoie le premier instant du trimestre civil de t.
func StartOfQuarter(t time.Time) time.Time {
	q := (int(t.Month()) - 1) / 3
	return time.Date(t.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, t.Location())
}

// StartOfSemester renvoie le premier instant du semestre civil de t.
func StartOfSemester(t time.Time) time.Time {
	m := time.January
	if int(t.Month()) > 6 {
		m = time.July
	}
	return time.Date(t.Year(), m, 1, 0, 0, 0, 0, t.Location())
}

// StartOfYear renvoie le premier instant de l'année de t.
func StartOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location())
}

// InRange indique si t appartient à l'intervalle [from, to], bornes incluses.
func InRange(t, from, to time.Time) bool {
	return !t.Before(from) && !t.After(to)
}

// MonthNamesFR sert aux libellés de rapports.
var MonthNamesFR = [...]string{
	"janvier", "février", "mars", "avril", "mai", "juin",
	"juillet", "août", "septembre", "octobre", "novembre", "décembre",
}

// MonthLabel renvoie « mars 2026 ».
func MonthLabel(t time.Time) string {
	return fmt.Sprintf("%s %d", MonthNamesFR[int(t.Month())-1], t.Year())
}
