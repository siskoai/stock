// Types miroirs des structures Go exposées par Wails.
//
// Ils sont écrits à la main plutôt que générés : l'application n'utilise pas
// les liaisons produites par « wails generate module », mais appelle
// directement window.go.*. Le contrat reste donc visible et versionné ici,
// au même endroit que le code qui s'en sert.

// Tous les montants sont des entiers en centièmes d'unité monétaire.
export type Money = number

export type Role = 'ADMIN' | 'MANAGER' | 'SELLER'
export type Scope =
  | 'catalog' | 'stock' | 'sales' | 'purchases' | 'expenses'
  | 'finance' | 'users' | 'settings' | 'backup' | 'delete'

export type DocStatus = 'DRAFT' | 'ISSUED' | 'PARTIAL' | 'PAID' | 'CANCELLED'
export type PaymentMethod = 'CASH' | 'MOBILE' | 'TRANSFER' | 'CHECK' | 'CREDIT'
export type PartyType = 'CUSTOMER' | 'SUPPLIER'
export type MovementType =
  | 'IN' | 'OUT' | 'RETURN_CUSTOMER' | 'RETURN_SUPPLIER'
  | 'DEFECT' | 'REPAIR' | 'SCRAP' | 'ADJUST'
export type Granularity = 'day' | 'week' | 'month' | 'quarter' | 'semester' | 'year'

export interface PublicUser {
  id: string
  username: string
  fullName: string
  role: Role
  active: boolean
  mustChangePwd: boolean
  lastLogin?: string
  createdAt: string
}

export interface State {
  needsSetup: boolean
  authenticated: boolean
  user?: PublicUser
  scopes: Scope[]
  companyName: string
  currencySymbol: string
  decimals: number
  theme: string
  appVersion: string
  author: string
  notice: string
  brandingIntact: boolean
  companyLogoFingerprint: string
}

/** Identité visuelle de l'auteur, demandée une fois au démarrage. */
export interface Attribution {
  author: string
  notice: string
  logoDataUrl: string
  licenseRef: string
  intact: boolean
  alert?: string
}

/** Réponses de l'assistant de premier démarrage, envoyées d'un seul bloc. */
export interface SetupInput {
  username: string; fullName: string; password: string
  companyName: string; legalForm: string; taxId: string; rccm: string
  address: string; city: string; country: string; phone: string; email: string
  currency: string; currencySymbol: string; decimals: number
  defaultTaxRate: number; pricesIncludeTax: boolean
  seedCategories: boolean
  autoBackup: boolean; backupsToKeep: number
  theme: string
}

export interface Category {
  id: string; name: string; description: string; color: string
  createdAt: string; updatedAt: string
}
export interface CategoryView extends Category {
  productCount: number; stockValue: Money; stockUnits: number
}

export interface Product {
  id: string; sku: string; barcode: string; name: string
  categoryId: string; brand: string; model: string; description: string
  unit: string; purchasePrice: Money; salePrice: Money
  quantity: number; defectiveQty: number; minStock: number
  location: string; warrantyMonths: number; serialized: boolean
  active: boolean; createdAt: string; updatedAt: string
}
export interface ProductView extends Product {
  categoryName: string; categoryColor: string
  stockValue: Money; marginAmount: Money; marginRate: number
  low: boolean; outOfStock: boolean
}

export interface Party {
  id: string; type: PartyType; name: string; company: string
  phone: string; email: string; address: string; city: string
  taxId: string; notes: string; active: boolean
  createdAt: string; updatedAt: string
}
export interface PartyView extends Party {
  documentCount: number; totalAmount: Money
  outstandingBalance: Money; lastActivity?: string
}

export interface DocLine {
  productId: string; productName: string; sku: string
  quantity: number; unitPrice: Money; unitCost: Money
  discount: Money; taxRate: number
  lineHT: Money; taxAmount: Money; lineTTC: Money
  serials?: string[]
}

export interface Invoice {
  id: string; number: string; date: string
  customerId: string; customerName: string; customerPhone: string
  customerAddress: string; customerTaxId: string
  lines: DocLine[]
  subtotalHT: Money; globalDiscount: Money; taxTotal: Money; total: Money
  amountPaid: Money; balance: Money; costTotal: Money; margin: Money
  refundDue: Money
  paymentMethod: PaymentMethod; status: DocStatus; notes: string
  userId: string; userName: string
  createdAt: string; updatedAt: string; cancelledAt?: string
}

export interface Purchase {
  id: string; number: string; date: string
  supplierId: string; supplierName: string; reference: string
  lines: DocLine[]
  subtotalHT: Money; taxTotal: Money; otherCosts: Money; total: Money
  status: DocStatus; notes: string
  userId: string; userName: string
  createdAt: string; updatedAt: string; cancelledAt?: string
}

export interface Movement {
  id: string; ref: string; type: MovementType; date: string
  productId: string; productName: string; productSku: string
  quantity: number; unitCost: Money; unitPrice: Money
  partyId: string; partyName: string
  documentId: string; documentNo: string
  reason: string; notes: string; stockAfter: number
  userId: string; userName: string; createdAt: string
}

export interface Expense {
  id: string; date: string; category: string; label: string
  amount: Money; paymentMethod: PaymentMethod
  beneficiary: string; notes: string
  userId: string; userName: string; createdAt: string; updatedAt: string
}

export interface AuditEntry {
  id: string; at: string; userId: string; userName: string
  action: string; entity: string; entityId: string; details: string
}

export interface Settings {
  id: string
  companyName: string; legalForm: string; taxId: string; rccm: string
  address: string; city: string; country: string
  phone: string; email: string; website: string; logoDataUrl: string
  currency: string; currencySymbol: string; decimals: number
  defaultTaxRate: number; pricesIncludeTax: boolean
  invoicePrefix: string; invoiceCounter: number
  purchasePrefix: string; purchaseCounter: number; movementCounter: number
  invoiceFooter: string; invoiceTerms: string
  fiscalYearStartMonth: number
  autoBackup: boolean; backupsToKeep: number
  sessionTimeoutMin: number; theme: string
  updatedAt: string
}

// --- Entrées ---------------------------------------------------------------

export interface ProductInput {
  id?: string; sku?: string; barcode?: string; name: string
  categoryId?: string; brand?: string; model?: string; description?: string
  unit?: string; purchasePrice: Money; salePrice: Money; minStock: number
  location?: string; warrantyMonths?: number; serialized?: boolean
  active: boolean; initialQuantity?: number
}

export interface PartyInput {
  id?: string; type: PartyType; name: string; company?: string
  phone?: string; email?: string; address?: string; city?: string
  taxId?: string; notes?: string; active: boolean
}

export interface InvoiceLineInput {
  productId: string; quantity: number
  unitPrice?: Money; discount?: Money
  // null ou absent : le taux par défaut s'applique. 0 : ligne exonérée.
  taxRate?: number | null
  serials?: string[]
}

export interface InvoiceInput {
  date?: string; lines: InvoiceLineInput[]
  customerId?: string; customerName?: string; customerPhone?: string
  customerAddress?: string; customerTaxId?: string
  globalDiscount?: Money; amountPaid?: Money
  paymentMethod?: PaymentMethod; notes?: string; draft?: boolean
}

export interface PurchaseLineInput {
  productId: string; quantity: number; unitCost: Money
  discount?: Money; taxRate?: number | null
}

export interface PurchaseInput {
  date?: string; supplierId?: string; reference?: string
  lines: PurchaseLineInput[]; otherCosts?: Money; notes?: string
  targetMarginPct?: number
}

export interface MovementInput {
  productId: string; quantity: number; date?: string
  reason?: string; notes?: string; partyId?: string
  invoiceId?: string; restock?: boolean
}

export interface InventoryInput {
  productId: string; countedSound: number; countedDefect: number
  reason: string; date?: string
}

export interface ExpenseInput {
  id?: string; date?: string; category: string; label: string
  amount: Money; paymentMethod?: PaymentMethod
  beneficiary?: string; notes?: string
}

export interface UserInput {
  id?: string; username?: string; fullName: string
  password?: string; role: Role
}

// --- Requêtes --------------------------------------------------------------

export interface ProductQuery {
  search?: string; categoryId?: string
  onlyLow?: boolean; onlyDefect?: boolean; includeArchived?: boolean
  sortBy?: string; sortDesc?: boolean
}
export interface InvoiceQuery {
  search?: string; customerId?: string; status?: string
  from?: string; to?: string; onlyUnpaid?: boolean; limit?: number
}
export interface PurchaseQuery {
  search?: string; supplierId?: string; status?: string
  from?: string; to?: string; limit?: number
}
export interface MovementQuery {
  search?: string; types?: MovementType[]; productId?: string
  from?: string; to?: string; limit?: number
}
export interface ExpenseQuery {
  search?: string; category?: string; from?: string; to?: string; limit?: number
}
export interface ReportQuery { from?: string; to?: string; granularity?: Granularity }
export interface AuditQuery {
  search?: string; action?: string; entity?: string
  userId?: string; from?: string; to?: string; limit?: number
}

// --- Résultats -------------------------------------------------------------

export interface StockSummary {
  productCount: number; totalUnits: number; defectiveUnits: number
  stockValueCost: Money; stockValueSale: Money; potentialMargin: Money
  lowStockCount: number; outOfStockCount: number; categoryCount: number
}

export interface ProductHistory {
  product: ProductView; movements: Movement[]
  totalIn: number; totalOut: number; totalSold: number
  revenue: Money; costOfSales: Money
}

export interface PeriodPoint {
  key: string; label: string
  revenueHT: Money; revenueTTC: Money; taxCollected: Money
  costOfSales: Money; grossMargin: Money; expenses: Money
  scrapLoss: Money; netResult: Money
  invoiceCount: number; unitsSold: number
  collected: Money; outstanding: Money; marginRate: number
}

export interface SalesReport {
  granularity: Granularity; from: string; to: string
  points: PeriodPoint[]; total: PeriodPoint; best?: PeriodPoint
  averagePerPeriod: Money; averageTicket: Money
}

export interface SnapshotKPI {
  value: Money; previous: Money; change: number; count: number
}

export interface ProductStat {
  productId: string; name: string; sku: string; category: string
  unitsSold: number; revenue: Money; margin: Money
  marginRate: number; stockLeft: number
}

export interface Dashboard {
  generatedAt: string
  todayRevenue: SnapshotKPI; monthRevenue: SnapshotKPI
  monthMargin: SnapshotKPI; monthExpenses: SnapshotKPI; monthResult: SnapshotKPI
  stockValue: Money; stockUnits: number; defectiveUnits: number
  outstanding: Money; outstandingCount: number
  last30Days: PeriodPoint[]; last12Months: PeriodPoint[]
  topProducts: ProductStat[]; lowStock: ProductView[]
  recentInvoices: Invoice[]; recentMovements: Movement[]
}

export interface ExpenseLine { category: string; amount: Money; share: number }

export interface IncomeStatement {
  from: string; to: string
  revenueHT: Money; taxCollected: Money; revenueTTC: Money; discountsGiven: Money
  costOfSales: Money; grossMargin: Money; marginRate: number
  expenseLines: ExpenseLine[]; totalExpenses: Money; scrapLoss: Money
  operatingResult: Money; resultRate: number
  invoiceCount: number; unitsSold: number; averageTicket: Money
  cashCollected: Money; purchasesPaid: Money; estimatedCashFlow: Money
}

export interface BalanceSheet {
  asOf: string
  stockValueSound: Money; stockValueDefective: Money
  receivables: Money; receivableCount: number; totalAssets: Money
  cumulativeRevenue: Money; cumulativeCostOfSales: Money
  cumulativeExpenses: Money; cumulativeScrapLoss: Money; cumulativeResult: Money
  taxCollected: Money
  stockUnits: number; defectiveUnits: number; productCount: number
}

export interface NamedStat {
  key: string; label: string; count: number; units: number
  amount: Money; margin: Money; share: number
}

export interface Statistics {
  from: string; to: string
  byCategory: NamedStat[]; byCustomer: NamedStat[]; byPayment: NamedStat[]
  byMovement: NamedStat[]; bySeller: NamedStat[]; byWeekday: NamedStat[]
  topProducts: ProductStat[]; slowProducts: ProductStat[]
}

export interface ExpenseBreakdown {
  category: string; amount: Money; count: number; share: number
}

export interface UserView extends PublicUser {
  invoiceCount: number; revenue: Money; scopes: Scope[]; isCurrent: boolean
}

export interface RoleInfo {
  role: Role; label: string; description: string; scopes: Scope[]
}

export interface BackupInfo {
  name: string; path: string; sizeBytes: number; createdAt: string
}

// Content est transmis en base64 par la sérialisation JSON de Go.
export interface FileResult { name: string; mime: string; content: string }

export type ImportAction = 'CREATE' | 'UPDATE' | 'SKIP'

export interface ImportRow {
  line: number; sku: string; name: string
  action: ImportAction; message: string; quantity: number
}

export interface ImportReport {
  applied: boolean
  rows: ImportRow[]
  created: number; updated: number; skipped: number
  categoriesCreated: string[]
  columns: string[]; ignored: string[]
}

export interface CurrencyPreset {
  code: string; symbol: string; decimals: number; label: string
}

export interface Presets {
  currencies: CurrencyPreset[]
  expenseCategories: string[]
  paymentMethods: { value: PaymentMethod; label: string }[]
}
