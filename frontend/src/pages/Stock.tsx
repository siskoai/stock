// Mouvements de stock : le journal, et les opérations qui l'alimentent en
// dehors des ventes et des achats, défauts, réparations, rebuts, retours et
// corrections d'inventaire.

import { useMemo, useState } from 'react'
import { Catalog, Documents, Export, Stock, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDate, isoDate, movementLabel, movementTone } from '../lib/format'
import { formatNumber } from '../lib/money'
import {
  Alert, Badge, Card, Checkbox, DataTable, Empty, Field, KPI,
  Loading, Modal, NumberInput, SearchInput, Select, TextArea,
} from '../components/UI'
import { ProductPicker } from '../components/ProductPicker'
import { useDocumentPreview } from '../components/DocumentPreview'
import { IconDownload, IconPrint } from '../components/Icons'
import type { MovementType, ProductView } from '../lib/types'
import type { PageContext } from '../App'

type Operation = 'defect' | 'repair' | 'scrap' | 'returnIn' | 'returnOut' | 'inventory'

const OPERATIONS: { key: Operation; label: string; description: string; scope: 'stock' }[] = [
  { key: 'defect', label: 'Déclarer défectueux', description: 'Sort du stock vendable vers le stock défectueux. Le total ne change pas, sa composition si.', scope: 'stock' },
  { key: 'repair', label: 'Remettre en vente', description: 'Un article défectueux réparé retourne au stock vendable.', scope: 'stock' },
  { key: 'scrap', label: 'Mettre au rebut', description: 'L\'article quitte définitivement le stock. Sa valeur devient une perte, reprise au compte de résultat.', scope: 'stock' },
  { key: 'returnIn', label: 'Retour client', description: 'La marchandise revient. Selon son état, elle rejoint le stock vendable ou le stock défectueux.', scope: 'stock' },
  { key: 'returnOut', label: 'Retour fournisseur', description: 'La marchandise repart. Le stock défectueux est prélevé en premier.', scope: 'stock' },
  { key: 'inventory', label: 'Corriger l\'inventaire', description: 'Aligne le stock théorique sur le comptage physique. L\'écart est tracé.', scope: 'stock' },
]

const TYPES: { value: MovementType; label: string }[] =
  (Object.keys(movementLabel) as MovementType[]).map((t) => ({ value: t, label: movementLabel[t] }))

export function StockPage({ refreshCounters }: PageContext) {
  const { money, amount, can } = useSession()
  const doc = useDocumentPreview()

  const [search, setSearch] = useState('')
  const [type, setType] = useState<string>('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [operation, setOperation] = useState<Operation | null>(null)

  const debounced = useDebounced(search)
  const query = useMemo(() => ({
    search: debounced,
    types: type ? [type as MovementType] : undefined,
    from, to, limit: 400,
  }), [debounced, type, from, to])

  const movements = useAsync(() => Stock.listMovements(query), [query])
  const summary = useAsync(() => Stock.summary(), [])

  function afterOperation() {
    setOperation(null)
    movements.reload()
    summary.reload()
    refreshCounters()
  }

  const sum = summary.data

  return (
    <div className="stack">
      {sum && (
        <div className={`grid ${can('finance') ? 'grid-4' : 'grid-3'}`}>
          <KPI label="Unités en stock" value={formatNumber(sum.totalUnits)}
            hint={`${sum.productCount} article(s) actif(s)`} />
          <KPI label="Défectueux" value={formatNumber(sum.defectiveUnits)}
            hint="isolés du stock vendable" accent={sum.defectiveUnits > 0} />
          <KPI label="À réapprovisionner" value={formatNumber(sum.lowStockCount + sum.outOfStockCount)}
            hint={`${sum.outOfStockCount} en rupture`} />
          {can('finance') && (
            <KPI label="Valeur du stock" value={money(sum.stockValueCost)}
              hint={`marge potentielle ${amount(sum.potentialMargin)}`} />
          )}
        </div>
      )}

      {can('stock') && (
        <Card title="Opérations de stock" note="Chaque opération laisse un mouvement daté et signé">
          <div className="grid grid-3" style={{ gap: 10 }}>
            {OPERATIONS.map((op) => (
              <button
                key={op.key}
                className="card"
                style={{ padding: 12, textAlign: 'left', cursor: 'pointer', font: 'inherit', color: 'inherit' }}
                onClick={() => setOperation(op.key)}
              >
                <div style={{ fontWeight: 600, fontSize: 13 }}>{op.label}</div>
                <div className="cell-secondary" style={{ marginTop: 3, lineHeight: 1.4 }}>{op.description}</div>
              </button>
            ))}
          </div>
        </Card>
      )}

      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SearchInput value={search} onChange={setSearch} placeholder="Article, tiers, document, motif…" />
          <div style={{ width: 180 }}>
            <Select value={type} onChange={setType} placeholder="Tous les types" options={TYPES} />
          </div>
          <div className="row" style={{ gap: 6 }}>
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ width: 148 }} />
            <span className="muted small">au</span>
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ width: 148 }} />
          </div>
          <div className="spacer" />
          <button className="btn" disabled={doc.busy} onClick={() => doc.download(() => Export.movements(query))}>
            <IconDownload />Exporter
          </button>
        </div>
      </Card>

      {movements.error && <Alert tone="danger">{movements.error}</Alert>}

      <Card flush>
        {movements.loading ? <Loading /> : (
          <DataTable
            rows={movements.data ?? []}
            rowKey={(m) => m.id}
            empty={<Empty title="Aucun mouvement" text="Les entrées, sorties et corrections apparaîtront ici." />}
            columns={[
              { key: 'ref', header: 'Référence', width: 130, render: (m) => <span className="mono small">{m.ref}</span> },
              { key: 'date', header: 'Date', width: 100, render: (m) => formatDate(m.date) },
              { key: 'type', header: 'Type', width: 150, render: (m) => <Badge tone={movementTone[m.type]}>{movementLabel[m.type]}</Badge> },
              {
                key: 'product', header: 'Article',
                render: (m) => (
                  <>
                    <div className="cell-primary">{m.productName}</div>
                    <div className="cell-secondary mono">{m.productSku}</div>
                  </>
                ),
              },
              { key: 'qty', header: 'Quantité', align: 'right', width: 90, render: (m) => formatNumber(m.quantity) },
              { key: 'after', header: 'Stock après', align: 'right', width: 100, render: (m) => <span className="muted">{formatNumber(m.stockAfter)}</span> },
              {
                key: 'reason', header: 'Motif / document',
                render: (m) => (
                  <>
                    <div className="truncate" style={{ maxWidth: 240 }}>{m.reason || '-'}</div>
                    {(m.documentNo || m.partyName) && (
                      <div className="cell-secondary truncate" style={{ maxWidth: 240 }}>
                        {[m.documentNo, m.partyName].filter(Boolean).join(' · ')}
                      </div>
                    )}
                  </>
                ),
              },
              { key: 'user', header: 'Opérateur', width: 130, render: (m) => <span className="small muted">{m.userName}</span> },
              {
                key: 'imprimer', header: '', align: 'right', width: 56,
                render: (m) => (
                  <button
                    className="btn btn-sm btn-ghost"
                    disabled={doc.busy}
                    title="Imprimer le bon de mouvement"
                    onClick={(e) => {
                      e.stopPropagation()
                      doc.open(`Bon de mouvement ${m.ref}`, () => Documents.movement(m.id))
                    }}
                  ><IconPrint /></button>
                ),
              },
            ]}
          />
        )}
      </Card>

      {operation && (
        <OperationModal
          operation={operation}
          onClose={() => setOperation(null)}
          onDone={afterOperation}
        />
      )}

      {doc.element}
    </div>
  )
}

// --- Opérations ------------------------------------------------------------

function OperationModal(props: { operation: Operation; onClose: () => void; onDone: () => void }) {
  const toast = useToast()
  const meta = OPERATIONS.find((o) => o.key === props.operation)!

  const [product, setProduct] = useState<ProductView | null>(null)
  const [quantity, setQuantity] = useState(1)
  const [countedSound, setCountedSound] = useState(0)
  const [countedDefect, setCountedDefect] = useState(0)
  const [reason, setReason] = useState('')
  const [notes, setNotes] = useState('')
  const [partyId, setPartyId] = useState('')
  const [restock, setRestock] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const isReturn = props.operation === 'returnIn' || props.operation === 'returnOut'
  const parties = useAsync(
    () => isReturn
      ? Catalog.listParties(props.operation === 'returnIn' ? 'CUSTOMER' : 'SUPPLIER', '')
      : Promise.resolve([]),
    [isReturn, props.operation],
  )

  function choose(p: ProductView) {
    setProduct(p)
    setCountedSound(p.quantity)
    setCountedDefect(p.defectiveQty)
  }

  async function submit() {
    if (!product) {
      setError('Choisissez un article.')
      return
    }
    setBusy(true)
    setError(null)
    const input = {
      productId: product.id, quantity, date: isoDate(),
      reason, notes, partyId: partyId || undefined, restock,
    }
    try {
      switch (props.operation) {
        case 'defect': await Stock.declareDefective(input); break
        case 'repair': await Stock.repairDefective(input); break
        case 'scrap': await Stock.scrapDefective(input); break
        case 'returnIn': await Stock.returnFromCustomer(input); break
        case 'returnOut': await Stock.returnToSupplier(input); break
        case 'inventory':
          await Stock.adjustInventory({
            productId: product.id, countedSound, countedDefect, reason, date: isoDate(),
          })
          break
      }
      toast.success('Mouvement enregistré.')
      props.onDone()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  const isInventory = props.operation === 'inventory'
  const reasonRequired = props.operation !== 'repair'

  return (
    <Modal
      title={meta.label}
      subtitle={meta.description}
      onClose={props.onClose}
      onSubmit={submit}
      footer={
        <>
          <button type="button" className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          <button type="submit" className="btn btn-primary" disabled={busy || !product}>
            {busy ? 'Enregistrement…' : 'Enregistrer le mouvement'}
          </button>
        </>
      }
    >
      <div className="stack" style={{ gap: 14 }}>
        {error && <Alert tone="danger">{error}</Alert>}

        {product ? (
          <div className="card" style={{ padding: 12 }}>
            <div className="row">
              <div style={{ flex: 1 }}>
                <div className="cell-primary">{product.name}</div>
                <div className="cell-secondary mono">{product.sku}</div>
              </div>
              <div className="row" style={{ gap: 6 }}>
                <Badge tone="muted">{formatNumber(product.quantity)} sain{product.quantity > 1 ? 's' : ''}</Badge>
                <Badge tone={product.defectiveQty > 0 ? 'red' : 'muted'}>
                  {formatNumber(product.defectiveQty)} défectueux
                </Badge>
              </div>
              <button type="button" className="btn btn-sm btn-ghost" onClick={() => setProduct(null)}>Changer</button>
            </div>
          </div>
        ) : (
          <Field label="Article" required>
            <ProductPicker
              autoFocus
              includeArchived
              onPick={choose}
              placeholder="Désignation, référence, code-barres…"
              meta={(p) => (
                <span className="small muted">
                  {formatNumber(p.quantity)} sain / {formatNumber(p.defectiveQty)} déf.
                </span>
              )}
            />
          </Field>
        )}

        {isInventory ? (
          <div className="grid grid-2">
            <Field label="Stock sain compté" required
              hint={product ? `Théorique : ${formatNumber(product.quantity)}` : undefined}>
              <NumberInput value={countedSound} onChange={setCountedSound} min={0} />
            </Field>
            <Field label="Stock défectueux compté" required
              hint={product ? `Théorique : ${formatNumber(product.defectiveQty)}` : undefined}>
              <NumberInput value={countedDefect} onChange={setCountedDefect} min={0} />
            </Field>
          </div>
        ) : (
          <Field label="Quantité" required>
            <NumberInput value={quantity} onChange={setQuantity} min={1} />
          </Field>
        )}

        {isReturn && (parties.data ?? []).length > 0 && (
          <Field label={props.operation === 'returnIn' ? 'Client' : 'Fournisseur'}>
            <Select
              value={partyId} onChange={setPartyId} placeholder="Non renseigné"
              options={(parties.data ?? []).map((p) => ({ value: p.id, label: p.name }))}
            />
          </Field>
        )}

        {props.operation === 'returnIn' && (
          <Checkbox
            checked={restock}
            onChange={setRestock}
            label="La marchandise est en état d'être revendue"
            hint="Décochée, elle rejoint le stock défectueux."
          />
        )}

        <Field
          label={isInventory ? "Motif de l'ajustement" : 'Motif'}
          required={reasonRequired}
          hint="Inscrit sur le mouvement et dans le journal d'audit."
        >
          <TextArea value={reason} onChange={setReason} rows={2}
            placeholder={isInventory ? 'Inventaire annuel, écart constaté au comptage…' : 'Écran cassé, panne au déballage…'} />
        </Field>

        {!isInventory && (
          <Field label="Notes">
            <TextArea value={notes} onChange={setNotes} rows={2} />
          </Field>
        )}
      </div>
    </Modal>
  )
}
