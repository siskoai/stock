// Recherche et sélection d'un article.
//
// C'est le geste le plus répété du logiciel : au comptoir, à la réception, à
// l'inventaire. Il est écrit une fois ici plutôt que trois fois ailleurs, et
// il est pensé pour le clavier autant que pour la souris.
//
// Une douchette code-barres se comporte comme un clavier très rapide suivi
// d'une touche Entrée. Comme la recherche est différée de quelques
// centièmes de seconde, la touche Entrée arrive souvent avant les résultats :
// la demande est alors mise en attente et satisfaite dès leur arrivée. Sans
// cela, scanner un article ne ferait rien une fois sur deux.

import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { Catalog } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { formatNumber } from '../lib/money'
import { Badge } from './UI'
import { IconSearch } from './Icons'
import type { ProductView } from '../lib/types'

const MAX_RESULTS = 8

export function ProductPicker(props: {
  onPick: (product: ProductView) => void
  placeholder?: string
  includeArchived?: boolean
  /** Écarte les articles inactifs des résultats (écran de vente). */
  onlyActive?: boolean
  autoFocus?: boolean
  meta?: (product: ProductView) => ReactNode
}) {
  const [search, setSearch] = useState('')
  const [highlight, setHighlight] = useState(0)
  const pending = useRef(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const debounced = useDebounced(search, 160)
  const query = debounced.trim()
  const results = useAsync(
    () => query === ''
      ? Promise.resolve([] as ProductView[])
      : Catalog.listProducts({ search: query, includeArchived: props.includeArchived }),
    [query, props.includeArchived],
  )

  const list = (results.data ?? [])
    .filter((p) => (props.onlyActive ? p.active : true))
    .slice(0, MAX_RESULTS)

  function pick(product: ProductView) {
    props.onPick(product)
    setSearch('')
    setHighlight(0)
    pending.current = false
    inputRef.current?.focus()
  }

  /**
   * exactMatch privilégie une correspondance stricte sur le code-barres ou la
   * référence : une douchette ne se trompe pas de code, l'ordre de pertinence
   * de la recherche textuelle, si.
   */
  function exactMatch(candidates: ProductView[], term: string): ProductView | undefined {
    const needle = term.trim().toUpperCase()
    return candidates.find((p) => p.barcode.toUpperCase() === needle)
      ?? candidates.find((p) => p.sku.toUpperCase() === needle)
  }

  // Satisfait une validation arrivée avant les résultats, le cas de la douchette.
  useEffect(() => {
    if (!pending.current || results.loading || query === '') return
    pending.current = false
    if (list.length === 0) return
    pick(exactMatch(list, query) ?? list[0])
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [results.loading, results.data, query])

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        setHighlight((h) => Math.min(h + 1, Math.max(0, list.length - 1)))
        break
      case 'ArrowUp':
        event.preventDefault()
        setHighlight((h) => Math.max(0, h - 1))
        break
      case 'Enter': {
        event.preventDefault()
        if (query === '') return
        if (results.loading || list.length === 0) {
          // Les résultats ne sont pas encore là : on retient l'intention.
          pending.current = true
          return
        }
        pick(exactMatch(list, query) ?? list[highlight] ?? list[0])
        break
      }
      case 'Escape':
        if (search !== '') {
          event.preventDefault()
          setSearch('')
          setHighlight(0)
        }
        break
    }
  }

  return (
    <div style={{ position: 'relative' }}>
      <div className="search" style={{ maxWidth: 'none' }}>
        <IconSearch />
        <input
          ref={inputRef}
          type="text"
          value={search}
          autoFocus={props.autoFocus}
          placeholder={props.placeholder ?? 'Rechercher un article, ou scanner un code-barres…'}
          onChange={(e) => { setSearch(e.target.value); setHighlight(0) }}
          onKeyDown={onKeyDown}
        />
      </div>

      {search.trim() !== '' && (
        <div style={{
          position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 20, marginTop: 4,
          background: 'var(--surface)', border: '1px solid var(--rule)',
          borderRadius: 'var(--radius-sm)', boxShadow: 'var(--shadow)', overflow: 'hidden',
        }}>
          {results.loading ? (
            <div className="small muted" style={{ padding: 12 }}>Recherche…</div>
          ) : list.length === 0 ? (
            <div className="small muted" style={{ padding: 12 }}>
              Aucun article ne correspond à « {search.trim()} ».
            </div>
          ) : (
            <>
              {list.map((p, i) => (
                <button
                  key={p.id}
                  type="button"
                  className="nav-item"
                  style={{
                    color: 'var(--ink)', borderRadius: 0, padding: '9px 12px',
                    background: i === highlight ? 'var(--wash)' : undefined,
                  }}
                  onMouseEnter={() => setHighlight(i)}
                  onClick={() => pick(p)}
                >
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div className="cell-primary truncate">{p.name}</div>
                    <div className="cell-secondary mono">
                      {p.sku}{p.categoryName ? ` · ${p.categoryName}` : ''}
                    </div>
                  </div>
                  {props.meta
                    ? props.meta(p)
                    : p.outOfStock
                      ? <Badge tone="red">Rupture</Badge>
                      : <Badge tone={p.low ? 'orange' : 'muted'}>{formatNumber(p.quantity)} en stock</Badge>}
                </button>
              ))}
              <div className="small muted" style={{ padding: '6px 12px', borderTop: '1px solid var(--rule)' }}>
                ↑ ↓ pour choisir · Entrée pour ajouter · Échap pour effacer
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}
