// Achats : réceptions de marchandise fournisseur.
//
// C'est le seul écran qui augmente le stock d'achat et recalcule le coût moyen
// pondéré. Les frais annexes saisis ici entrent dans le coût de revient, donc
// dans la marge : les oublier revient à surestimer les bénéfices.

import { useMemo, useState } from 'react'
import { Catalog, Config, Documents, Purchases, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDate, isoDate, statusLabel, statusTone } from '../lib/format'
import { formatNumber, formatPercent } from '../lib/money'
import {
  Alert, Badge, Card, DataTable, Empty, Field, Loading,
  Modal, MoneyInput, SearchInput, Select, TextArea, TextInput,
} from '../components/UI'
import { ProductPicker } from '../components/ProductPicker'
import { useDocumentPreview } from '../components/DocumentPreview'
import { IconClose, IconPlus, IconPrint } from '../components/Icons'
import type { ProductView, Purchase, PurchaseInput } from '../lib/types'
import type { PageContext } from '../App'

export function PurchasesPage({ refreshCounters }: PageContext) {
  const { money, amount } = useSession()
  const toast = useToast()
  const doc = useDocumentPreview()

  const [search, setSearch] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [creating, setCreating] = useState(false)
  const [detail, setDetail] = useState<Purchase | null>(null)

  const debounced = useDebounced(search)
  const query = useMemo(() => ({ search: debounced, from, to }), [debounced, from, to])
  const purchases = useAsync(() => Purchases.list(query), [query])

  const rows = purchases.data ?? []
  const total = rows.reduce((s, p) => s + (p.status === 'CANCELLED' ? 0 : p.total), 0)

  return (
    <div className="stack">
      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SearchInput value={search} onChange={setSearch} placeholder="Numéro, fournisseur, référence…" />
          <div className="row" style={{ gap: 6 }}>
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ width: 148 }} />
            <span className="muted small">au</span>
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ width: 148 }} />
          </div>
          <div className="spacer" />
          <button className="btn btn-primary" onClick={() => setCreating(true)}>
            <IconPlus />Nouvelle réception
          </button>
        </div>
      </Card>

      {purchases.error && <Alert tone="danger">{purchases.error}</Alert>}

      <Card flush>
        {purchases.loading ? <Loading /> : (
          <DataTable
            rows={rows}
            rowKey={(p) => p.id}
            onRowClick={setDetail}
            dimmed={(p) => p.status === 'CANCELLED'}
            empty={
              <Empty
                title="Aucune réception"
                text="Enregistrez une réception pour entrer de la marchandise en stock et mettre à jour le coût moyen."
                action={<button className="btn btn-primary" onClick={() => setCreating(true)}><IconPlus />Nouvelle réception</button>}
              />
            }
            columns={[
              { key: 'number', header: 'Numéro', width: 140, render: (p) => <span className="mono">{p.number}</span> },
              { key: 'date', header: 'Date', width: 100, render: (p) => formatDate(p.date) },
              {
                key: 'supplier', header: 'Fournisseur',
                render: (p) => (
                  <>
                    <div className="cell-primary">{p.supplierName || 'Non renseigné'}</div>
                    <div className="cell-secondary">
                      {p.lines.length} ligne{p.lines.length > 1 ? 's' : ''}
                      {p.reference && ` · réf. ${p.reference}`}
                    </div>
                  </>
                ),
              },
              { key: 'status', header: 'Statut', width: 110, render: (p) => <Badge tone={statusTone[p.status]}>{statusLabel[p.status]}</Badge> },
              { key: 'ht', header: 'Total HT', align: 'right', width: 120, render: (p) => amount(p.subtotalHT) },
              {
                key: 'other', header: 'Frais', align: 'right', width: 100,
                render: (p) => p.otherCosts > 0 ? amount(p.otherCosts) : <span className="muted">-</span>,
              },
              { key: 'total', header: 'Total', align: 'right', width: 120, render: (p) => <strong>{amount(p.total)}</strong> },
              {
                key: 'actions', header: '', align: 'right', width: 60,
                render: (p) => (
                  <button className="btn btn-sm btn-ghost" disabled={doc.busy}
                    onClick={(e) => { e.stopPropagation(); doc.open(`Bon d'entrée ${p.number}`, () => Documents.purchase(p.id)) }}>
                    <IconPrint />
                  </button>
                ),
              },
            ]}
          />
        )}
      </Card>

      {rows.length > 0 && (
        <div className="row small muted" style={{ justifyContent: 'flex-end', gap: 22 }}>
          <span>{rows.length} bon{rows.length > 1 ? 's' : ''} d'entrée</span>
          <span>Total acheté : <strong className="tabular">{money(total)}</strong></span>
        </div>
      )}

      {creating && (
        <NewPurchase
          onClose={() => setCreating(false)}
          onCreated={(purchase) => {
            setCreating(false)
            purchases.reload()
            refreshCounters()
            toast.success(`Bon d'entrée ${purchase.number} enregistré : le stock est à jour.`)
            setDetail(purchase)
          }}
        />
      )}

      {detail && (
        <PurchaseDetail
          purchase={detail}
          onClose={() => setDetail(null)}
          onCancelled={() => { setDetail(null); purchases.reload(); refreshCounters() }}
          onPrint={() => doc.open(`Bon d'entrée ${detail.number}`, () => Documents.purchase(detail.id))}
        />
      )}

      {doc.element}
    </div>
  )
}

// --- Saisie d'une réception ------------------------------------------------

interface Line {
  key: number
  product: ProductView
  quantity: number
  unitCost: number
  taxRate: number | null
}

let nextKey = 1

function NewPurchase(props: { onClose: () => void; onCreated: (p: Purchase) => void }) {
  const { money, amount, decimals, symbol } = useSession()
  const [lines, setLines] = useState<Line[]>([])
  const [supplierId, setSupplierId] = useState('')
  const [reference, setReference] = useState('')
  const [date, setDate] = useState(isoDate())
  const [otherCosts, setOtherCosts] = useState(0)
  const [targetMargin, setTargetMargin] = useState(0)
  const [notes, setNotes] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const suppliers = useAsync(() => Catalog.listParties('SUPPLIER', ''), [])
  const settings = useAsync(() => Config.get(), [])
  const defaultTax = settings.data?.defaultTaxRate ?? 0

  const totals = useMemo(() => {
    const subtotal = lines.reduce((s, l) => s + l.quantity * l.unitCost, 0)
    const tax = lines.reduce((s, l) => {
      const rate = l.taxRate === null ? defaultTax : l.taxRate
      return s + Math.round((l.quantity * l.unitCost * rate) / 100)
    }, 0)
    return { subtotal, tax, total: subtotal + tax + Math.max(0, otherCosts) }
  }, [lines, otherCosts, defaultTax])

  function addProduct(p: ProductView) {
    setLines((current) => [...current, {
      key: nextKey++, product: p, quantity: 1,
      unitCost: p.purchasePrice, taxRate: null,
    }])
  }

  function update(key: number, patch: Partial<Line>) {
    setLines((c) => c.map((l) => l.key === key ? { ...l, ...patch } : l))
  }

  async function save() {
    if (lines.length === 0) {
      setError('Ajoutez au moins une ligne.')
      return
    }
    setBusy(true)
    setError(null)
    const payload: PurchaseInput = {
      date,
      supplierId: supplierId || undefined,
      reference,
      otherCosts: Math.max(0, otherCosts),
      targetMarginPct: targetMargin,
      notes,
      lines: lines.map((l) => ({
        productId: l.product.id, quantity: l.quantity,
        unitCost: l.unitCost, taxRate: l.taxRate,
      })),
    }
    try {
      props.onCreated(await Purchases.create(payload))
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Nouvelle réception"
      subtitle="Le stock augmente et le coût moyen pondéré est recalculé à l'enregistrement."
      size="xwide"
      onClose={props.onClose}
      footer={
        <>
          <button className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          <button className="btn btn-primary" onClick={save} disabled={busy || lines.length === 0}>
            {busy ? 'Enregistrement…' : `Enregistrer ${money(totals.total)}`}
          </button>
        </>
      }
    >
      <div className="stack">
        {error && <Alert tone="danger">{error}</Alert>}

        <div className="grid grid-3">
          <Field label="Fournisseur">
            <Select
              value={supplierId} onChange={setSupplierId} placeholder="Non renseigné"
              options={(suppliers.data ?? []).filter((s) => s.active).map((s) => ({ value: s.id, label: s.name }))}
            />
          </Field>
          <Field label="Référence fournisseur" hint="N° de sa facture ou de son bon de livraison.">
            <TextInput value={reference} onChange={setReference} />
          </Field>
          <Field label="Date de réception">
            <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
          </Field>
        </div>

        <ProductPicker
          onPick={addProduct}
          includeArchived
          placeholder="Rechercher l'article reçu, ou scanner un code-barres…"
          meta={(p) => <span className="small muted">{formatNumber(p.quantity)} en stock</span>}
        />

        {lines.length === 0 ? (
          <div className="empty" style={{ padding: 28, border: '1px dashed var(--rule)', borderRadius: 'var(--radius)' }}>
            <div className="empty-title">Aucune ligne</div>
            <p className="empty-text">Cherchez un article ci-dessus pour l'ajouter à la réception.</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Article</th>
                  <th style={{ width: 90 }} className="num">Qté reçue</th>
                  <th style={{ width: 140 }} className="num">Coût unitaire</th>
                  <th style={{ width: 96 }} className="num">Taxe %</th>
                  <th style={{ width: 120 }} className="num">Total HT</th>
                  <th style={{ width: 40 }} />
                </tr>
              </thead>
              <tbody>
                {lines.map((l) => (
                  <tr key={l.key}>
                    <td>
                      <div className="cell-primary">{l.product.name}</div>
                      <div className="cell-secondary mono">
                        {l.product.sku} · coût actuel {amount(l.product.purchasePrice)}
                      </div>
                    </td>
                    <td className="num">
                      <input className="num" type="number" min={1} value={l.quantity}
                        onChange={(e) => update(l.key, { quantity: Math.max(1, parseInt(e.target.value, 10) || 1) })} />
                    </td>
                    <td className="num">
                      <MoneyInput value={l.unitCost} decimals={decimals}
                        onChange={(v) => update(l.key, { unitCost: v })} />
                    </td>
                    <td className="num">
                      <input className="num" type="number" min={0} max={100} step="0.5"
                        value={l.taxRate === null ? defaultTax : l.taxRate}
                        onChange={(e) => {
                          const v = parseFloat(e.target.value)
                          update(l.key, { taxRate: Number.isNaN(v) ? 0 : v })
                        }} />
                    </td>
                    <td className="num strong">{amount(l.quantity * l.unitCost)}</td>
                    <td className="num">
                      <button className="btn btn-sm btn-ghost"
                        onClick={() => setLines((c) => c.filter((x) => x.key !== l.key))} aria-label="Retirer">
                        <IconClose size={14} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <div className="row" style={{ alignItems: 'flex-start', gap: 20 }}>
          <div className="stack" style={{ flex: 1, gap: 12 }}>
            <Field
              label="Frais annexes"
              hint="Transport, douane, manutention. Répartis au prorata et intégrés au coût de revient."
            >
              <MoneyInput value={otherCosts} onChange={setOtherCosts} decimals={decimals} symbol={symbol} />
            </Field>
            <Field
              label="Marge cible sur le prix de vente"
              hint={targetMargin > 0
                ? `Les prix de vente seront recalculés : coût ÷ (1 − ${targetMargin} %).`
                : "Laissée à zéro, les prix de vente ne changent pas."}
            >
              <input className="num" type="number" min={0} max={99} step="1"
                value={targetMargin}
                onChange={(e) => setTargetMargin(Math.min(99, Math.max(0, parseFloat(e.target.value) || 0)))} />
            </Field>
            <Field label="Notes">
              <TextArea value={notes} onChange={setNotes} rows={2} />
            </Field>
          </div>

          <div style={{ width: 300 }}>
            <div className="section-title">Totaux</div>
            <div className="card" style={{ padding: 14 }}>
              <Row label="Sous-total HT" value={amount(totals.subtotal)} />
              <Row label="Taxes" value={amount(totals.tax)} />
              <Row label="Frais annexes" value={amount(Math.max(0, otherCosts))} />
              <div className="divider" style={{ margin: '10px 0' }} />
              <Row label="Total réception" value={money(totals.total)} strong />
            </div>
            {targetMargin > 0 && (
              <p className="small muted" style={{ marginTop: 10 }}>
                Prix de vente cible : marge de {formatPercent(targetMargin, 0)} sur le prix de vente.
              </p>
            )}
          </div>
        </div>
      </div>
    </Modal>
  )
}

// --- Détail ----------------------------------------------------------------

function PurchaseDetail(props: {
  purchase: Purchase
  onClose: () => void
  onCancelled: () => void
  onPrint: () => void
}) {
  const { money, amount } = useSession()
  const toast = useToast()
  const [cancelling, setCancelling] = useState(false)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const pu = props.purchase

  async function cancel() {
    setBusy(true)
    try {
      await Purchases.cancel(pu.id, reason)
      toast.success("Bon d'entrée annulé : la marchandise a été retirée du stock.")
      props.onCancelled()
    } catch (err) {
      toast.error(messageOf(err))
      setBusy(false)
    }
  }

  if (cancelling) {
    return (
      <Modal
        title="Annuler le bon d'entrée"
        subtitle={`${pu.number}, ${money(pu.total)}`}
        onClose={() => setCancelling(false)}
        onSubmit={cancel}
        footer={
          <>
            <button type="button" className="btn" onClick={() => setCancelling(false)} disabled={busy}>Revenir</button>
            <button type="submit" className="btn btn-danger" disabled={busy}>
              {busy ? 'Annulation…' : "Confirmer l'annulation"}
            </button>
          </>
        }
      >
        <div className="stack" style={{ gap: 14 }}>
          <Alert tone="warn">
            La marchandise sera retirée du stock. L'annulation est refusée si une partie a déjà été vendue.
            Le coût moyen n'est pas recalculé à rebours : il se réaligne à la prochaine entrée.
          </Alert>
          <Field label="Motif de l'annulation" required>
            <TextArea value={reason} onChange={setReason} rows={2} placeholder="Livraison non conforme, erreur de saisie…" />
          </Field>
        </div>
      </Modal>
    )
  }

  return (
    <Modal
      title={`Bon d'entrée ${pu.number}`}
      subtitle={`${pu.supplierName || 'Fournisseur non renseigné'}, ${formatDate(pu.date)}`}
      size="wide"
      onClose={props.onClose}
      footer={
        <>
          {pu.status !== 'CANCELLED' && (
            <button className="btn btn-danger" onClick={() => setCancelling(true)}>Annuler le bon</button>
          )}
          <div className="spacer" />
          <button className="btn" onClick={props.onPrint}><IconPrint />Imprimer</button>
          <button className="btn" onClick={props.onClose}>Fermer</button>
        </>
      }
    >
      <div className="stack">
        <div className="row" style={{ gap: 8 }}>
          <Badge tone={statusTone[pu.status]}>{statusLabel[pu.status]}</Badge>
          <Badge>Saisi par {pu.userName}</Badge>
          {pu.reference && <Badge>Réf. {pu.reference}</Badge>}
        </div>

        <DataTable
          rows={pu.lines}
          rowKey={(l) => l.productId + l.sku + l.unitCost}
          columns={[
            {
              key: 'product', header: 'Article',
              render: (l) => (
                <>
                  <div className="cell-primary">{l.productName}</div>
                  <div className="cell-secondary mono">{l.sku}</div>
                </>
              ),
            },
            { key: 'qty', header: 'Qté', align: 'right', width: 70, render: (l) => formatNumber(l.quantity) },
            { key: 'cost', header: 'Coût unitaire', align: 'right', width: 120, render: (l) => amount(l.unitCost) },
            { key: 'tax', header: 'Taxe', align: 'right', width: 70, render: (l) => `${l.taxRate} %` },
            { key: 'ht', header: 'Total HT', align: 'right', width: 120, render: (l) => <strong>{amount(l.lineHT)}</strong> },
          ]}
        />

        <div className="row" style={{ alignItems: 'flex-start', gap: 20 }}>
          <div style={{ flex: 1 }}>
            {pu.notes && (
              <>
                <div className="section-title">Notes</div>
                <p className="small" style={{ whiteSpace: 'pre-wrap' }} data-selectable>{pu.notes}</p>
              </>
            )}
          </div>
          <div style={{ width: 300 }}>
            <Row label="Sous-total HT" value={amount(pu.subtotalHT)} />
            <Row label="Taxes" value={amount(pu.taxTotal)} />
            {pu.otherCosts > 0 && <Row label="Frais annexes" value={amount(pu.otherCosts)} />}
            <div className="divider" style={{ margin: '8px 0' }} />
            <Row label="Total" value={money(pu.total)} strong />
          </div>
        </div>
      </div>
    </Modal>
  )
}

function Row(props: { label: string; value: string; strong?: boolean }) {
  return (
    <div className="ligne-total" style={{ padding: '4px 0' }}>
      <span className="ligne-total-libelle small">{props.label}</span>
      <span className="ligne-total-valeur" style={{
        fontWeight: props.strong ? 650 : 400,
        fontSize: props.strong ? 15 : 13,
      }}>{props.value}</span>
    </div>
  )
}
