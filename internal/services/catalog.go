package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"comptoir/internal/auth"
	"comptoir/internal/models"
	"comptoir/internal/storage"
	"comptoir/internal/util"
)

// Catalog gère les catégories, les produits et les tiers (clients,
// fournisseurs). Exposé à l'interface sous le nom « Catalog ».
type Catalog struct{ core }

// NewCatalog construit le service catalogue.
func NewCatalog(db *storage.Database, a *auth.Service) *Catalog {
	return &Catalog{core{db: db, auth: a}}
}

// ---------------------------------------------------------------------------
// Catégories
// ---------------------------------------------------------------------------

// CategoryInput est la charge utile reçue de l'interface. Un ID vide signifie
// « créer », un ID renseigné signifie « modifier ».
type CategoryInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// CategoryView ajoute le nombre de produits rattachés, pour éviter un calcul
// côté interface.
type CategoryView struct {
	models.Category
	ProductCount int   `json:"productCount"`
	StockValue   int64 `json:"stockValue"`
	StockUnits   int   `json:"stockUnits"`
}

// ListCategories renvoie les catégories triées par nom, avec leurs indicateurs.
func (s *Catalog) ListCategories() ([]CategoryView, error) {
	u, err := s.guard("")
	if err != nil {
		return nil, err
	}
	showCost := s.canSeeCost(u)
	products := s.db.Products.All()
	cats := s.db.Categories.All()

	out := make([]CategoryView, 0, len(cats))
	for _, c := range cats {
		v := CategoryView{Category: c}
		for _, p := range products {
			if p.CategoryID == c.ID {
				v.ProductCount++
				v.StockUnits += p.Quantity
				v.StockValue += p.StockValue()
			}
		}
		if !showCost {
			v.StockValue = 0
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return util.Slug(out[i].Name) < util.Slug(out[j].Name) })
	return out, nil
}

// SaveCategory crée ou met à jour une catégorie.
func (s *Catalog) SaveCategory(in CategoryInput) (models.Category, error) {
	u, err := s.guard("catalog")
	if err != nil {
		return models.Category{}, err
	}
	name, err := requireText(in.Name, "Nom")
	if err != nil {
		return models.Category{}, err
	}
	// Unicité du nom, insensible à la casse et aux accents.
	if dup, err := s.db.Categories.FindOne(func(c models.Category) bool {
		return c.ID != in.ID && util.Slug(c.Name) == util.Slug(name)
	}); err == nil {
		return models.Category{}, fmt.Errorf("la catégorie « %s » existe déjà", dup.Name)
	}

	now := time.Now()
	if in.ID == "" {
		c := models.Category{
			ID: util.NewID("cat"), Name: name, Description: trim(in.Description),
			Color: defaultString(in.Color, "blue"), CreatedAt: now, UpdatedAt: now,
		}
		if err := s.db.Categories.Insert(c); err != nil {
			return models.Category{}, err
		}
		s.log(u, "CREATE", "category", c.ID, "Catégorie « %s » créée", c.Name)
		return c, nil
	}

	c, err := s.db.Categories.Get(in.ID)
	if err != nil {
		return models.Category{}, err
	}
	c.Name, c.Description, c.UpdatedAt = name, trim(in.Description), now
	c.Color = defaultString(in.Color, c.Color)
	if err := s.db.Categories.Update(c); err != nil {
		return models.Category{}, err
	}
	s.log(u, "UPDATE", "category", c.ID, "Catégorie « %s » modifiée", c.Name)
	return c, nil
}

// DeleteCategory supprime une catégorie vide. Une catégorie contenant des
// produits n'est pas supprimable : cela orphelinerait des articles.
func (s *Catalog) DeleteCategory(id string) error {
	u, err := s.guard("delete")
	if err != nil {
		return err
	}
	c, err := s.db.Categories.Get(id)
	if err != nil {
		return err
	}
	used := s.db.Products.Find(func(p models.Product) bool { return p.CategoryID == id })
	if len(used) > 0 {
		return fmt.Errorf("« %s » contient %d produit(s) : déplacez-les d'abord vers une autre catégorie", c.Name, len(used))
	}
	if err := s.db.Categories.Delete(id); err != nil {
		return err
	}
	s.log(u, "DELETE", "category", id, "Catégorie « %s » supprimée", c.Name)
	return nil
}

// ---------------------------------------------------------------------------
// Produits
// ---------------------------------------------------------------------------

// ProductInput est la charge utile de création ou de modification d'un produit.
// Les quantités n'y figurent pas : le stock ne se modifie que par un mouvement
// (entrée, sortie, retour, inventaire), jamais par édition de la fiche.
type ProductInput struct {
	ID             string `json:"id"`
	SKU            string `json:"sku"`
	Barcode        string `json:"barcode"`
	Name           string `json:"name"`
	CategoryID     string `json:"categoryId"`
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	Description    string `json:"description"`
	Unit           string `json:"unit"`
	PurchasePrice  int64  `json:"purchasePrice"`
	SalePrice      int64  `json:"salePrice"`
	MinStock       int    `json:"minStock"`
	Location       string `json:"location"`
	WarrantyMonths int    `json:"warrantyMonths"`
	Serialized     bool   `json:"serialized"`
	Active         bool   `json:"active"`

	// InitialQuantity n'est pris en compte qu'à la création : il génère un
	// mouvement d'inventaire initial et non une écriture silencieuse.
	InitialQuantity int `json:"initialQuantity"`
}

// ProductView enrichit un produit des informations d'affichage.
type ProductView struct {
	models.Product
	CategoryName  string  `json:"categoryName"`
	CategoryColor string  `json:"categoryColor"`
	StockValue    int64   `json:"stockValue"`
	MarginAmount  int64   `json:"marginAmount"`
	MarginRate    float64 `json:"marginRate"` // en pourcentage du prix de vente
	Low           bool    `json:"low"`
	OutOfStock    bool    `json:"outOfStock"`
}

// ProductQuery filtre la liste des produits.
type ProductQuery struct {
	Search          string `json:"search"`
	CategoryID      string `json:"categoryId"`
	OnlyLow         bool   `json:"onlyLow"`
	OnlyDefect      bool   `json:"onlyDefect"`
	IncludeArchived bool   `json:"includeArchived"`
	SortBy          string `json:"sortBy"` // name | sku | quantity | value | margin
	SortDesc        bool   `json:"sortDesc"`
}

// ListProducts renvoie les produits filtrés et triés.
func (s *Catalog) ListProducts(q ProductQuery) ([]ProductView, error) {
	u, err := s.guard("")
	if err != nil {
		return nil, err
	}
	cats := map[string]models.Category{}
	for _, c := range s.db.Categories.All() {
		cats[c.ID] = c
	}

	out := []ProductView{}
	for _, p := range s.db.Products.All() {
		if !p.Active && !q.IncludeArchived {
			continue
		}
		if q.CategoryID != "" && p.CategoryID != q.CategoryID {
			continue
		}
		if q.OnlyLow && !p.IsLow() && p.Quantity > 0 {
			continue
		}
		if q.OnlyDefect && p.DefectiveQty == 0 {
			continue
		}
		if q.Search != "" && !matchesProduct(p, q.Search) {
			continue
		}
		out = append(out, s.viewProduct(p, cats, s.canSeeCost(u)))
	}
	sortProducts(out, q.SortBy, q.SortDesc)
	return out, nil
}

func matchesProduct(p models.Product, search string) bool {
	return util.Contains(p.Name, search) ||
		util.Contains(p.SKU, search) ||
		util.Contains(p.Brand, search) ||
		util.Contains(p.Model, search) ||
		util.Contains(p.Barcode, search) ||
		util.Contains(p.Description, search)
}

// viewProduct enrichit un produit pour l'affichage. showCost à faux retire le
// prix d'achat et tout ce qui s'en déduit : un vendeur voit ce qu'il vend, pas
// ce que la boutique paie. Le filtrage est fait ici, à la source, et pas dans
// l'interface — masquer une colonne n'est pas une protection.
func (s *Catalog) viewProduct(p models.Product, cats map[string]models.Category, showCost bool) ProductView {
	v := ProductView{
		Product:    p,
		StockValue: p.StockValue(),
		Low:        p.IsLow(),
		OutOfStock: p.Quantity <= 0,
	}
	if c, ok := cats[p.CategoryID]; ok {
		v.CategoryName, v.CategoryColor = c.Name, c.Color
	} else {
		v.CategoryName = "Sans catégorie"
		v.CategoryColor = "muted"
	}
	v.MarginAmount = p.SalePrice - p.PurchasePrice
	if p.SalePrice > 0 {
		v.MarginRate = float64(v.MarginAmount) / float64(p.SalePrice) * 100
	}
	if !showCost {
		v.PurchasePrice, v.StockValue, v.MarginAmount, v.MarginRate = 0, 0, 0, 0
	}
	return v
}

func sortProducts(list []ProductView, by string, desc bool) {
	less := func(i, j int) bool { return util.Slug(list[i].Name) < util.Slug(list[j].Name) }
	switch by {
	case "sku":
		less = func(i, j int) bool { return util.Slug(list[i].SKU) < util.Slug(list[j].SKU) }
	case "quantity":
		less = func(i, j int) bool { return list[i].Quantity < list[j].Quantity }
	case "value":
		less = func(i, j int) bool { return list[i].StockValue < list[j].StockValue }
	case "margin":
		less = func(i, j int) bool { return list[i].MarginRate < list[j].MarginRate }
	case "price":
		less = func(i, j int) bool { return list[i].SalePrice < list[j].SalePrice }
	}
	if desc {
		inner := less
		less = func(i, j int) bool { return inner(j, i) }
	}
	sort.SliceStable(list, less)
}

// GetProduct renvoie une fiche produit enrichie.
func (s *Catalog) GetProduct(id string) (ProductView, error) {
	u, err := s.guard("")
	if err != nil {
		return ProductView{}, err
	}
	p, err := s.db.Products.Get(id)
	if err != nil {
		return ProductView{}, err
	}
	cats := map[string]models.Category{}
	for _, c := range s.db.Categories.All() {
		cats[c.ID] = c
	}
	return s.viewProduct(p, cats, s.canSeeCost(u)), nil
}

// SaveProduct crée ou met à jour une fiche produit.
func (s *Catalog) SaveProduct(in ProductInput) (models.Product, error) {
	u, err := s.guard("catalog")
	if err != nil {
		return models.Product{}, err
	}
	name, err := requireText(in.Name, "Désignation")
	if err != nil {
		return models.Product{}, err
	}
	if in.CategoryID != "" {
		if _, err := s.db.Categories.Get(in.CategoryID); err != nil {
			return models.Product{}, fmt.Errorf("catégorie introuvable")
		}
	}
	if in.SalePrice < 0 || in.PurchasePrice < 0 {
		return models.Product{}, fmt.Errorf("les prix ne peuvent pas être négatifs")
	}

	sku := strings.ToUpper(trim(in.SKU))
	if sku != "" {
		if dup, err := s.db.Products.FindOne(func(p models.Product) bool {
			return p.ID != in.ID && strings.EqualFold(p.SKU, sku)
		}); err == nil {
			return models.Product{}, fmt.Errorf("la référence %s est déjà utilisée par « %s »", sku, dup.Name)
		}
	}

	now := time.Now()
	if in.ID == "" {
		if sku == "" {
			sku = s.generateSKU(name)
		}
		p := models.Product{
			ID: util.NewID("prd"), SKU: sku, Barcode: trim(in.Barcode), Name: name,
			CategoryID: in.CategoryID, Brand: trim(in.Brand), Model: trim(in.Model),
			Description: trim(in.Description), Unit: defaultString(in.Unit, "pièce"),
			PurchasePrice: in.PurchasePrice, SalePrice: in.SalePrice,
			MinStock: maxInt(in.MinStock, 0), Location: trim(in.Location),
			WarrantyMonths: maxInt(in.WarrantyMonths, 0), Serialized: in.Serialized,
			Active: true, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.db.Products.Insert(p); err != nil {
			return models.Product{}, err
		}
		s.log(u, "CREATE", "product", p.ID, "Produit « %s » (%s) créé", p.Name, p.SKU)

		if in.InitialQuantity > 0 {
			if err := s.recordInitialStock(u, &p, in.InitialQuantity); err != nil {
				return p, err
			}
		}
		return p, nil
	}

	p, err := s.db.Products.Get(in.ID)
	if err != nil {
		return models.Product{}, err
	}
	p.SKU = defaultString(sku, p.SKU)
	p.Barcode, p.Name, p.CategoryID = trim(in.Barcode), name, in.CategoryID
	p.Brand, p.Model, p.Description = trim(in.Brand), trim(in.Model), trim(in.Description)
	p.Unit = defaultString(in.Unit, p.Unit)
	p.PurchasePrice, p.SalePrice = in.PurchasePrice, in.SalePrice
	p.MinStock, p.Location = maxInt(in.MinStock, 0), trim(in.Location)
	p.WarrantyMonths, p.Serialized, p.Active = maxInt(in.WarrantyMonths, 0), in.Serialized, in.Active
	p.UpdatedAt = now

	if err := s.db.Products.Update(p); err != nil {
		return models.Product{}, err
	}
	s.log(u, "UPDATE", "product", p.ID, "Produit « %s » modifié", p.Name)
	return p, nil
}

// recordInitialStock inscrit le stock de départ comme un mouvement d'inventaire
// pour que l'historique soit complet dès la création de la fiche.
func (s *Catalog) recordInitialStock(u models.User, p *models.Product, qty int) error {
	if _, err := s.applyStock(u, []stockOp{{
		ProductID: p.ID,
		Delta:     stockDelta{Sound: qty},
		Movement: models.Movement{
			Type: models.MovementAdjust, Date: time.Now(), Quantity: qty,
			UnitCost: p.PurchasePrice, Reason: "Stock initial à la création de la fiche",
		},
	}}); err != nil {
		return err
	}
	p.Quantity = qty
	return nil
}

// generateSKU propose une référence lisible à partir de la désignation.
// Exemple : « Imprimante HP LaserJet » → IMP-HP-LAS-4821.
func (s *Catalog) generateSKU(name string) string {
	words := strings.Fields(strings.ToUpper(util.Slug(name)))
	parts := []string{}
	for i, w := range words {
		if i >= 3 {
			break
		}
		if len(w) > 3 {
			w = w[:3]
		}
		parts = append(parts, w)
	}
	if len(parts) == 0 {
		parts = append(parts, "ART")
	}
	base := strings.Join(parts, "-")
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s-%04d", base, s.db.Products.Count()+n)
		if _, err := s.db.Products.FindOne(func(p models.Product) bool {
			return strings.EqualFold(p.SKU, candidate)
		}); err != nil {
			return candidate
		}
	}
}

// ArchiveProduct désactive un produit sans effacer son historique. C'est
// l'opération recommandée pour un article qui n'est plus vendu.
func (s *Catalog) ArchiveProduct(id string, archived bool) error {
	u, err := s.guard("catalog")
	if err != nil {
		return err
	}
	p, err := s.db.Products.Get(id)
	if err != nil {
		return err
	}
	p.Active = !archived
	p.UpdatedAt = time.Now()
	if err := s.db.Products.Update(p); err != nil {
		return err
	}
	action := "réactivé"
	if archived {
		action = "archivé"
	}
	s.log(u, "UPDATE", "product", id, "Produit « %s » %s", p.Name, action)
	return nil
}

// DeleteProduct supprime définitivement un produit. Refusé si le produit a un
// stock non nul ou apparaît dans un mouvement : supprimer effacerait de
// l'historique comptable. L'archivage reste possible dans ce cas.
func (s *Catalog) DeleteProduct(id string) error {
	u, err := s.guard("delete")
	if err != nil {
		return err
	}
	p, err := s.db.Products.Get(id)
	if err != nil {
		return err
	}
	if p.Quantity != 0 || p.DefectiveQty != 0 {
		return fmt.Errorf("« %s » a encore du stock (%d sain, %d défectueux) : soldez-le ou archivez la fiche",
			p.Name, p.Quantity, p.DefectiveQty)
	}
	if n := len(s.db.Movements.Find(func(m models.Movement) bool { return m.ProductID == id })); n > 0 {
		return fmt.Errorf("« %s » apparaît dans %d mouvement(s) de stock : archivez la fiche plutôt que de la supprimer, l'historique doit rester intact", p.Name, n)
	}
	if err := s.db.Products.Delete(id); err != nil {
		return err
	}
	s.log(u, "DELETE", "product", id, "Produit « %s » (%s) supprimé", p.Name, p.SKU)
	return nil
}

// LowStockProducts renvoie les articles au seuil d'alerte ou en rupture, les
// plus critiques d'abord.
func (s *Catalog) LowStockProducts() ([]ProductView, error) {
	u, err := s.guard("")
	if err != nil {
		return nil, err
	}
	cats := map[string]models.Category{}
	for _, c := range s.db.Categories.All() {
		cats[c.ID] = c
	}
	showCost := s.canSeeCost(u)
	out := []ProductView{}
	for _, p := range s.db.Products.All() {
		if p.Active && (p.IsLow() || p.Quantity <= 0) {
			out = append(out, s.viewProduct(p, cats, showCost))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Quantity < out[j].Quantity })
	return out, nil
}

// ---------------------------------------------------------------------------
// Tiers : clients et fournisseurs
// ---------------------------------------------------------------------------

// PartyInput est la charge utile de création ou modification d'un tiers.
type PartyInput struct {
	ID      string           `json:"id"`
	Type    models.PartyType `json:"type"`
	Name    string           `json:"name"`
	Company string           `json:"company"`
	Phone   string           `json:"phone"`
	Email   string           `json:"email"`
	Address string           `json:"address"`
	City    string           `json:"city"`
	TaxID   string           `json:"taxId"`
	Notes   string           `json:"notes"`
	Active  bool             `json:"active"`
}

// PartyView ajoute l'activité commerciale du tiers.
type PartyView struct {
	models.Party
	DocumentCount      int        `json:"documentCount"`
	TotalAmount        int64      `json:"totalAmount"`
	OutstandingBalance int64      `json:"outstandingBalance"` // impayés, clients uniquement
	LastActivity       *time.Time `json:"lastActivity,omitempty"`
}

// ListParties renvoie les tiers d'un type donné, avec leurs cumuls.
func (s *Catalog) ListParties(partyType string, search string) ([]PartyView, error) {
	if _, err := s.guard(""); err != nil {
		return nil, err
	}
	invoices := s.db.Invoices.All()
	purchases := s.db.Purchases.All()

	out := []PartyView{}
	for _, p := range s.db.Parties.All() {
		if partyType != "" && string(p.Type) != partyType {
			continue
		}
		if search != "" && !(util.Contains(p.Name, search) || util.Contains(p.Company, search) ||
			util.Contains(p.Phone, search) || util.Contains(p.Email, search)) {
			continue
		}
		v := PartyView{Party: p}
		if p.Type == models.PartyCustomer {
			for _, inv := range invoices {
				if inv.CustomerID != p.ID || inv.Status == models.StatusCancelled {
					continue
				}
				v.DocumentCount++
				v.TotalAmount += inv.Total
				v.OutstandingBalance += inv.Balance
				v.LastActivity = laterOf(v.LastActivity, inv.Date)
			}
		} else {
			for _, pu := range purchases {
				if pu.SupplierID != p.ID || pu.Status == models.StatusCancelled {
					continue
				}
				v.DocumentCount++
				v.TotalAmount += pu.Total
				v.LastActivity = laterOf(v.LastActivity, pu.Date)
			}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return util.Slug(out[i].Name) < util.Slug(out[j].Name) })
	return out, nil
}

// SaveParty crée ou met à jour un client ou un fournisseur.
func (s *Catalog) SaveParty(in PartyInput) (models.Party, error) {
	u, err := s.guard("catalog")
	if err != nil {
		// Un vendeur doit pouvoir créer un client au moment de la vente.
		if in.Type != models.PartyCustomer {
			return models.Party{}, err
		}
		if u, err = s.guard("sales"); err != nil {
			return models.Party{}, err
		}
	}
	name, err := requireText(in.Name, "Nom")
	if err != nil {
		return models.Party{}, err
	}
	if in.Type != models.PartyCustomer && in.Type != models.PartySupplier {
		return models.Party{}, fmt.Errorf("type de tiers invalide")
	}
	if in.Email != "" && !strings.Contains(in.Email, "@") {
		return models.Party{}, fmt.Errorf("adresse e-mail invalide")
	}

	now := time.Now()
	if in.ID == "" {
		p := models.Party{
			ID: util.NewID("prt"), Type: in.Type, Name: name, Company: trim(in.Company),
			Phone: trim(in.Phone), Email: trim(in.Email), Address: trim(in.Address),
			City: trim(in.City), TaxID: trim(in.TaxID), Notes: trim(in.Notes),
			Active: true, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.db.Parties.Insert(p); err != nil {
			return models.Party{}, err
		}
		s.log(u, "CREATE", "party", p.ID, "%s « %s » créé", partyLabel(p.Type), p.Name)
		return p, nil
	}

	p, err := s.db.Parties.Get(in.ID)
	if err != nil {
		return models.Party{}, err
	}
	p.Name, p.Company, p.Phone, p.Email = name, trim(in.Company), trim(in.Phone), trim(in.Email)
	p.Address, p.City, p.TaxID, p.Notes = trim(in.Address), trim(in.City), trim(in.TaxID), trim(in.Notes)
	p.Active, p.UpdatedAt = in.Active, now
	if err := s.db.Parties.Update(p); err != nil {
		return models.Party{}, err
	}
	s.log(u, "UPDATE", "party", p.ID, "%s « %s » modifié", partyLabel(p.Type), p.Name)
	return p, nil
}

// DeleteParty supprime un tiers sans historique commercial.
func (s *Catalog) DeleteParty(id string) error {
	u, err := s.guard("delete")
	if err != nil {
		return err
	}
	p, err := s.db.Parties.Get(id)
	if err != nil {
		return err
	}
	used := len(s.db.Invoices.Find(func(i models.Invoice) bool { return i.CustomerID == id })) +
		len(s.db.Purchases.Find(func(pu models.Purchase) bool { return pu.SupplierID == id }))
	if used > 0 {
		return fmt.Errorf("« %s » est lié à %d document(s) : désactivez la fiche plutôt que de la supprimer", p.Name, used)
	}
	if err := s.db.Parties.Delete(id); err != nil {
		return err
	}
	s.log(u, "DELETE", "party", id, "%s « %s » supprimé", partyLabel(p.Type), p.Name)
	return nil
}

// ---------------------------------------------------------------------------

func partyLabel(t models.PartyType) string {
	if t == models.PartySupplier {
		return "Fournisseur"
	}
	return "Client"
}

func defaultString(v, fallback string) string {
	if trim(v) == "" {
		return fallback
	}
	return trim(v)
}

func maxInt(v, min int) int {
	if v < min {
		return min
	}
	return v
}

func laterOf(current *time.Time, candidate time.Time) *time.Time {
	if current == nil || candidate.After(*current) {
		c := candidate
		return &c
	}
	return current
}
