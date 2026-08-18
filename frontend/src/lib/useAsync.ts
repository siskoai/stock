// Chargement de données : un état, une erreur, un rechargement.
//
// Le motif « charger / afficher / recharger après action » revient sur toutes
// les pages ; il est écrit une fois ici plutôt que douze fois ailleurs.

import { useCallback, useEffect, useState } from 'react'
import { messageOf } from './api'

export interface Async<T> {
  data: T | null
  loading: boolean
  error: string | null
  reload: () => void
}

export function useAsync<T>(loader: () => Promise<T>, deps: unknown[]): Async<T> {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [tick, setTick] = useState(0)

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const run = useCallback(loader, deps)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    run()
      .then((result) => { if (!cancelled) { setData(result); setError(null) } })
      .catch((err) => { if (!cancelled) setError(messageOf(err)) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [run, tick])

  return { data, loading, error, reload: () => setTick((t) => t + 1) }
}

/** useDebounced retarde la propagation d'une valeur : utile pour les recherches. */
export function useDebounced<T>(value: T, delay = 250): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay)
    return () => window.clearTimeout(timer)
  }, [value, delay])
  return debounced
}
