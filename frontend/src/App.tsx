// Aiguillage de l'application.
//
// Pas de routeur : l'application est un poste de travail, pas un site. Une
// page à la fois, sélectionnée dans la barre latérale, l'état vit ici.

import { useCallback, useEffect, useState } from 'react'
import { Catalog, Sales, Session, hasBackend, messageOf } from './lib/api'
import { useSession } from './lib/session'
import { useToast } from './lib/toast'
import { Layout, type PageKey } from './components/Layout'
import { Alert, Loading } from './components/UI'
import { ChangePasswordGate, LoginPage } from './pages/Gate'
import { Onboarding } from './pages/Onboarding'
import { DashboardPage } from './pages/Dashboard'
import { ProductsPage } from './pages/Products'
import { CategoriesPage } from './pages/Categories'
import { StockPage } from './pages/Stock'
import { SalesPage } from './pages/Sales'
import { CreancesPage } from './pages/Creances'
import { PurchasesPage } from './pages/Purchases'
import { PartiesPage } from './pages/Parties'
import { ExpensesPage } from './pages/Expenses'
import { ReportsPage } from './pages/Reports'
import { SettingsPage } from './pages/Settings'
import { UsersPage } from './pages/Users'

/** Ce que chaque page reçoit du cadre. */
export interface PageContext {
  /** Signale au cadre que les compteurs de la barre latérale ont bougé. */
  refreshCounters: () => void
  navigate: (page: PageKey, arg?: string) => void
  /** Argument de navigation : identifiant d'article, de client… */
  arg?: string
}

const titles: Record<PageKey, { title: string; subtitle: string }> = {
  dashboard: { title: 'Tableau de bord', subtitle: "L'activité de la boutique en un coup d'œil" },
  products: { title: 'Articles', subtitle: 'Catalogue et état du stock' },
  categories: { title: 'Catégories', subtitle: 'Classement des articles' },
  stock: { title: 'Mouvements de stock', subtitle: 'Journal des entrées, sorties et corrections' },
  sales: { title: 'Ventes', subtitle: 'Factures, devis et règlements' },
  creances: { title: 'Créances', subtitle: 'Ce que les clients doivent, et depuis quand' },
  purchases: { title: 'Achats', subtitle: 'Réceptions de marchandise fournisseur' },
  parties: { title: 'Clients et fournisseurs', subtitle: 'Coordonnées et encours' },
  expenses: { title: 'Charges', subtitle: "Dépenses d'exploitation, hors achat de marchandise" },
  reports: { title: 'Rapports', subtitle: 'Résultat, statistiques et exports' },
  settings: { title: 'Paramètres', subtitle: 'Identité de la boutique, monnaie, sauvegardes' },
  users: { title: 'Comptes et journal', subtitle: "Accès au poste et traçabilité des actions" },
}

export function App() {
  const { state, loading, error, refresh, setState } = useSession()
  const toast = useToast()
  const [page, setPage] = useState<PageKey>('dashboard')
  const [arg, setArg] = useState<string | undefined>()
  const [lowStock, setLowStock] = useState(0)
  const [unpaid, setUnpaid] = useState(0)

  const navigate = useCallback((next: PageKey, nextArg?: string) => {
    setPage(next)
    setArg(nextArg)
  }, [])

  const refreshCounters = useCallback(() => {
    if (!state.authenticated || state.user?.mustChangePwd) return
    Catalog.lowStock().then((list) => setLowStock(list.length)).catch(() => {})
    Sales.list({ onlyUnpaid: true }).then((list) => setUnpaid(list.length)).catch(() => {})
  }, [state.authenticated, state.user?.mustChangePwd])

  useEffect(() => { refreshCounters() }, [refreshCounters])

  const logout = useCallback(async () => {
    try {
      setState(await Session.logout())
      setPage('dashboard')
    } catch (err) {
      toast.error(messageOf(err))
    }
  }, [setState, toast])

  if (!hasBackend()) {
    return (
      <div className="gate">
        <div className="gate-card">
          <div className="gate-mark">C</div>
          <h1 className="gate-title">Moteur indisponible</h1>
          <p className="gate-sub">
            Cette interface doit être ouverte depuis l'application Comptoir, pas depuis un navigateur.
          </p>
        </div>
      </div>
    )
  }

  if (loading) return <div className="gate"><Loading label="Ouverture de la boutique…" /></div>

  if (error && !state.authenticated) {
    return (
      <div className="gate">
        <div className="gate-card">
          <div className="gate-mark">C</div>
          <h1 className="gate-title">Comptoir n'a pas pu s'ouvrir</h1>
          <p className="gate-sub">{error}</p>
          <button className="btn btn-primary btn-block" onClick={() => refresh().catch(() => {})}>
            Réessayer
          </button>
        </div>
      </div>
    )
  }

  if (state.needsSetup) return <Onboarding />
  if (!state.authenticated) return <LoginPage />
  if (state.user?.mustChangePwd) return <ChangePasswordGate />

  const ctx: PageContext = { refreshCounters, navigate, arg }
  const meta = titles[page]

  return (
    <Layout
      page={page}
      onNavigate={(next) => navigate(next)}
      title={meta.title}
      subtitle={meta.subtitle}
      lowStockCount={lowStock}
      unpaidCount={unpaid}
      onLogout={logout}
    >
      <PageBody page={page} ctx={ctx} />
    </Layout>
  )
}

function PageBody({ page, ctx }: { page: PageKey; ctx: PageContext }) {
  switch (page) {
    case 'dashboard': return <DashboardPage {...ctx} />
    case 'products': return <ProductsPage {...ctx} />
    case 'categories': return <CategoriesPage {...ctx} />
    case 'stock': return <StockPage {...ctx} />
    case 'sales': return <SalesPage {...ctx} />
    case 'creances': return <CreancesPage {...ctx} />
    case 'purchases': return <PurchasesPage {...ctx} />
    case 'parties': return <PartiesPage {...ctx} />
    case 'expenses': return <ExpensesPage {...ctx} />
    case 'reports': return <ReportsPage {...ctx} />
    case 'settings': return <SettingsPage {...ctx} />
    case 'users': return <UsersPage {...ctx} />
    default: return <Alert tone="warn">Page inconnue.</Alert>
  }
}
