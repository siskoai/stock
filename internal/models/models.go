// Package models définit toutes les structures de données persistées par
// Comptoir. Chaque type implémente Entity afin d'être stocké dans une
// collection générique (voir internal/storage).
//
// Convention monétaire : tous les montants sont des entiers int64 exprimés en
// centièmes d'unité monétaire (« minor units »). 1 500 FCFA = 150000.
// Cela évite toute erreur d'arrondi liée aux flottants. Le formatage est
// effectué côté interface (frontend/src/lib/money.ts).
package models

import "time"

// Entity est le contrat minimal d'un enregistrement persistable.
type Entity interface {
	GetID() string
}

// SchemaVersion est incrémentée à chaque changement incompatible du format de
// stockage. Les migrations sont appliquées au démarrage (storage.Migrate).
const SchemaVersion = 1

// ---------------------------------------------------------------------------
// Rôles et permissions
// ---------------------------------------------------------------------------

type Role string

const (
	RoleAdmin   Role = "ADMIN"   // accès total, gestion des utilisateurs
	RoleManager Role = "MANAGER" // stock, ventes, achats, rapports, charges
	RoleSeller  Role = "SELLER"  // ventes et consultation du stock uniquement
)

// User représente un compte local. Le hash du mot de passe n'est jamais
// sérialisé vers l'interface (tag json:"-" sur le champ exporté vers le front,
// voir Sanitized()).
type User struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	FullName      string     `json:"fullName"`
	Role          Role       `json:"role"`
	PasswordHash  string     `json:"passwordHash"`
	Active        bool       `json:"active"`
	MustChangePwd bool       `json:"mustChangePwd"`
	LastLogin     *time.Time `json:"lastLogin,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (u User) GetID() string { return u.ID }

// PublicUser est la projection sûre envoyée à l'interface : aucun secret.
type PublicUser struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	FullName      string     `json:"fullName"`
	Role          Role       `json:"role"`
	Active        bool       `json:"active"`
	MustChangePwd bool       `json:"mustChangePwd"`
	LastLogin     *time.Time `json:"lastLogin,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// Sanitized retire toute donnée sensible avant transmission au frontend.
func (u User) Sanitized() PublicUser {
	return PublicUser{
		ID: u.ID, Username: u.Username, FullName: u.FullName,
		Role: u.Role, Active: u.Active, MustChangePwd: u.MustChangePwd,
		LastLogin: u.LastLogin, CreatedAt: u.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// Catalogue
// ---------------------------------------------------------------------------

// Category classe les produits (Ordinateurs, Imprimantes, Réseau, ...).
type Category struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"` // teinte du badge dans l'interface
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (c Category) GetID() string { return c.ID }

// Product est un article vendable. Quantity est le stock sain disponible ;
// DefectiveQty est isolé et n'est jamais vendable.
type Product struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Barcode     string `json:"barcode"`
	Name        string `json:"name"`
	CategoryID  string `json:"categoryId"`
	Brand       string `json:"brand"`
	Model       string `json:"model"`
	Description string `json:"description"`
	Unit        string `json:"unit"` // pièce, carton, mètre...

	// PurchasePrice est le coût unitaire moyen pondéré (CUMP), recalculé à
	// chaque entrée en stock. Il sert au calcul de la marge et de la valeur
	// d'inventaire.
	PurchasePrice int64 `json:"purchasePrice"`
	SalePrice     int64 `json:"salePrice"`

	Quantity     int `json:"quantity"`
	DefectiveQty int `json:"defectiveQty"`
	MinStock     int `json:"minStock"`

	Location       string    `json:"location"` // emplacement physique (rayon)
	WarrantyMonths int       `json:"warrantyMonths"`
	Serialized     bool      `json:"serialized"` // suivi par numéro de série
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (p Product) GetID() string { return p.ID }

// StockValue est la valeur d'inventaire au coût moyen pondéré.
func (p Product) StockValue() int64 { return int64(p.Quantity) * p.PurchasePrice }

// IsLow indique une rupture imminente.
func (p Product) IsLow() bool { return p.MinStock > 0 && p.Quantity <= p.MinStock }

// ---------------------------------------------------------------------------
// Tiers (clients et fournisseurs)
// ---------------------------------------------------------------------------

type PartyType string

const (
	PartyCustomer PartyType = "CUSTOMER"
	PartySupplier PartyType = "SUPPLIER"
)

// Party regroupe clients et fournisseurs : mêmes champs, même écran, deux
// listes filtrées par Type.
type Party struct {
	ID      string    `json:"id"`
	Type    PartyType `json:"type"`
	Name    string    `json:"name"`
	Company string    `json:"company"`
	Phone   string    `json:"phone"`
	Email   string    `json:"email"`
	Address string    `json:"address"`
	City    string    `json:"city"`
	TaxID   string    `json:"taxId"` // NIF / N° contribuable
	Notes   string    `json:"notes"`
	Active  bool      `json:"active"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (p Party) GetID() string { return p.ID }

// ---------------------------------------------------------------------------
// Mouvements de stock
// ---------------------------------------------------------------------------

type MovementType string

const (
	MovementIn             MovementType = "IN"              // entrée d'achat / réception
	MovementOut            MovementType = "OUT"             // sortie de vente
	MovementReturnCustomer MovementType = "RETURN_CUSTOMER" // retour client → remise en stock
	MovementReturnSupplier MovementType = "RETURN_SUPPLIER" // retour fournisseur → sortie
	MovementDefect         MovementType = "DEFECT"          // constat de défaut : stock → défectueux
	MovementRepair         MovementType = "REPAIR"          // réparation : défectueux → stock
	MovementScrap          MovementType = "SCRAP"           // rebut : défectueux → perte sèche
	MovementAdjust         MovementType = "ADJUST"          // correction d'inventaire
)

// Movement est le journal immuable de toute variation de stock. Aucune
// quantité produit ne change sans écriture d'un mouvement correspondant.
type Movement struct {
	ID   string       `json:"id"`
	Ref  string       `json:"ref"` // MVT-2026-000123
	Type MovementType `json:"type"`
	Date time.Time    `json:"date"`

	ProductID   string `json:"productId"`
	ProductName string `json:"productName"` // instantané, résiste à la suppression
	ProductSKU  string `json:"productSku"`

	Quantity  int   `json:"quantity"`  // toujours positif ; le Type porte le sens
	UnitCost  int64 `json:"unitCost"`  // valorisation au moment du mouvement
	UnitPrice int64 `json:"unitPrice"` // prix de vente le cas échéant

	PartyID   string `json:"partyId"`
	PartyName string `json:"partyName"`

	DocumentID string `json:"documentId"` // facture ou bon d'entrée lié
	DocumentNo string `json:"documentNo"`

	Reason string `json:"reason"`
	Notes  string `json:"notes"`

	StockAfter int `json:"stockAfter"`

	UserID    string    `json:"userId"`
	UserName  string    `json:"userName"`
	CreatedAt time.Time `json:"createdAt"`
}

func (m Movement) GetID() string { return m.ID }

// ---------------------------------------------------------------------------
// Documents commerciaux
// ---------------------------------------------------------------------------

// DocLine est une ligne de facture ou de bon d'entrée. Les champs calculés
// (LineTotal, TaxAmount) sont recalculés côté serveur : la valeur envoyée par
// l'interface n'est jamais utilisée telle quelle.
type DocLine struct {
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	SKU         string  `json:"sku"`
	Quantity    int     `json:"quantity"`
	UnitPrice   int64   `json:"unitPrice"` // PU HT
	UnitCost    int64   `json:"unitCost"`  // coût au moment de la vente (marge)
	Discount    int64   `json:"discount"`  // remise en montant sur la ligne
	TaxRate     float64 `json:"taxRate"`   // en pourcentage, ex. 18
	LineHT      int64   `json:"lineHT"`    // (PU × qté) − remise
	TaxAmount   int64   `json:"taxAmount"`
	LineTTC     int64   `json:"lineTTC"`

	// Serials porte les numéros de série des unités vendues lorsque le produit
	// est suivi à l'unité (Product.Serialized). Vide sinon.
	Serials []string `json:"serials,omitempty"`
}

type DocStatus string

const (
	StatusDraft     DocStatus = "DRAFT"
	StatusIssued    DocStatus = "ISSUED"
	StatusPartial   DocStatus = "PARTIAL"
	StatusPaid      DocStatus = "PAID"
	StatusCancelled DocStatus = "CANCELLED"
)

type PaymentMethod string

const (
	PayCash     PaymentMethod = "CASH"
	PayMobile   PaymentMethod = "MOBILE"
	PayTransfer PaymentMethod = "TRANSFER"
	PayCheck    PaymentMethod = "CHECK"
	PayCredit   PaymentMethod = "CREDIT"
)

// Invoice est une facture de vente. Elle décrémente le stock à l'émission et
// constitue la source unique du chiffre d'affaires.
type Invoice struct {
	ID     string    `json:"id"`
	Number string    `json:"number"` // FA-2026-0042
	Date   time.Time `json:"date"`

	CustomerID      string `json:"customerId"`
	CustomerName    string `json:"customerName"`
	CustomerPhone   string `json:"customerPhone"`
	CustomerAddress string `json:"customerAddress"`
	CustomerTaxID   string `json:"customerTaxId"`

	Lines []DocLine `json:"lines"`

	SubtotalHT     int64 `json:"subtotalHT"`
	GlobalDiscount int64 `json:"globalDiscount"`
	TaxTotal       int64 `json:"taxTotal"`
	Total          int64 `json:"total"` // net à payer TTC
	AmountPaid     int64 `json:"amountPaid"`
	Balance        int64 `json:"balance"`
	CostTotal      int64 `json:"costTotal"` // coût des marchandises vendues
	Margin         int64 `json:"margin"`    // SubtotalHT − GlobalDiscount − CostTotal

	// RefundDue est l'acompte encaissé qui reste à rembourser après annulation
	// de la facture. Nul dans tous les autres cas.
	RefundDue int64 `json:"refundDue"`

	// DueDate est l'échéance convenue pour le solde. Elle n'est renseignée que
	// sur les ventes laissées à crédit : une facture réglée comptant n'a pas
	// d'échéance, et une échéance sans dette n'aurait aucun sens.
	DueDate *time.Time `json:"dueDate,omitempty"`

	PaymentMethod PaymentMethod `json:"paymentMethod"`
	Status        DocStatus     `json:"status"`
	Notes         string        `json:"notes"`

	UserID      string     `json:"userId"`
	UserName    string     `json:"userName"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CancelledAt *time.Time `json:"cancelledAt,omitempty"`
}

func (i Invoice) GetID() string { return i.ID }

// Purchase est un bon d'entrée (réception de marchandise fournisseur).
type Purchase struct {
	ID     string    `json:"id"`
	Number string    `json:"number"` // BE-2026-0031
	Date   time.Time `json:"date"`

	SupplierID   string `json:"supplierId"`
	SupplierName string `json:"supplierName"`
	Reference    string `json:"reference"` // n° de facture fournisseur

	Lines []DocLine `json:"lines"`

	SubtotalHT int64 `json:"subtotalHT"`
	TaxTotal   int64 `json:"taxTotal"`
	OtherCosts int64 `json:"otherCosts"` // transport, douane, réparti au prorata
	Total      int64 `json:"total"`

	Status DocStatus `json:"status"`
	Notes  string    `json:"notes"`

	UserID      string     `json:"userId"`
	UserName    string     `json:"userName"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CancelledAt *time.Time `json:"cancelledAt,omitempty"`
}

func (p Purchase) GetID() string { return p.ID }

// ---------------------------------------------------------------------------
// Charges d'exploitation
// ---------------------------------------------------------------------------

// ExpenseCategories est la liste par défaut proposée dans l'interface.
var ExpenseCategories = []string{
	"Loyer", "Salaires", "Électricité", "Eau", "Internet", "Transport",
	"Fournitures", "Maintenance", "Publicité", "Impôts et taxes",
	"Frais bancaires", "Divers",
}

// Expense est une charge d'exploitation, hors coût d'achat des marchandises.
type Expense struct {
	ID       string    `json:"id"`
	Date     time.Time `json:"date"`
	Category string    `json:"category"`
	Label    string    `json:"label"`
	Amount   int64     `json:"amount"`

	PaymentMethod PaymentMethod `json:"paymentMethod"`
	Beneficiary   string        `json:"beneficiary"`
	Notes         string        `json:"notes"`

	UserID    string    `json:"userId"`
	UserName  string    `json:"userName"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (e Expense) GetID() string { return e.ID }

// ---------------------------------------------------------------------------
// Journal d'audit
// ---------------------------------------------------------------------------

// AuditEntry trace chaque action sensible. Le journal est en ajout seul.
type AuditEntry struct {
	ID       string    `json:"id"`
	At       time.Time `json:"at"`
	UserID   string    `json:"userId"`
	UserName string    `json:"userName"`
	Action   string    `json:"action"` // CREATE, UPDATE, DELETE, LOGIN, CANCEL...
	Entity   string    `json:"entity"` // product, invoice, user...
	EntityID string    `json:"entityId"`
	Details  string    `json:"details"`
}

func (a AuditEntry) GetID() string { return a.ID }

// ---------------------------------------------------------------------------
// Paramètres
// ---------------------------------------------------------------------------

// Settings contient l'identité de l'entreprise et les réglages globaux.
// Un seul enregistrement, stocké dans settings.json.
type Settings struct {
	ID string `json:"id"`

	CompanyName string `json:"companyName"`
	LegalForm   string `json:"legalForm"`
	TaxID       string `json:"taxId"` // NIF
	RCCM        string `json:"rccm"`  // registre du commerce
	Address     string `json:"address"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Website     string `json:"website"`
	LogoDataURL string `json:"logoDataUrl"` // image embarquée (data:image/png;base64,...)

	Currency       string `json:"currency"`       // code ISO, ex. XOF
	CurrencySymbol string `json:"currencySymbol"` // affiché, ex. FCFA
	Decimals       int    `json:"decimals"`       // 0 pour le FCFA, 2 pour EUR/USD

	DefaultTaxRate   float64 `json:"defaultTaxRate"` // TVA appliquée par défaut
	PricesIncludeTax bool    `json:"pricesIncludeTax"`

	// DefaultPaymentTermDays est le délai de règlement proposé sur une vente à
	// crédit, en jours. Zéro laisse l'échéance à la discrétion du vendeur.
	DefaultPaymentTermDays int `json:"defaultPaymentTermDays"`

	InvoicePrefix   string `json:"invoicePrefix"`
	InvoiceCounter  int    `json:"invoiceCounter"`
	PurchasePrefix  string `json:"purchasePrefix"`
	PurchaseCounter int    `json:"purchaseCounter"`
	MovementCounter int    `json:"movementCounter"`
	InvoiceFooter   string `json:"invoiceFooter"`
	InvoiceTerms    string `json:"invoiceTerms"`

	FiscalYearStartMonth int    `json:"fiscalYearStartMonth"` // 1 = janvier
	AutoBackup           bool   `json:"autoBackup"`
	BackupsToKeep        int    `json:"backupsToKeep"`
	SessionTimeoutMin    int    `json:"sessionTimeoutMin"`
	Theme                string `json:"theme"` // light | dark | system

	UpdatedAt time.Time `json:"updatedAt"`
}

func (s Settings) GetID() string { return s.ID }

// DefaultSettings renvoie une configuration initiale adaptée à une boutique
// de matériel informatique en zone UEMOA.
func DefaultSettings() Settings {
	return Settings{
		ID:                     "settings",
		CompanyName:            "Ma Société",
		Country:                "Mali",
		Currency:               "XOF",
		CurrencySymbol:         "FCFA",
		Decimals:               0,
		DefaultTaxRate:         18,
		InvoicePrefix:          "FA",
		PurchasePrefix:         "BE",
		InvoiceTerms:           "Marchandise vendue ne peut être ni reprise ni échangée, sauf défaut constaté sous 7 jours.",
		InvoiceFooter:          "Merci de votre confiance.",
		FiscalYearStartMonth:   1,
		DefaultPaymentTermDays: 30,
		AutoBackup:             true,
		BackupsToKeep:          30,
		SessionTimeoutMin:      60,
		Theme:                  "light",
		UpdatedAt:              time.Now(),
	}
}

// DefaultCategories amorce le catalogue au premier lancement.
var DefaultCategories = []struct{ Name, Description, Color string }{
	{"Ordinateurs portables", "PC portables, ultrabooks, stations mobiles", "blue"},
	{"Ordinateurs de bureau", "Unités centrales, tout-en-un", "aqua"},
	{"Écrans", "Moniteurs et vidéoprojecteurs", "violet"},
	{"Imprimantes", "Jet d'encre, laser, multifonctions", "orange"},
	{"Consommables", "Cartouches, toners, papier", "yellow"},
	{"Composants", "RAM, disques, cartes, alimentations", "magenta"},
	{"Réseau", "Routeurs, switchs, câblage", "green"},
	{"Accessoires", "Claviers, souris, sacoches, onduleurs", "red"},
}
