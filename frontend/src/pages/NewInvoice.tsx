// Saisie d'une vente.
//
// L'écran est fait pour le comptoir : on cherche un article, on l'ajoute, on
// encaisse. Les totaux affichés reproduisent le calcul du backend, mais ce
// dernier recalcule tout à l'enregistrement, ce qui est montré ici n'est
// qu'une aide à la saisie, jamais la source des montants enregistrés.

import { useMemo, useRef, useState } from 'react'
import { Catalog, Config, Sales, messageOf } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { isoDate, paymentLabel } from '../lib/format'
import { formatNumber } from '../lib/money'
import {
  Alert, Checkbox, Field, Modal, MoneyInput, Select, TextArea, TextInput,
} from '../components/UI'
import { ProductPicker } from '../components/ProductPicker'
import { IconClose, IconPlus } from '../components/Icons'
import type { Invoice, InvoiceInput, PaymentMethod, ProductView } from '../lib/types'

interface Line {
  key: number
  product: ProductView
  quantity: number
  unitPrice: number
  discount: number
  taxRate: number | null
  serials: string[]
}

const PAYMENTS: { value: PaymentMethod; label: string }[] =
  (['CASH', 'MOBILE', 'TRANSFER', 'CHECK', 'CREDIT'] as PaymentMethod[])
    .map((v) => ({ value: v, label: paymentLabel[v] }))

let nextKey = 1

export function NewInvoice(props: { onClose: () => void; onCreated: (invoice: Invoice) => void }) {
  const { money, amount, decimals, symbol } = useSession()

  const [lines, setLines] = useState<Line[]>([])
  const [customerId, setCustomerId] = useState('')
  const [customerName, setCustomerName] = useState('')
  const [customerPhone, setCustomerPhone] = useState('')
  const [date, setDate] = useState(isoDate())
  const [globalDiscount, setGlobalDiscount] = useState(0)
  const [amountPaid, setAmountPaid] = useState(0)
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>('CASH')
  const [notes, setNotes] = useState('')
  const [draft, setDraft] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const paidTouched = useRef(false)

  const customers = useAsync(() => Catalog.listParties('CUSTOMER', ''), [])
  const settings = useAsync(() => Config.get(), [])
  const defaultTax = settings.data?.defaultTaxRate ?? 0

  // Totaux d'aide à la saisie : même règle que le backend, la remise globale
  // réduit la base taxable au prorata de chaque ligne.
  const totals = useMemo(() => {
    let subtotal = 0
    const bases: number[] = []
    for (const l of lines) {
      const gross = l.quantity * l.unitPrice
      const net = Math.max(0, gross - Math.min(l.discount, gross))
      bases.push(net)
      subtotal += net
    }
    const discount = Math.min(Math.max(0, globalDiscount), subtotal)
    let tax = 0
    lines.forEach((l, i) => {
      const cut = subtotal > 0 ? Math.round((discount * bases[i]) / subtotal) : 0
      const rate = l.taxRate === null ? defaultTax : l.taxRate
      tax += Math.round(((bases[i] - cut) * rate) / 100)
    })
    return { subtotal, discount, tax, total: subtotal - discount + tax }
  }, [lines, globalDiscount, defaultTax])

  function addProduct(p: ProductView) {
    setLines((current) => {
      const existing = current.find((l) => l.product.id === p.id)
      if (existing) {
        return current.map((l) => l === existing ? { ...l, quantity: l.quantity + 1 } : l)
      }
      return [...current, {
        key: nextKey++, product: p, quantity: 1,
        unitPrice: p.salePrice, discount: 0, taxRate: null, serials: [],
      }]
    })
  }

  function update(key: number, patch: Partial<Line>) {
    setLines((current) => current.map((l) => l.key === key ? { ...l, ...patch } : l))
  }

  function remove(key: number) {
    setLines((current) => current.filter((l) => l.key !== key))
  }

  // Tant que l'utilisateur n'a pas saisi de montant reçu, le champ suit le
  // total : le cas courant est un règlement comptant intégral.
  const effectivePaid = paidTouched.current ? amountPaid : (draft ? 0 : totals.total)

  async function save() {
    if (lines.length === 0) {
      setError('Ajoutez au moins un article.')
      return
    }
    setBusy(true)
    setError(null)
    const payload: InvoiceInput = {
      date,
      draft,
      customerId: customerId || undefined,
      customerName: customerId ? undefined : customerName,
      customerPhone: customerId ? undefined : customerPhone,
      globalDiscount: totals.discount,
      amountPaid: draft ? 0 : effectivePaid,
      paymentMethod,
      notes,
      lines: lines.map((l) => ({
        productId: l.product.id,
        quantity: l.quantity,
        unitPrice: l.unitPrice,
        discount: l.discount,
        taxRate: l.taxRate,
        serials: l.serials.filter((s) => s.trim() !== ''),
      })),
    }
    try {
      props.onCreated(await Sales.create(payload))
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={draft ? 'Nouveau devis' : 'Nouvelle vente'}
      subtitle="Le stock est déduit à l'enregistrement, sauf pour un devis."
      size="xwide"
      onClose={props.onClose}
      footer={
        <>
          <Checkbox
            checked={draft}
            onChange={setDraft}
            label="Enregistrer comme devis"
            hint="Rien n'est déduit du stock tant qu'il n'est pas confirmé."
          />
          <div className="spacer" />
          <button className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          <button className="btn btn-primary" onClick={save} disabled={busy || lines.length === 0}>
            {busy ? 'Enregistrement…' : draft ? 'Enregistrer le devis' : `Encaisser ${money(totals.total)}`}
          </button>
        </>
      }
    >
      <div className="stack">
        {error && <Alert tone="danger">{error}</Alert>}

        {/* Recherche d'article : le champ garde le focus après chaque ajout,
            pour enchaîner les scans sans toucher la souris. */}
        <ProductPicker
          autoFocus
          onlyActive
          onPick={addProduct}
          placeholder="Rechercher un article, ou scanner un code-barres…"
        />

        {/* Lignes */}
        {lines.length === 0 ? (
          <div className="empty" style={{ padding: 28, border: '1px dashed var(--rule)', borderRadius: 'var(--radius)' }}>
            <div className="empty-title">Aucun article</div>
            <p className="empty-text">Cherchez un article ci-dessus pour l'ajouter à la vente.</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Article</th>
                  <th style={{ width: 86 }} className="num">Qté</th>
                  <th style={{ width: 130 }} className="num">PU HT</th>
                  <th style={{ width: 120 }} className="num">Remise</th>
                  <th style={{ width: 96 }} className="num">Taxe %</th>
                  <th style={{ width: 110 }} className="num">Total HT</th>
                  <th style={{ width: 40 }} />
                </tr>
              </thead>
              <tbody>
                {lines.map((l) => {
                  const gross = l.quantity * l.unitPrice
                  const net = Math.max(0, gross - Math.min(l.discount, gross))
                  const short = !draft && l.quantity > l.product.quantity
                  return (
                    <tr key={l.key}>
                      <td>
                        <div className="cell-primary">{l.product.name}</div>
                        <div className="cell-secondary mono">
                          {l.product.sku} · {formatNumber(l.product.quantity)} en stock
                        </div>
                        {short && (
                          <div className="cell-secondary neg">
                            Stock insuffisant : {formatNumber(l.product.quantity)} disponible(s)
                          </div>
                        )}
                        {l.product.serialized && (
                          <input
                            style={{ marginTop: 5, fontSize: 12 }}
                            placeholder={`${l.quantity} numéro(s) de série, séparés par une virgule`}
                            value={l.serials.join(', ')}
                            onChange={(e) => update(l.key, { serials: e.target.value.split(',').map((s) => s.trim()) })}
                          />
                        )}
                      </td>
                      <td className="num">
                        <input
                          className="num" type="number" min={1} value={l.quantity}
                          onChange={(e) => update(l.key, { quantity: Math.max(1, parseInt(e.target.value, 10) || 1) })}
                        />
                      </td>
                      <td className="num">
                        <MoneyInput value={l.unitPrice} decimals={decimals}
                          onChange={(v) => update(l.key, { unitPrice: v })} />
                      </td>
                      <td className="num">
                        <MoneyInput value={l.discount} decimals={decimals}
                          onChange={(v) => update(l.key, { discount: v })} />
                      </td>
                      <td className="num">
                        <input
                          className="num" type="number" min={0} max={100} step="0.5"
                          value={l.taxRate === null ? defaultTax : l.taxRate}
                          onChange={(e) => {
                            const v = parseFloat(e.target.value)
                            update(l.key, { taxRate: Number.isNaN(v) ? 0 : v })
                          }}
                        />
                      </td>
                      <td className="num strong">{amount(net)}</td>
                      <td className="num">
                        <button className="btn btn-sm btn-ghost" onClick={() => remove(l.key)} aria-label="Retirer">
                          <IconClose size={14} />
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Client, règlement, totaux */}
        <div className="row" style={{ alignItems: 'flex-start', gap: 20 }}>
          <div className="stack" style={{ flex: 1, gap: 12 }}>
            <div className="section-title">Client</div>
            <Field label="Client enregistré" hint="Laissez vide pour une vente au comptoir.">
              <Select
                value={customerId}
                onChange={setCustomerId}
                placeholder="Client comptoir"
                options={(customers.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.name }))}
              />
            </Field>
            {!customerId && (
              <div className="grid grid-2">
                <Field label="Nom (facultatif)">
                  <TextInput value={customerName} onChange={setCustomerName} placeholder="Client comptoir" />
                </Field>
                <Field label="Téléphone">
                  <TextInput value={customerPhone} onChange={setCustomerPhone} />
                </Field>
              </div>
            )}
            <div className="grid grid-2">
              <Field label="Date">
                <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
              </Field>
              <Field label="Mode de règlement">
                <Select value={paymentMethod} onChange={setPaymentMethod} options={PAYMENTS} />
              </Field>
            </div>
            <Field label="Notes">
              <TextArea value={notes} onChange={setNotes} rows={2} placeholder="Conditions particulières, garantie…" />
            </Field>
          </div>

          <div style={{ width: 320 }}>
            <div className="section-title">Totaux</div>
            <div className="card" style={{ padding: 14 }}>
              <Row label="Sous-total HT" value={amount(totals.subtotal)} />
              <Field label="Remise globale">
                <MoneyInput value={globalDiscount} onChange={setGlobalDiscount}
                  decimals={decimals} symbol={symbol} />
              </Field>
              <Row label="Taxes" value={amount(totals.tax)} />
              <div className="divider" style={{ margin: '10px 0' }} />
              <Row label="Net à payer" value={money(totals.total)} strong />

              {!draft && (
                <div style={{ marginTop: 12 }}>
                  <Field label="Montant reçu">
                    <MoneyInput
                      value={effectivePaid}
                      onChange={(v) => { paidTouched.current = true; setAmountPaid(v) }}
                      decimals={decimals}
                      symbol={symbol}
                    />
                  </Field>
                  <Row
                    label={effectivePaid >= totals.total ? 'À rendre' : 'Reste dû'}
                    value={amount(Math.abs(totals.total - effectivePaid))}
                    accent={effectivePaid < totals.total}
                  />
                </div>
              )}
            </div>

            {lines.length > 0 && !draft && (
              <p className="small muted" style={{ marginTop: 10 }}>
                <IconPlus size={12} /> {lines.length} ligne{lines.length > 1 ? 's' : ''} ·{' '}
                {formatNumber(lines.reduce((s, l) => s + l.quantity, 0))} article(s) sortiront du stock.
              </p>
            )}
          </div>
        </div>
      </div>
    </Modal>
  )
}

function Row(props: { label: string; value: string; strong?: boolean; accent?: boolean }) {
  return (
    <div className="ligne-total" style={{ padding: '4px 0' }}>
      <span className="ligne-total-libelle small">{props.label}</span>
      <span className="ligne-total-valeur" style={{
        fontWeight: props.strong ? 650 : 400,
        fontSize: props.strong ? 15 : 13,
        color: props.accent ? 'var(--accent)' : undefined,
      }}>{props.value}</span>
    </div>
  )
}
