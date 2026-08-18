// Catalogue : liste, fiche article, historique.

import { useEffect, useMemo, useState } from 'react'
import { Catalog, Documents, Export, Stock, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDate, movementLabel, movementTone } from '../lib/format'
import { formatNumber, formatPercent } from '../lib/money'
import {
  Alert, Badge, Card, Checkbox, ConfirmDialog, DataTable, Empty, Field,
  Loading, Modal, MoneyInput, NumberInput, SearchInput, Select, TextArea, TextInput,
} from '../components/UI'
import { useDocumentPreview } from '../components/DocumentPreview'
import { ImportCatalog } from '../components/ImportCatalog'
import { IconDownload, IconPlus, IconPrint } from '../components/Icons'
import type { CategoryView, ProductInput, ProductQuery, ProductView } from '../lib/types'
import type { PageContext } from '../App'

export function ProductsPage({ refreshCounters, arg }: PageContext) {
  const { money, amount, can, decimals, symbol } = useSession()
  const toast = useToast()
  const doc = useDocumentPreview()

  const [search, setSearch] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [onlyLow, setOnlyLow] = useState(false)
  const [includeArchived, setIncludeArchived] = useState(false)
  const [sortBy, setSortBy] = useState('name')
  const [sortDesc, setSortDesc] = useState(false)
  const [editing, setEditing] = useState<ProductView | 'new' | null>(null)
  const [historyOf, setHistoryOf] = useState<string | null>(arg ?? null)
  const [importing, setImporting] = useState(false)

  const debounced = useDebounced(search)
  const query = useMemo<ProductQuery>(
    () => ({ search: debounced, categoryId, onlyLow, includeArchived, sortBy, sortDesc }),
    [debounced, categoryId, onlyLow, includeArchived, sortBy, sortDesc],
  )

  const products = useAsync(() => Catalog.listProducts(query), [query])
  const categories = useAsync(() => Catalog.listCategories(), [])

  useEffect(() => { if (arg) setHistoryOf(arg) }, [arg])

  function sort(key: string) {
    if (key === sortBy) setSortDesc(!sortDesc)
    else { setSortBy(key); setSortDesc(key !== 'name' && key !== 'sku') }
  }

  function afterChange() {
    products.reload()
    categories.reload()
    refreshCounters()
  }

  const rows = products.data ?? []
  const showCost = can('finance')

  return (
    <div className="stack">
      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SearchInput value={search} onChange={setSearch} placeholder="Référence, désignation, marque, code-barres…" />
          <div style={{ width: 190 }}>
            <Select
              value={categoryId}
              onChange={setCategoryId}
              placeholder="Toutes les catégories"
              options={(categories.data ?? []).map((c: CategoryView) => ({ value: c.id, label: c.name }))}
            />
          </div>
          <Checkbox checked={onlyLow} onChange={setOnlyLow} label="Sous le seuil" />
          <Checkbox checked={includeArchived} onChange={setIncludeArchived} label="Avec les archivés" />
          <div className="spacer" />
          <button className="btn" disabled={doc.busy}
            onClick={() => doc.open("État du stock", () => Documents.stockReport(query))}>
            <IconPrint />Imprimer
          </button>
          <button className="btn" disabled={doc.busy} onClick={() => doc.download(() => Export.products(query))}>
            <IconDownload />Exporter
          </button>
          {can('catalog') && (
            <>
              <button className="btn" onClick={() => setImporting(true)}>Importer…</button>
              <button className="btn btn-primary" onClick={() => setEditing('new')}>
                <IconPlus />Nouvel article
              </button>
            </>
          )}
        </div>
      </Card>

      {products.error && <Alert tone="danger">{products.error}</Alert>}

      <Card flush>
        {products.loading ? <Loading /> : (
          <DataTable
            rows={rows}
            rowKey={(p) => p.id}
            onRowClick={(p) => setHistoryOf(p.id)}
            dimmed={(p) => !p.active}
            sortBy={sortBy}
            sortDesc={sortDesc}
            onSort={sort}
            empty={
              <Empty
                title="Aucun article"
                text={search || categoryId || onlyLow
                  ? "Aucun article ne correspond à cette recherche."
                  : "Le catalogue est vide. Créez un premier article pour commencer."}
                action={can('catalog') && !search
                  ? <button className="btn btn-primary" onClick={() => setEditing('new')}><IconPlus />Nouvel article</button>
                  : undefined}
              />
            }
            columns={[
              {
                key: 'sku', header: 'Référence', sortable: true, width: 130,
                render: (p) => <span className="mono">{p.sku}</span>,
              },
              {
                key: 'name', header: 'Désignation', sortable: true,
                render: (p) => (
                  <>
                    <div className="cell-primary">{p.name}</div>
                    <div className="cell-secondary">
                      {[p.brand, p.model].filter(Boolean).join(' ') || p.categoryName}
                      {!p.active && ' · archivé'}
                    </div>
                  </>
                ),
              },
              {
                key: 'category', header: 'Catégorie', width: 150,
                render: (p) => <Badge tone={p.categoryColor}>{p.categoryName}</Badge>,
              },
              {
                key: 'quantity', header: 'Stock', sortable: true, align: 'right', width: 110,
                render: (p) => (
                  <>
                    <div className={p.outOfStock ? 'neg strong' : p.low ? 'strong' : ''}>
                      {formatNumber(p.quantity)}
                    </div>
                    {p.defectiveQty > 0 && <div className="cell-secondary">{p.defectiveQty} déf.</div>}
                  </>
                ),
              },
              {
                key: 'price', header: 'Prix de vente', sortable: true, align: 'right', width: 120,
                render: (p) => amount(p.salePrice),
              },
              ...(showCost ? [
                {
                  key: 'margin', header: 'Marge', sortable: true, align: 'right' as const, width: 110,
                  render: (p: ProductView) => (
                    <>
                      <div>{amount(p.marginAmount)}</div>
                      <div className="cell-secondary">{formatPercent(p.marginRate)}</div>
                    </>
                  ),
                },
                {
                  key: 'value', header: 'Valeur', sortable: true, align: 'right' as const, width: 110,
                  render: (p: ProductView) => amount(p.stockValue),
                },
              ] : []),
              {
                key: 'actions', header: '', align: 'right', width: 90,
                render: (p) => can('catalog') ? (
                  <button className="btn btn-sm btn-ghost"
                    onClick={(e) => { e.stopPropagation(); setEditing(p) }}>Modifier</button>
                ) : null,
              },
            ]}
          />
        )}
      </Card>

      {rows.length > 0 && showCost && (
        <div className="row small muted" style={{ justifyContent: 'flex-end', gap: 22 }}>
          <span>{rows.length} article{rows.length > 1 ? 's' : ''}</span>
          <span>Valeur totale : <strong className="tabular">{money(rows.reduce((s, p) => s + p.stockValue, 0))}</strong></span>
        </div>
      )}

      {editing && (
        <ProductModal
          product={editing === 'new' ? null : editing}
          categories={categories.data ?? []}
          decimals={decimals}
          symbol={symbol}
          canDelete={can('delete')}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); afterChange(); toast.success('Article enregistré.') }}
        />
      )}

      {historyOf && (
        <HistoryModal productId={historyOf} onClose={() => setHistoryOf(null)} />
      )}

      {importing && (
        <ImportCatalog
          onClose={() => setImporting(false)}
          onImported={() => { setImporting(false); afterChange() }}
        />
      )}

      {doc.element}
    </div>
  )
}

// --- Fiche article ---------------------------------------------------------

const emptyProduct: ProductInput = {
  name: '', sku: '', barcode: '', categoryId: '', brand: '', model: '',
  description: '', unit: 'pièce', purchasePrice: 0, salePrice: 0, minStock: 0,
  location: '', warrantyMonths: 0, serialized: false, active: true, initialQuantity: 0,
}

function ProductModal(props: {
  product: ProductView | null
  categories: CategoryView[]
  decimals: number
  symbol: string
  canDelete: boolean
  onClose: () => void
  onSaved: () => void
}) {
  const toast = useToast()
  const isNew = props.product === null
  const [form, setForm] = useState<ProductInput>(() =>
    props.product
      ? { ...props.product, initialQuantity: 0 }
      : { ...emptyProduct })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const set = <K extends keyof ProductInput>(key: K, value: ProductInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  // La marge se lit pendant la saisie : c'est le chiffre qui décide du prix.
  const margin = form.salePrice - form.purchasePrice
  const marginRate = form.salePrice > 0 ? (margin / form.salePrice) * 100 : 0

  async function save() {
    setBusy(true)
    setError(null)
    try {
      await Catalog.saveProduct(form)
      props.onSaved()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    setBusy(true)
    try {
      await Catalog.deleteProduct(props.product!.id)
      toast.success('Article supprimé.')
      props.onSaved()
    } catch (err) {
      setError(messageOf(err))
      setConfirmDelete(false)
    } finally {
      setBusy(false)
    }
  }

  async function archive() {
    setBusy(true)
    try {
      await Catalog.archiveProduct(props.product!.id, props.product!.active)
      toast.success(props.product!.active ? 'Article archivé.' : 'Article réactivé.')
      props.onSaved()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  if (confirmDelete) {
    return (
      <ConfirmDialog
        title="Supprimer définitivement ?"
        danger
        busy={busy}
        confirmLabel="Supprimer"
        message={<>
          <p>« {props.product?.name} » sera effacé du catalogue.</p>
          <p style={{ marginTop: 8 }} className="muted">
            La suppression est refusée si l'article a du stock ou apparaît dans un mouvement :
            l'historique comptable doit rester intact. Dans ce cas, archivez-le.
          </p>
        </>}
        onConfirm={remove}
        onCancel={() => setConfirmDelete(false)}
      />
    )
  }

  return (
    <Modal
      title={isNew ? 'Nouvel article' : form.name}
      subtitle={isNew ? undefined : `Référence ${props.product?.sku}`}
      size="wide"
      onClose={props.onClose}
      onSubmit={save}
      footer={
        <>
          {!isNew && (
            <>
              <button type="button" className="btn" onClick={archive} disabled={busy}>
                {props.product?.active ? 'Archiver' : 'Réactiver'}
              </button>
              {props.canDelete && (
                <button type="button" className="btn btn-danger" onClick={() => setConfirmDelete(true)} disabled={busy}>
                  Supprimer
                </button>
              )}
            </>
          )}
          <div className="spacer" />
          <button type="button" className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? 'Enregistrement…' : 'Enregistrer'}
          </button>
        </>
      }
    >
      <div className="stack" style={{ gap: 16 }}>
        {error && <Alert tone="danger">{error}</Alert>}

        <div className="grid grid-2">
          <Field label="Désignation" required>
            <TextInput value={form.name} onChange={(v) => set('name', v)} autoFocus />
          </Field>
          <Field label="Catégorie">
            <Select
              value={form.categoryId ?? ''}
              onChange={(v) => set('categoryId', v)}
              placeholder="Sans catégorie"
              options={props.categories.map((c) => ({ value: c.id, label: c.name }))}
            />
          </Field>
        </div>

        <div className="grid grid-3">
          <Field label="Référence (SKU)" hint={isNew ? 'Laissée vide, elle est générée.' : undefined}>
            <TextInput value={form.sku ?? ''} onChange={(v) => set('sku', v)} />
          </Field>
          <Field label="Code-barres">
            <TextInput value={form.barcode ?? ''} onChange={(v) => set('barcode', v)} />
          </Field>
          <Field label="Unité">
            <TextInput value={form.unit ?? ''} onChange={(v) => set('unit', v)} placeholder="pièce" />
          </Field>
        </div>

        <div className="grid grid-2">
          <Field label="Marque"><TextInput value={form.brand ?? ''} onChange={(v) => set('brand', v)} /></Field>
          <Field label="Modèle"><TextInput value={form.model ?? ''} onChange={(v) => set('model', v)} /></Field>
        </div>

        <div className="divider" />

        <div className="grid grid-3">
          <Field
            label="Coût d'achat"
            hint={isNew ? undefined : 'Recalculé à chaque réception (coût moyen pondéré).'}
          >
            <MoneyInput value={form.purchasePrice} onChange={(v) => set('purchasePrice', v)}
              decimals={props.decimals} symbol={props.symbol} />
          </Field>
          <Field label="Prix de vente" required>
            <MoneyInput value={form.salePrice} onChange={(v) => set('salePrice', v)}
              decimals={props.decimals} symbol={props.symbol} />
          </Field>
          <Field label="Marge" hint="Calculée sur le prix de vente.">
            <div style={{
              padding: '8px 10px', borderRadius: 7, background: 'var(--wash)',
              textAlign: 'right', fontVariantNumeric: 'tabular-nums',
              color: margin < 0 ? 'var(--red)' : 'var(--ink)',
            }}>
              {formatPercent(marginRate)}
            </div>
          </Field>
        </div>

        <div className="grid grid-3">
          {isNew && (
            <Field label="Stock initial" hint="Inscrit comme mouvement d'inventaire.">
              <NumberInput value={form.initialQuantity ?? 0} onChange={(v) => set('initialQuantity', v)} min={0} />
            </Field>
          )}
          <Field label="Seuil d'alerte" hint="0 : pas d'alerte.">
            <NumberInput value={form.minStock} onChange={(v) => set('minStock', v)} min={0} />
          </Field>
          <Field label="Emplacement">
            <TextInput value={form.location ?? ''} onChange={(v) => set('location', v)} placeholder="Rayon A2" />
          </Field>
          <Field label="Garantie (mois)">
            <NumberInput value={form.warrantyMonths ?? 0} onChange={(v) => set('warrantyMonths', v)} min={0} />
          </Field>
        </div>

        <Field label="Description">
          <TextArea value={form.description ?? ''} onChange={(v) => set('description', v)} rows={2} />
        </Field>

        <div className="grid grid-2">
          <Checkbox
            checked={form.serialized ?? false}
            onChange={(v) => set('serialized', v)}
            label="Suivi par numéro de série"
            hint="Les numéros seront saisis sur les lignes de facture."
          />
          {!isNew && (
            <Checkbox
              checked={form.active}
              onChange={(v) => set('active', v)}
              label="Article actif"
              hint="Un article inactif n'apparaît plus à la vente."
            />
          )}
        </div>
      </div>
    </Modal>
  )
}

// --- Historique ------------------------------------------------------------

function HistoryModal({ productId, onClose }: { productId: string; onClose: () => void }) {
  const { amount, can } = useSession()
  const { data, loading, error } = useAsync(() => Stock.history(productId), [productId])

  return (
    <Modal
      title={data?.product.name ?? 'Fiche de vie'}
      subtitle={data ? `Référence ${data.product.sku}, ${data.movements.length} mouvement(s)` : undefined}
      size="xwide"
      onClose={onClose}
      footer={<button className="btn" onClick={onClose}>Fermer</button>}
    >
      {loading ? <Loading /> : error ? <Alert tone="danger">{error}</Alert> : data && (
        <div className="stack">
          <div className={`grid ${can('finance') ? 'grid-4' : 'grid-3'}`}>
            <div className="card kpi">
              <div className="kpi-label">En stock</div>
              <div className="kpi-value">{formatNumber(data.product.quantity)}</div>
              <div className="kpi-hint">{data.product.defectiveQty} défectueux</div>
            </div>
            <div className="card kpi">
              <div className="kpi-label">Vendus</div>
              <div className="kpi-value">{formatNumber(data.totalSold)}</div>
              <div className="kpi-hint">{formatNumber(data.totalIn)} entrés au total</div>
            </div>
            <div className="card kpi">
              <div className="kpi-label">Chiffre d'affaires</div>
              <div className="kpi-value">{amount(data.revenue)}</div>
            </div>
            {can('finance') && (
              <div className="card kpi">
                <div className="kpi-label">Marge dégagée</div>
                <div className="kpi-value">{amount(data.revenue - data.costOfSales)}</div>
              </div>
            )}
          </div>

          <DataTable
            rows={data.movements}
            rowKey={(m) => m.id}
            empty={<Empty title="Aucun mouvement" />}
            columns={[
              { key: 'ref', header: 'Référence', render: (m) => <span className="mono small">{m.ref}</span> },
              { key: 'date', header: 'Date', render: (m) => <span className="small">{formatDate(m.date)}</span> },
              { key: 'type', header: 'Type', render: (m) => <Badge tone={movementTone[m.type]}>{movementLabel[m.type]}</Badge> },
              { key: 'qty', header: 'Quantité', align: 'right', render: (m) => formatNumber(m.quantity) },
              { key: 'after', header: 'Stock après', align: 'right', render: (m) => <span className="muted">{formatNumber(m.stockAfter)}</span> },
              {
                key: 'reason', header: 'Motif',
                render: (m) => (
                  <>
                    <div className="truncate" style={{ maxWidth: 260 }}>{m.reason || '-'}</div>
                    {m.documentNo && <div className="cell-secondary mono">{m.documentNo}</div>}
                  </>
                ),
              },
              { key: 'user', header: 'Opérateur', render: (m) => <span className="small muted">{m.userName}</span> },
            ]}
          />
        </div>
      )}
    </Modal>
  )
}
