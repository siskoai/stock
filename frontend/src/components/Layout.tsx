// Cadre de l'application : barre latérale, en-tête, zone de contenu.

import { useState, type ReactNode } from 'react'
import { useSession } from '../lib/session'
import { roleLabel } from '../lib/format'
import type { Scope } from '../lib/types'
import {
  IconBox, IconChart, IconCart, IconDashboard, IconLayers,
  IconLogout, IconSettings, IconShield, IconTruck, IconUsers, IconWallet,
} from './Icons'
import { Account } from './Account'
import { CompanyLogo } from './CompanyLogo'

export type PageKey =
  | 'dashboard' | 'products' | 'categories' | 'stock' | 'sales' | 'creances'
  | 'purchases' | 'parties' | 'expenses' | 'reports' | 'settings' | 'users'

interface NavEntry {
  key: PageKey
  label: string
  icon: ReactNode
  /** Domaine requis. Absent : accessible à tout compte connecté. */
  scope?: Scope
  badge?: number
}

export function Layout(props: {
  page: PageKey
  onNavigate: (page: PageKey) => void
  title: string
  subtitle?: string
  actions?: ReactNode
  lowStockCount?: number
  unpaidCount?: number
  onLogout: () => void
  children: ReactNode
}) {
  const { state, can } = useSession()
  const [account, setAccount] = useState(false)

  const groups: { title: string; items: NavEntry[] }[] = [
    {
      title: 'Activité',
      items: [
        { key: 'dashboard', label: 'Tableau de bord', icon: <IconDashboard /> },
        { key: 'sales', label: 'Ventes', icon: <IconCart />, scope: 'sales' },
        { key: 'creances', label: 'Créances', icon: <IconWallet />, scope: 'sales', badge: props.unpaidCount },
        { key: 'purchases', label: 'Achats', icon: <IconTruck />, scope: 'purchases' },
      ],
    },
    {
      title: 'Marchandise',
      items: [
        { key: 'products', label: 'Articles', icon: <IconBox />, badge: props.lowStockCount },
        { key: 'categories', label: 'Catégories', icon: <IconLayers /> },
        { key: 'stock', label: 'Mouvements', icon: <IconRefreshNav /> },
      ],
    },
    {
      title: 'Gestion',
      items: [
        { key: 'parties', label: 'Clients & fournisseurs', icon: <IconUsers /> },
        { key: 'expenses', label: 'Charges', icon: <IconReceipt />, scope: 'expenses' },
        { key: 'reports', label: 'Rapports', icon: <IconChart />, scope: 'finance' },
      ],
    },
    {
      title: 'Administration',
      items: [
        { key: 'users', label: 'Comptes & journal', icon: <IconShield />, scope: 'users' },
        { key: 'settings', label: 'Paramètres', icon: <IconSettings />, scope: 'settings' },
      ],
    },
  ]

  const initials = (state.user?.fullName ?? '?')
    .split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('')

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="sidebar-head">
          <div className="sidebar-identity">
            <CompanyLogo className="sidebar-logo" />
            <div style={{ minWidth: 0 }}>
              <div className="sidebar-brand truncate" title={state.companyName}>
                {state.companyName}
              </div>
              <div className="sidebar-company">Comptoir</div>
            </div>
          </div>
        </div>

        <nav className="nav">
          {groups.map((group) => {
            const visible = group.items.filter((i) => !i.scope || can(i.scope))
            if (visible.length === 0) return null
            return (
              <div className="nav-group" key={group.title}>
                <div className="nav-group-title">{group.title}</div>
                {visible.map((item) => (
                  <button
                    key={item.key}
                    className={`nav-item ${props.page === item.key ? 'active' : ''}`}
                    onClick={() => props.onNavigate(item.key)}
                  >
                    {item.icon}
                    <span>{item.label}</span>
                    {!!item.badge && item.badge > 0 && <span className="nav-badge">{item.badge}</span>}
                  </button>
                ))}
              </div>
            )
          })}
        </nav>

        <div className="sidebar-foot">
          <div className="sidebar-user">
            <button
              className="nav-item"
              style={{ flex: 1, minWidth: 0, padding: 4, gap: 9 }}
              onClick={() => setAccount(true)}
              title="Mon compte"
            >
              <div className="avatar">{initials}</div>
              <div style={{ minWidth: 0, flex: 1, textAlign: 'left' }}>
                <div className="sidebar-user-name truncate">{state.user?.fullName}</div>
                <div className="sidebar-user-role">{state.user ? roleLabel[state.user.role] : ''}</div>
              </div>
            </button>
            <button className="nav-item" style={{ width: 'auto', padding: 6 }}
              onClick={props.onLogout} title="Se déconnecter" aria-label="Se déconnecter">
              <IconLogout />
            </button>
          </div>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div>
            <div className="topbar-title">{props.title}</div>
            {props.subtitle && <div className="topbar-sub">{props.subtitle}</div>}
          </div>
          {props.actions && <div className="topbar-actions">{props.actions}</div>}
        </header>
        <main className="content">{props.children}</main>
      </div>

      {account && <Account onClose={() => setAccount(false)} />}
    </div>
  )
}

// Les charges prennent une icône distincte de celle des créances : deux
// entrées voisines avec le même dessin se confondent.
function IconReceipt() {
  return (
    <svg width={17} height={17} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 3v18l2.5-1.6L10 21l2-1.6L14 21l2.5-1.6L19 21V3z" />
      <path d="M9 8h6M9 12h6" />
    </svg>
  )
}

// Petite icône propre au menu : deux flèches de mouvement.
function IconRefreshNav() {
  return (
    <svg width={17} height={17} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 8h13l-3-3M20 16H7l3 3" />
    </svg>
  )
}
