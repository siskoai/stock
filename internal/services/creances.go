package services

import (
	"fmt"
	"sort"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Créances : ce que les clients doivent encore.
//
// Une vente à crédit n'est pas un mode de règlement de plus, c'est une dette
// qu'il faut suivre. Le tableau de bord dit combien on attend ; cet écran dit de
// qui, depuis quand, et lesquelles sont en retard. C'est le classement par
// ancienneté qui compte : mille francs dus depuis trois mois ne valent pas mille
// francs dus depuis trois jours.

// Creances est le service de suivi des impayés.
type Creances struct{ core }

// NewCreances construit le service des créances.
func NewCreances(db *storage.Database, a *auth.Service) *Creances {
	return &Creances{core{db: db, auth: a}}
}

// Tranche classe une créance selon son ancienneté.
type Tranche string

const (
	TrancheNonEchue  Tranche = "NON_ECHUE" // l'échéance n'est pas encore passée
	Tranche1a30      Tranche = "J1_30"
	Tranche31a60     Tranche = "J31_60"
	Tranche61a90     Tranche = "J61_90"
	TrancheAuDela    Tranche = "J90_PLUS"
	TrancheSansTerme Tranche = "SANS_TERME" // aucune échéance convenue
)

// LibelleTranche nomme une tranche pour l'affichage et les documents.
func LibelleTranche(t Tranche) string {
	switch t {
	case TrancheNonEchue:
		return "Non échu"
	case Tranche1a30:
		return "1 à 30 jours"
	case Tranche31a60:
		return "31 à 60 jours"
	case Tranche61a90:
		return "61 à 90 jours"
	case TrancheAuDela:
		return "Plus de 90 jours"
	default:
		return "Sans échéance"
	}
}

// ordreTranches fixe l'ordre d'affichage, du plus sain au plus inquiétant.
var ordreTranches = []Tranche{
	TrancheNonEchue, TrancheSansTerme, Tranche1a30, Tranche31a60, Tranche61a90, TrancheAuDela,
}

// Creance est une facture dont le solde n'est pas réglé.
type Creance struct {
	InvoiceID string     `json:"invoiceId"`
	Number    string     `json:"number"`
	Date      time.Time  `json:"date"`
	DueDate   *time.Time `json:"dueDate,omitempty"`

	CustomerID    string `json:"customerId"`
	CustomerName  string `json:"customerName"`
	CustomerPhone string `json:"customerPhone"`

	Total   int64 `json:"total"`
	Paid    int64 `json:"paid"`
	Balance int64 `json:"balance"`

	// JoursDeRetard est nul tant que l'échéance n'est pas passée.
	JoursDeRetard int     `json:"joursDeRetard"`
	Tranche       Tranche `json:"tranche"`
	Vendeur       string  `json:"vendeur"`
}

// EnRetard indique si l'échéance est dépassée.
func (c Creance) EnRetard() bool { return c.JoursDeRetard > 0 }

// TrancheTotal résume une tranche d'ancienneté.
type TrancheTotal struct {
	Tranche Tranche `json:"tranche"`
	Libelle string  `json:"libelle"`
	Montant int64   `json:"montant"`
	Nombre  int     `json:"nombre"`
	Part    float64 `json:"part"`
}

// DebiteurTotal résume ce que doit un client.
type DebiteurTotal struct {
	PartyID   string `json:"partyId"`
	Nom       string `json:"nom"`
	Telephone string `json:"telephone"`

	Nombre     int   `json:"nombre"`
	Solde      int64 `json:"solde"`
	EnRetard   int64 `json:"enRetard"`
	PlusAncien int   `json:"plusAncien"` // jours de retard de la plus vieille
}

// EtatCreances est la vue complète des impayés.
type EtatCreances struct {
	ArreteAu time.Time `json:"arreteAu"`

	Total        int64 `json:"total"`
	Nombre       int   `json:"nombre"`
	EnRetard     int64 `json:"enRetard"`
	NombreRetard int   `json:"nombreRetard"`

	Tranches  []TrancheTotal  `json:"tranches"`
	Lignes    []Creance       `json:"lignes"`
	Debiteurs []DebiteurTotal `json:"debiteurs"`
}

// CreanceQuery filtre la liste des impayés.
type CreanceQuery struct {
	Search     string `json:"search"`
	CustomerID string `json:"customerId"`
	// SeulementEnRetard restreint aux échéances dépassées.
	SeulementEnRetard bool `json:"seulementEnRetard"`
}

// Etat dresse l'état des créances à l'instant présent.
func (s *Creances) Etat(q CreanceQuery) (EtatCreances, error) {
	if _, err := s.guard("sales"); err != nil {
		return EtatCreances{}, err
	}
	maintenant := time.Now()
	etat := EtatCreances{ArreteAu: maintenant}

	parTranche := map[Tranche]*TrancheTotal{}
	parDebiteur := map[string]*DebiteurTotal{}

	for _, inv := range s.db.Invoices.All() {
		if !countsAsSale(inv) || inv.Balance <= 0 {
			continue
		}
		if q.CustomerID != "" && inv.CustomerID != q.CustomerID {
			continue
		}
		if q.Search != "" && !(util.Contains(inv.Number, q.Search) ||
			util.Contains(inv.CustomerName, q.Search) ||
			util.Contains(inv.CustomerPhone, q.Search)) {
			continue
		}

		retard, tranche := classer(inv.DueDate, maintenant)
		if q.SeulementEnRetard && retard <= 0 {
			continue
		}

		ligne := Creance{
			InvoiceID: inv.ID, Number: inv.Number, Date: inv.Date, DueDate: inv.DueDate,
			CustomerID: inv.CustomerID, CustomerName: inv.CustomerName,
			CustomerPhone: inv.CustomerPhone,
			Total:         inv.Total, Paid: inv.AmountPaid, Balance: inv.Balance,
			JoursDeRetard: retard, Tranche: tranche, Vendeur: inv.UserName,
		}
		etat.Lignes = append(etat.Lignes, ligne)
		etat.Total += inv.Balance
		etat.Nombre++
		if retard > 0 {
			etat.EnRetard += inv.Balance
			etat.NombreRetard++
		}

		t, ok := parTranche[tranche]
		if !ok {
			t = &TrancheTotal{Tranche: tranche, Libelle: LibelleTranche(tranche)}
			parTranche[tranche] = t
		}
		t.Montant += inv.Balance
		t.Nombre++

		// Les ventes au comptoir laissées à crédit sont regroupées sous le nom
		// saisi : sans fiche client, c'est tout ce dont on dispose pour relancer.
		cle := inv.CustomerID
		if cle == "" {
			cle = "nom:" + util.Slug(inv.CustomerName)
		}
		d, ok := parDebiteur[cle]
		if !ok {
			d = &DebiteurTotal{
				PartyID: inv.CustomerID, Nom: inv.CustomerName, Telephone: inv.CustomerPhone,
			}
			parDebiteur[cle] = d
		}
		d.Nombre++
		d.Solde += inv.Balance
		if retard > 0 {
			d.EnRetard += inv.Balance
			if retard > d.PlusAncien {
				d.PlusAncien = retard
			}
		}
		if d.Telephone == "" {
			d.Telephone = inv.CustomerPhone
		}
	}

	// Les plus en retard d'abord : c'est l'ordre dans lequel on relance.
	sort.Slice(etat.Lignes, func(i, j int) bool {
		if etat.Lignes[i].JoursDeRetard != etat.Lignes[j].JoursDeRetard {
			return etat.Lignes[i].JoursDeRetard > etat.Lignes[j].JoursDeRetard
		}
		return etat.Lignes[i].Date.Before(etat.Lignes[j].Date)
	})

	for _, t := range ordreTranches {
		if v, ok := parTranche[t]; ok {
			if etat.Total > 0 {
				v.Part = float64(v.Montant) / float64(etat.Total) * 100
			}
			etat.Tranches = append(etat.Tranches, *v)
		}
	}

	for _, d := range parDebiteur {
		etat.Debiteurs = append(etat.Debiteurs, *d)
	}
	sort.Slice(etat.Debiteurs, func(i, j int) bool {
		if etat.Debiteurs[i].EnRetard != etat.Debiteurs[j].EnRetard {
			return etat.Debiteurs[i].EnRetard > etat.Debiteurs[j].EnRetard
		}
		return etat.Debiteurs[i].Solde > etat.Debiteurs[j].Solde
	})
	return etat, nil
}

// classer calcule le retard et la tranche d'une créance.
//
// Une facture sans échéance convenue n'est pas déclarée en retard : personne
// n'a promis de date. Elle est signalée à part, pour qu'on pense à en fixer une.
func classer(echeance *time.Time, maintenant time.Time) (int, Tranche) {
	if echeance == nil {
		return 0, TrancheSansTerme
	}
	limite := util.EndOfDay(*echeance)
	if !maintenant.After(limite) {
		return 0, TrancheNonEchue
	}
	jours := int(maintenant.Sub(limite).Hours() / 24)
	if jours < 1 {
		jours = 1
	}
	switch {
	case jours <= 30:
		return jours, Tranche1a30
	case jours <= 60:
		return jours, Tranche31a60
	case jours <= 90:
		return jours, Tranche61a90
	default:
		return jours, TrancheAuDela
	}
}

// EcheanceInput reporte ou fixe l'échéance d'une facture impayée.
type EcheanceInput struct {
	InvoiceID string `json:"invoiceId"`
	DueDate   string `json:"dueDate"` // AAAA-MM-JJ, vide pour retirer l'échéance
	Note      string `json:"note"`
}

// FixerEcheance convient d'une nouvelle date de règlement.
//
// Reporter une échéance est une décision commerciale : elle est journalisée et
// inscrite dans les notes de la facture, pour que la trace du report survive au
// changement de vendeur.
func (s *Creances) FixerEcheance(in EcheanceInput) (models.Invoice, error) {
	u, err := s.guard("sales")
	if err != nil {
		return models.Invoice{}, err
	}
	inv, err := s.db.Invoices.Get(in.InvoiceID)
	if err != nil {
		return models.Invoice{}, err
	}
	if !countsAsSale(inv) {
		return models.Invoice{}, fmt.Errorf("la facture %s n'est pas une vente en cours", inv.Number)
	}
	if inv.Balance <= 0 {
		return models.Invoice{}, fmt.Errorf("la facture %s est réglée : elle n'a pas d'échéance", inv.Number)
	}

	ancienne := "aucune"
	if inv.DueDate != nil {
		ancienne = inv.DueDate.Format("02/01/2006")
	}

	if trim(in.DueDate) == "" {
		inv.DueDate = nil
	} else {
		d, err := parseDate(in.DueDate)
		if err != nil {
			return models.Invoice{}, err
		}
		fin := util.EndOfDay(d)
		inv.DueDate = &fin
	}

	nouvelle := "aucune"
	if inv.DueDate != nil {
		nouvelle = inv.DueDate.Format("02/01/2006")
	}
	mention := fmt.Sprintf("%s, échéance portée de %s à %s par %s",
		time.Now().Format("02/01/2006"), ancienne, nouvelle, u.FullName)
	if trim(in.Note) != "" {
		mention += " : " + trim(in.Note)
	}
	inv.Notes = appendNote(inv.Notes, mention)
	inv.UpdatedAt = time.Now()

	if err := s.db.Invoices.Update(inv); err != nil {
		return models.Invoice{}, err
	}
	s.log(u, "ECHEANCE", "invoice", inv.ID, "Facture %s : échéance %s puis %s", inv.Number, ancienne, nouvelle)
	return redactInvoice(inv, s.canSeeCost(u)), nil
}
