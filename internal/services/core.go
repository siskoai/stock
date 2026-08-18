// Package services contient la logique métier de Comptoir. Chaque service est
// exposé tel quel à l'interface par Wails ; les méthodes exportées sont donc
// l'API publique de l'application.
//
// Règles communes à tous les services :
//   - toute méthode commence par un contrôle de session et de rôle (core.guard) ;
//   - toute écriture est journalisée dans le registre d'audit ;
//   - les montants reçus de l'interface sont recalculés côté Go, jamais
//     acceptés tels quels ;
//   - les messages d'erreur sont rédigés en français, à destination de
//     l'utilisateur final.
package services

import (
	"fmt"
	"math"
	"strings"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// core rassemble les dépendances partagées. Ses méthodes sont volontairement
// non exportées : Wails ne lie que les méthodes exportées, ce qui garde les
// utilitaires internes hors de l'API accessible depuis l'interface.
type core struct {
	db   *storage.Database
	auth *auth.Service
}

// guard vérifie la session et le rôle, puis renvoie l'utilisateur agissant.
func (c *core) guard(scope string) (models.User, error) {
	return c.auth.Require(scope)
}

// canSeeCost indique si le rôle a accès aux prix d'achat, aux marges et à la
// valorisation du stock. Un vendeur ne les voit pas.
func (c *core) canSeeCost(u models.User) bool { return auth.Can(u.Role, "finance") }

// log ajoute une entrée au journal d'audit. Une erreur d'écriture du journal ne
// fait pas échouer l'opération métier : le journal est une trace, pas une
// condition de validité.
func (c *core) log(u models.User, action, entity, entityID, format string, args ...any) {
	_ = c.db.Audit.Insert(models.AuditEntry{
		ID:       util.NewID("aud"),
		At:       time.Now(),
		UserID:   u.ID,
		UserName: u.FullName,
		Action:   action,
		Entity:   entity,
		EntityID: entityID,
		Details:  fmt.Sprintf(format, args...),
	})
}

// ---------------------------------------------------------------------------
// Calculs monétaires
//
// Les montants sont des entiers en centièmes d'unité. Les divisions sont
// arrondies au plus proche (arrondi commercial), jamais tronquées, pour que la
// somme des lignes corresponde toujours au total affiché.
// ---------------------------------------------------------------------------

// pct applique un pourcentage à un montant entier, arrondi au plus proche.
func pct(amount int64, rate float64) int64 {
	return int64(math.Round(float64(amount) * rate / 100))
}

// share répartit un montant au prorata d'un poids, arrondi au plus proche.
func share(amount int64, weight, total int64) int64 {
	if total == 0 {
		return 0
	}
	return int64(math.Round(float64(amount) * float64(weight) / float64(total)))
}

// computeTotals recalcule intégralement les lignes et les totaux d'un document.
// pricesIncludeTax indique si les prix unitaires saisis sont TTC.
//
// La remise globale réduit la base taxable au prorata de chaque ligne, ce qui
// est le traitement correct : la TVA porte sur le prix effectivement payé.
func computeTotals(lines []models.DocLine, globalDiscount int64, pricesIncludeTax bool) (
	out []models.DocLine, subtotalHT, taxTotal, total int64) {

	out = make([]models.DocLine, len(lines))
	copy(out, lines)

	for i := range out {
		l := &out[i]
		if l.Quantity < 0 {
			l.Quantity = 0
		}
		if l.Discount < 0 {
			l.Discount = 0
		}
		gross := int64(l.Quantity) * l.UnitPrice
		if l.Discount > gross {
			l.Discount = gross
		}
		net := gross - l.Discount

		if pricesIncludeTax {
			// Prix saisis TTC : on extrait la base hors taxe.
			l.LineHT = int64(math.Round(float64(net) / (1 + l.TaxRate/100)))
		} else {
			l.LineHT = net
		}
		subtotalHT += l.LineHT
	}

	if globalDiscount < 0 {
		globalDiscount = 0
	}
	if globalDiscount > subtotalHT {
		globalDiscount = subtotalHT
	}

	// Base taxable après remise globale, répartie au prorata.
	remaining := globalDiscount
	for i := range out {
		l := &out[i]
		var cut int64
		if i == len(out)-1 {
			cut = remaining // le reliquat d'arrondi va sur la dernière ligne
		} else {
			cut = share(globalDiscount, l.LineHT, subtotalHT)
			if cut > remaining {
				cut = remaining
			}
			remaining -= cut
		}
		base := l.LineHT - cut
		if base < 0 {
			base = 0
		}
		l.TaxAmount = pct(base, l.TaxRate)
		l.LineTTC = base + l.TaxAmount
		taxTotal += l.TaxAmount
	}

	total = subtotalHT - globalDiscount + taxTotal
	return out, subtotalHT, taxTotal, total
}

// ---------------------------------------------------------------------------
// Aides diverses
// ---------------------------------------------------------------------------

func trim(s string) string { return strings.TrimSpace(s) }

// requireText valide un champ obligatoire et renvoie sa version nettoyée.
func requireText(value, field string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("le champ « %s » est obligatoire", field)
	}
	return v, nil
}

// Period décrit un intervalle de dates utilisé par les rapports.
type Period struct {
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	Label string    `json:"label"`
}

// parseRange convertit deux dates ISO (AAAA-MM-JJ) envoyées par l'interface en
// intervalle inclusif. Des chaînes vides valent « depuis toujours ».
func parseRange(from, to string) (time.Time, time.Time, error) {
	start := time.Time{}
	end := util.EndOfDay(time.Now())

	if trim(from) != "" {
		t, err := time.ParseInLocation("2006-01-02", trim(from), time.Local)
		if err != nil {
			return start, end, fmt.Errorf("date de début invalide : %s", from)
		}
		start = util.StartOfDay(t)
	}
	if trim(to) != "" {
		t, err := time.ParseInLocation("2006-01-02", trim(to), time.Local)
		if err != nil {
			return start, end, fmt.Errorf("date de fin invalide : %s", to)
		}
		end = util.EndOfDay(t)
	}
	if end.Before(start) {
		return start, end, fmt.Errorf("la date de fin précède la date de début")
	}
	return start, end, nil
}

// parseDate lit une date ISO ; une chaîne vide vaut « maintenant ».
func parseDate(s string) (time.Time, error) {
	if trim(s) == "" {
		return time.Now(), nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05Z07:00", trim(s), time.Local); err == nil {
		return t, nil
	}
	t, err := time.ParseInLocation("2006-01-02", trim(s), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("date invalide : %s", s)
	}
	// On conserve l'heure courante pour que l'ordre de saisie du jour soit
	// respecté dans les journaux.
	now := time.Now()
	return time.Date(t.Year(), t.Month(), t.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.Local), nil
}
