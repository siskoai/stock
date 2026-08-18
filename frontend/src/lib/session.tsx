// Contexte de session : l'état renvoyé par le backend, partagé par toute
// l'interface, et les aides qui en découlent (droits, formatage monétaire).

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Session as SessionAPI, messageOf } from './api'
import { formatMoney, formatWithSymbol } from './money'
import type { Scope, State } from './types'

interface SessionValue {
  state: State
  loading: boolean
  error: string | null
  /** refresh relit l'état auprès du backend (session, société, monnaie). */
  refresh: () => Promise<State>
  setState: (s: State) => void
  /** can indique si le rôle courant couvre un domaine. */
  can: (scope: Scope) => boolean
  /** money formate un montant avec le symbole de la boutique. */
  money: (amount: number) => string
  /** amount formate un montant sans le symbole (colonnes de tableau). */
  amount: (value: number) => string
  symbol: string
  decimals: number
}

const empty: State = {
  needsSetup: false, authenticated: false, scopes: [],
  companyName: 'Comptoir', currencySymbol: 'FCFA', decimals: 0,
  theme: 'light', appVersion: '',
  author: 'SISKO', notice: 'Édité avec Comptoir — un logiciel SISKO',
  brandingIntact: true,
}

const Ctx = createContext<SessionValue | null>(null)

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>(empty)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const next = await SessionAPI.state()
      setState(next)
      setError(null)
      return next
    } catch (err) {
      setError(messageOf(err))
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { refresh().catch(() => {}) }, [refresh])

  // Le thème est appliqué à la racine du document : le CSS ne connaît que
  // l'attribut, jamais l'état React.
  useEffect(() => {
    const theme = state.theme === 'system'
      ? (window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
      : state.theme || 'light'
    document.documentElement.dataset.theme = theme
  }, [state.theme])

  // Une session expirée doit se voir sans attendre l'action suivante : on
  // interroge le backend à intervalle régulier tant qu'une session est ouverte.
  useEffect(() => {
    if (!state.authenticated) return
    const timer = window.setInterval(() => {
      SessionAPI.touch().then(setState).catch(() => {})
    }, 60_000)
    return () => window.clearInterval(timer)
  }, [state.authenticated])

  const value = useMemo<SessionValue>(() => ({
    state, loading, error, refresh, setState,
    can: (scope) => state.scopes.includes(scope),
    money: (a) => formatWithSymbol(a, state.decimals, state.currencySymbol),
    amount: (a) => formatMoney(a, state.decimals),
    symbol: state.currencySymbol,
    decimals: state.decimals,
  }), [state, loading, error, refresh])

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useSession(): SessionValue {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useSession doit être utilisé dans un SessionProvider')
  return ctx
}
