// Pont vers le backend Go.
//
// Wails expose les méthodes liées sous window.go.<paquet>.<Type>.<Méthode>.
// On les appelle directement plutôt que d'importer les liaisons générées :
// le build du frontend ne dépend alors d'aucun fichier produit par l'outil
// wails, et les types du contrat vivent en un seul endroit (lib/types.ts).

import type * as T from './types'

type Method = (...args: unknown[]) => Promise<unknown>
/** window.go est indexé par paquet Go, puis par type lié, puis par méthode. */
type Bindings = Record<string, Record<string, Record<string, Method>>>

declare global {
  interface Window {
    go?: Bindings
    runtime?: { EventsOn(name: string, cb: (...data: unknown[]) => void): void }
  }
}

/** Levée quand l'application tourne dans un navigateur, sans backend Wails. */
export class NoBackendError extends Error {
  constructor() {
    super("Le moteur de l'application n'est pas disponible. Lancez Comptoir depuis son exécutable, pas depuis un navigateur.")
    this.name = 'NoBackendError'
  }
}

export const hasBackend = () => typeof window !== 'undefined' && !!window.go

/**
 * bind résout une méthode liée par Wails.  s'écrit « paquet.Type » ; Wails
 * expose window.go[paquet][Type][méthode], d'où la décomposition.
 *
 * La résolution est faite à l'appel et non au chargement du module : les
 * liaisons sont injectées par le moteur après l'exécution du bundle.
 */
function bind<A extends unknown[], R>(ns: string, method: string) {
  const [pkg, type] = ns.split('.')
  return (...args: A): Promise<R> => {
    const target = window.go?.[pkg]?.[type]?.[method]
    if (typeof target !== 'function') return Promise.reject(new NoBackendError())
    return target(...args) as Promise<R>
  }
}

/**
 * Les erreurs remontées par Wails sont des chaînes rédigées en français côté
 * Go : on les présente telles quelles à l'utilisateur.
 */
export function messageOf(err: unknown): string {
  if (err instanceof Error) return err.message
  if (typeof err === 'string') return err
  return "Une erreur inattendue s'est produite."
}

// --- Session ---------------------------------------------------------------

export const Session = {
  state: bind<[], T.State>('services.Session', 'State'),
  setup: bind<[T.SetupInput], T.State>('services.Session', 'Setup'),
  brand: bind<[], T.Attribution>('services.Session', 'Brand'),
  currencies: bind<[], T.CurrencyPreset[]>('services.Session', 'Currencies'),
  defaultCategories: bind<[], string[]>('services.Session', 'DefaultCategoryNames'),
  login: bind<[string, string], T.State>('services.Session', 'Login'),
  logout: bind<[], T.State>('services.Session', 'Logout'),
  touch: bind<[], T.State>('services.Session', 'Touch'),
  changePassword: bind<[string, string], T.State>('services.Session', 'ChangePassword'),
}

// --- Catalogue -------------------------------------------------------------

export const Catalog = {
  listCategories: bind<[], T.CategoryView[]>('services.Catalog', 'ListCategories'),
  saveCategory: bind<[{ id?: string; name: string; description?: string; color?: string }], T.Category>('services.Catalog', 'SaveCategory'),
  deleteCategory: bind<[string], void>('services.Catalog', 'DeleteCategory'),

  listProducts: bind<[T.ProductQuery], T.ProductView[]>('services.Catalog', 'ListProducts'),
  getProduct: bind<[string], T.ProductView>('services.Catalog', 'GetProduct'),
  saveProduct: bind<[T.ProductInput], T.Product>('services.Catalog', 'SaveProduct'),
  archiveProduct: bind<[string, boolean], void>('services.Catalog', 'ArchiveProduct'),
  deleteProduct: bind<[string], void>('services.Catalog', 'DeleteProduct'),
  lowStock: bind<[], T.ProductView[]>('services.Catalog', 'LowStockProducts'),

  listParties: bind<[string, string], T.PartyView[]>('services.Catalog', 'ListParties'),

  importProducts: bind<[string, boolean], T.ImportReport>('services.Catalog', 'ImportProducts'),
  importTemplate: bind<[], T.FileResult>('services.Catalog', 'ImportTemplate'),
  saveParty: bind<[T.PartyInput], T.Party>('services.Catalog', 'SaveParty'),
  deleteParty: bind<[string], void>('services.Catalog', 'DeleteParty'),
}

// --- Stock -----------------------------------------------------------------

export const Stock = {
  listMovements: bind<[T.MovementQuery], T.Movement[]>('services.Stock', 'ListMovements'),
  summary: bind<[], T.StockSummary>('services.Stock', 'Summary'),
  history: bind<[string], T.ProductHistory>('services.Stock', 'History'),
  declareDefective: bind<[T.MovementInput], T.Movement>('services.Stock', 'DeclareDefective'),
  repairDefective: bind<[T.MovementInput], T.Movement>('services.Stock', 'RepairDefective'),
  scrapDefective: bind<[T.MovementInput], T.Movement>('services.Stock', 'ScrapDefective'),
  returnFromCustomer: bind<[T.MovementInput], T.Movement>('services.Stock', 'ReturnFromCustomer'),
  returnToSupplier: bind<[T.MovementInput], T.Movement>('services.Stock', 'ReturnToSupplier'),
  adjustInventory: bind<[T.InventoryInput], T.Movement>('services.Stock', 'AdjustInventory'),
}

// --- Ventes ----------------------------------------------------------------

export const Sales = {
  list: bind<[T.InvoiceQuery], T.Invoice[]>('services.Sales', 'ListInvoices'),
  get: bind<[string], T.Invoice>('services.Sales', 'GetInvoice'),
  create: bind<[T.InvoiceInput], T.Invoice>('services.Sales', 'CreateInvoice'),
  issueDraft: bind<[string], T.Invoice>('services.Sales', 'IssueDraft'),
  registerPayment: bind<[{ invoiceId: string; amount: number; method?: T.PaymentMethod; note?: string }], T.Invoice>('services.Sales', 'RegisterPayment'),
  cancel: bind<[string, string], void>('services.Sales', 'CancelInvoice'),
  deleteDraft: bind<[string], void>('services.Sales', 'DeleteDraft'),
}

// --- Achats ----------------------------------------------------------------

export const Purchases = {
  list: bind<[T.PurchaseQuery], T.Purchase[]>('services.Purchases', 'ListPurchases'),
  get: bind<[string], T.Purchase>('services.Purchases', 'GetPurchase'),
  create: bind<[T.PurchaseInput], T.Purchase>('services.Purchases', 'CreatePurchase'),
  cancel: bind<[string, string], void>('services.Purchases', 'CancelPurchase'),
}

// --- Charges ---------------------------------------------------------------

export const Expenses = {
  categories: bind<[], string[]>('services.Expenses', 'Categories'),
  list: bind<[T.ExpenseQuery], T.Expense[]>('services.Expenses', 'ListExpenses'),
  save: bind<[T.ExpenseInput], T.Expense>('services.Expenses', 'SaveExpense'),
  remove: bind<[string], void>('services.Expenses', 'DeleteExpense'),
  breakdown: bind<[string, string], T.ExpenseBreakdown[]>('services.Expenses', 'Breakdown'),
}

// --- Rapports --------------------------------------------------------------

export const Reports = {
  dashboard: bind<[], T.Dashboard>('services.Reports', 'Dashboard'),
  salesReport: bind<[T.ReportQuery], T.SalesReport>('services.Reports', 'SalesReport'),
  incomeStatement: bind<[string, string], T.IncomeStatement>('services.Reports', 'IncomeStatement'),
  balanceSheet: bind<[string], T.BalanceSheet>('services.Reports', 'BalanceSheet'),
  topProducts: bind<[T.ReportQuery, number], T.ProductStat[]>('services.Reports', 'TopProducts'),
  statistics: bind<[T.ReportQuery], T.Statistics>('services.Reports', 'Statistics'),
}

// --- Documents et exports --------------------------------------------------

export const Documents = {
  invoice: bind<[string], T.FileResult>('services.Documents', 'Invoice'),
  purchase: bind<[string], T.FileResult>('services.Documents', 'Purchase'),
  salesReport: bind<[T.ReportQuery], T.FileResult>('services.Documents', 'SalesReport'),
  incomeStatement: bind<[string, string], T.FileResult>('services.Documents', 'IncomeStatement'),
  stockReport: bind<[T.ProductQuery], T.FileResult>('services.Documents', 'StockReport'),
  partyStatement: bind<[string], T.FileResult>('services.Documents', 'PartyStatement'),
}

export const Export = {
  products: bind<[T.ProductQuery], T.FileResult>('services.Export', 'Products'),
  invoices: bind<[T.InvoiceQuery], T.FileResult>('services.Export', 'Invoices'),
  invoiceLines: bind<[T.InvoiceQuery], T.FileResult>('services.Export', 'InvoiceLines'),
  movements: bind<[T.MovementQuery], T.FileResult>('services.Export', 'Movements'),
  expenses: bind<[T.ExpenseQuery], T.FileResult>('services.Export', 'Expenses'),
  parties: bind<[string], T.FileResult>('services.Export', 'Parties'),
  salesReport: bind<[T.ReportQuery], T.FileResult>('services.Export', 'SalesReport'),
  save: bind<[T.FileResult], string>('services.Export', 'Save'),
}

// --- Comptes, paramètres, sauvegardes --------------------------------------

export const Users = {
  roles: bind<[], T.RoleInfo[]>('services.Users', 'Roles'),
  list: bind<[], T.UserView[]>('services.Users', 'List'),
  create: bind<[T.UserInput], T.UserView>('services.Users', 'Create'),
  update: bind<[T.UserInput], T.UserView>('services.Users', 'Update'),
  setActive: bind<[string, boolean], T.UserView>('services.Users', 'SetActive'),
  resetPassword: bind<[string, string], string>('services.Users', 'ResetPassword'),
  remove: bind<[string], void>('services.Users', 'Delete'),
  audit: bind<[T.AuditQuery], T.AuditEntry[]>('services.Users', 'Audit'),
  auditActions: bind<[], string[]>('services.Users', 'AuditActions'),
}

export const Config = {
  get: bind<[], T.Settings>('services.Config', 'Get'),
  save: bind<[T.Settings], T.Settings>('services.Config', 'Save'),
  presets: bind<[], T.Presets>('services.Config', 'Presets'),
  resetCounters: bind<[], T.Settings>('services.Config', 'ResetCounters'),
  dataLocation: bind<[], Record<string, string>>('services.Config', 'DataLocation'),
}

export const Backups = {
  list: bind<[], T.BackupInfo[]>('services.Backups', 'List'),
  create: bind<[string], T.BackupInfo>('services.Backups', 'Create'),
  remove: bind<[string], void>('services.Backups', 'Delete'),
  restore: bind<[string], void>('services.Backups', 'Restore'),
  restoreFromPath: bind<[string], void>('services.Backups', 'RestoreFromPath'),
}

// --- Système ---------------------------------------------------------------

export const Host = {
  saveFile: bind<[string, string], string>('main.App', 'SaveFile'),
  pickBackupArchive: bind<[], string>('main.App', 'PickBackupArchive'),
  pickLogo: bind<[], string>('main.App', 'PickLogo'),
  pickCatalogFile: bind<[], string>('main.App', 'PickCatalogFile'),
  openDataFolder: bind<[string], void>('main.App', 'OpenDataFolder'),
  quit: bind<[], void>('main.App', 'Quit'),
}

/**
 * saveDocument propose l'enregistrement d'un fichier produit par le backend.
 * Renvoie le chemin retenu, ou null si l'utilisateur a annulé.
 *
 * Le contenu fait l'aller-retour en base64 : c'est la contrepartie d'un
 * frontend qui affiche d'abord le document avant de proposer de l'enregistrer.
 */
export async function saveDocument(file: T.FileResult): Promise<string | null> {
  const path = await Host.saveFile(file.name, file.content)
  return path === '' ? null : path
}

/** dataURL construit l'URL d'aperçu d'un document renvoyé par le backend. */
export function dataURL(file: T.FileResult): string {
  return `data:${file.mime};base64,${file.content}`
}
