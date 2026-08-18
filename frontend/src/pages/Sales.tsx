// Ventes : liste des factures, saisie d'une vente, règlements, impression.

import { useEffect, useMemo, useState } from 'react'
import { Documents, Export, Sales, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDate, formatDateTime, paymentLabel, statusLabel, statusTone } from '../lib/format'
import { formatNumber } from '../lib/money'
import {
  Alert, Badge, Card, ConfirmDialog, DataTable, Empty, Field,
  Loading, Modal, MoneyInput, SearchInput, Select, TextArea, TextInput,
} from '../components/UI'
import { useDocumentPreview } from '../components/DocumentPreview'
import { IconDownload, IconPlus, IconPrint, IconTrash } from '../components/Icons'
import { NewInvoice } from './NewInvoice'
import type { Invoice, InvoiceQuery, PaymentMethod } from '../lib/types'
import type { PageContext } from '../App'

const STATUSES = [
  { value: '', label: 'Tous les statuts' },
  { value: 'DRAFT', label: 'Devis' },
  { value: 'ISSUED', label: 'Émises' },
  { value: 'PARTIAL', label: 'Partiellement réglées' },
  { value: 'PAID', label: 'Réglées' },
  { value: 'CANCELLED', label: 'Annulées' },
]

export function SalesPage({ refreshCounters, arg }: PageContext) {
  const { money, amount, can } = useSession()
  const toast = useToast()
  const doc = useDocumentPreview()

  const [search, setSearch] = useState('')
  const [customerId, setCustomerId] = useState('')
  const [status, setStatus] = useState('')
  const [onlyUnpaid, setOnlyUnpaid] = useState(false)
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [creating, setCreating] = useState(false)
  const [detail, setDetail] = useState<Invoice | null>(null)

  const debounced = useDebounced(search)
  const query = useMemo<InvoiceQuery>(
    () => ({ search: debounced, customerId, status, onlyUnpaid, from, to }),
    [debounced, customerId, status, onlyUnpaid, from, to],
  )
  const invoices = useAsync(() => Sales.list(query), [query])

  // Une navigation venue d'ailleurs porte soit un identifiant de facture, la
  // page l'ouvre, soit « client:<id> », qui filtre la liste sur ce client.
  useEffect(() => {
    if (!arg) return
    if (arg.startsWith('client:')) {
      setCustomerId(arg.slice('client:'.length))
      return
    }
    setCustomerId('')
    Sales.get(arg).then(setDetail).catch(() => setSearch(arg))
  }, [arg])

  const rows = invoices.data ?? []
  const totals = useMemo(() => ({
    count: rows.length,
    total: rows.reduce((s, i) => s + (i.status === 'CANCELLED' ? 0 : i.total), 0),
    due: rows.reduce((s, i) => s + i.balance, 0),
  }), [rows])

  function afterChange() {
    invoices.reload()
    refreshCounters()
  }

  return (
    <div className="stack">
      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SearchInput value={search} onChange={setSearch} placeholder="Numéro, client, article, n° de série…" />
          <div style={{ width: 190 }}>
            <Select value={status} onChange={setStatus} options={STATUSES} />
          </div>
          <button
            className={`btn ${onlyUnpaid ? 'btn-primary' : ''}`}
            onClick={() => setOnlyUnpaid(!onlyUnpaid)}
          >Impayées</button>
          {customerId && (
            <button className="btn btn-primary" onClick={() => setCustomerId('')}>
              Client filtré · retirer
            </button>
          )}
          <div className="row" style={{ gap: 6 }}>
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ width: 148 }} />
            <span className="muted small">au</span>
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ width: 148 }} />
          </div>
          <div className="spacer" />
          <button className="btn" disabled={doc.busy} onClick={() => doc.download(() => Export.invoiceLines(query))}>
            <IconDownload />Exporter
          </button>
          <button className="btn btn-primary" onClick={() => setCreating(true)}>
            <IconPlus />Nouvelle vente
          </button>
        </div>
      </Card>

      {invoices.error && <Alert tone="danger">{invoices.error}</Alert>}

      <Card flush>
        {invoices.loading ? <Loading /> : (
          <DataTable
            rows={rows}
            rowKey={(i) => i.id}
            onRowClick={setDetail}
            dimmed={(i) => i.status === 'CANCELLED'}
            empty={
              <Empty
                title="Aucune facture"
                text={search || status || onlyUnpaid
                  ? 'Aucune facture ne correspond à ces critères.'
                  : 'Enregistrez votre première vente : le stock se met à jour tout seul.'}
                action={!search
                  ? <button className="btn btn-primary" onClick={() => setCreating(true)}><IconPlus />Nouvelle vente</button>
                  : undefined}
              />
            }
            columns={[
              {
                key: 'number', header: 'Numéro', width: 140,
                render: (i) => <span className="mono">{i.number}</span>,
              },
              { key: 'date', header: 'Date', width: 100, render: (i) => formatDate(i.date) },
              {
                key: 'customer', header: 'Client',
                render: (i) => (
                  <>
                    <div className="cell-primary">{i.customerName}</div>
                    <div className="cell-secondary">
                      {i.lines.length} ligne{i.lines.length > 1 ? 's' : ''}
                      {i.customerPhone && ` · ${i.customerPhone}`}
                    </div>
                  </>
                ),
              },
              { key: 'status', header: 'Statut', width: 150, render: (i) => <Badge tone={statusTone[i.status]}>{statusLabel[i.status]}</Badge> },
              { key: 'total', header: 'Total', align: 'right', width: 120, render: (i) => <strong>{amount(i.total)}</strong> },
              {
                key: 'balance', header: 'Reste dû', align: 'right', width: 110,
                render: (i) => i.balance > 0
                  ? <span style={{ color: 'var(--accent)', fontWeight: 600 }}>{amount(i.balance)}</span>
                  : <span className="muted">-</span>,
              },
              {
                key: 'actions', header: '', align: 'right', width: 100,
                render: (i) => (
                  <button className="btn btn-sm btn-ghost" disabled={doc.busy}
                    onClick={(e) => {
                      e.stopPropagation()
                      doc.open(i.status === 'DRAFT' ? `Devis ${i.number}` : `Facture ${i.number}`,
                        () => Documents.invoice(i.id))
                    }}><IconPrint /></button>
                ),
              },
            ]}
          />
        )}
      </Card>

      {totals.count > 0 && (
        <div className="row small muted" style={{ justifyContent: 'flex-end', gap: 22 }}>
          <span>{totals.count} document{totals.count > 1 ? 's' : ''}</span>
          <span>Total : <strong className="tabular">{money(totals.total)}</strong></span>
          {totals.due > 0 && (
            <span>Reste dû : <strong className="tabular" style={{ color: 'var(--accent)' }}>{money(totals.due)}</strong></span>
          )}
        </div>
      )}

      {creating && (
        <NewInvoice
          onClose={() => setCreating(false)}
          onCreated={(invoice) => {
            setCreating(false)
            afterChange()
            toast.success(`${invoice.status === 'DRAFT' ? 'Devis' : 'Facture'} ${invoice.number} enregistré.`)
            setDetail(invoice)
          }}
        />
      )}

      {detail && (
        <InvoiceDetail
          invoice={detail}
          canSeeCost={can('finance')}
          onClose={() => setDetail(null)}
          onChanged={(next) => { setDetail(next); afterChange() }}
          onPrint={() => doc.open(
            detail.status === 'DRAFT' ? `Devis ${detail.number}` : `Facture ${detail.number}`,
            () => Documents.invoice(detail.id),
          )}
        />
      )}

      {doc.element}
    </div>
  )
}

// --- Détail d'une facture --------------------------------------------------

function InvoiceDetail(props: {
  invoice: Invoice
  canSeeCost: boolean
  onClose: () => void
  onChanged: (next: Invoice | null) => void
  onPrint: () => void
}) {
  const { money, amount } = useSession()
  const toast = useToast()
  const inv = props.invoice
  const [action, setAction] = useState<'none' | 'pay' | 'cancel' | 'delete'>('none')
  const [busy, setBusy] = useState(false)

  async function issue() {
    setBusy(true)
    try {
      props.onChanged(await Sales.issueDraft(inv.id))
      toast.success('Devis converti en facture : le stock a été mis à jour.')
    } catch (err) {
      toast.error(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  async function removeDraft() {
    setBusy(true)
    try {
      await Sales.deleteDraft(inv.id)
      toast.success('Devis supprimé.')
      props.onChanged(null)
      props.onClose()
    } catch (err) {
      toast.error(messageOf(err))
    } finally {
      setBusy(false)
      setAction('none')
    }
  }

  if (action === 'pay') {
    return <PaymentModal invoice={inv} onClose={() => setAction('none')}
      onPaid={(next) => { setAction('none'); props.onChanged(next) }} />
  }
  if (action === 'cancel') {
    return <CancelModal invoice={inv} onClose={() => setAction('none')}
      onCancelled={async () => {
        setAction('none')
        props.onChanged(await Sales.get(inv.id))
      }} />
  }
  if (action === 'delete') {
    return (
      <ConfirmDialog
        title="Supprimer le devis ?"
        danger busy={busy} confirmLabel="Supprimer"
        message={`Le devis ${inv.number} sera effacé. Aucun stock n'a été engagé, rien d'autre ne change.`}
        onConfirm={removeDraft}
        onCancel={() => setAction('none')}
      />
    )
  }

  return (
    <Modal
      title={`${inv.status === 'DRAFT' ? 'Devis' : 'Facture'} ${inv.number}`}
      subtitle={`${inv.customerName}, ${formatDate(inv.date)}`}
      size="wide"
      onClose={props.onClose}
      footer={
        <>
          {inv.status === 'DRAFT' && (
            <>
              <button className="btn btn-danger" onClick={() => setAction('delete')} disabled={busy}>
                <IconTrash />Supprimer
              </button>
              <button className="btn btn-primary" onClick={issue} disabled={busy}>
                {busy ? 'Émission…' : 'Émettre la facture'}
              </button>
            </>
          )}
          {inv.balance > 0 && inv.status !== 'DRAFT' && inv.status !== 'CANCELLED' && (
            <button className="btn btn-primary" onClick={() => setAction('pay')}>Enregistrer un règlement</button>
          )}
          {inv.status !== 'CANCELLED' && inv.status !== 'DRAFT' && (
            <button className="btn btn-danger" onClick={() => setAction('cancel')}>Annuler la facture</button>
          )}
          <div className="spacer" />
          <button className="btn" onClick={props.onPrint}><IconPrint />Imprimer</button>
          <button className="btn" onClick={props.onClose}>Fermer</button>
        </>
      }
    >
      <div className="stack">
        <div className="row row-wrap" style={{ gap: 8 }}>
          <Badge tone={statusTone[inv.status]}>{statusLabel[inv.status]}</Badge>
          <Badge>{paymentLabel[inv.paymentMethod]}</Badge>
          <Badge>Vendu par {inv.userName}</Badge>
          {inv.refundDue > 0 && <Badge tone="orange">À rembourser : {money(inv.refundDue)}</Badge>}
        </div>

        {inv.status === 'CANCELLED' && (
          <Alert tone="warn">
            Cette facture est annulée. La marchandise a été remise en stock et le document est conservé
            pour que la numérotation reste continue.
          </Alert>
        )}

        <DataTable
          rows={inv.lines}
          rowKey={(l) => l.productId + l.sku + l.unitPrice}
          columns={[
            {
              key: 'product', header: 'Article',
              render: (l) => (
                <>
                  <div className="cell-primary">{l.productName}</div>
                  <div className="cell-secondary mono">{l.sku}</div>
                  {l.serials && l.serials.length > 0 && (
                    <div className="cell-secondary">N° série : {l.serials.join(', ')}</div>
                  )}
                </>
              ),
            },
            { key: 'qty', header: 'Qté', align: 'right', width: 70, render: (l) => formatNumber(l.quantity) },
            { key: 'price', header: 'PU HT', align: 'right', width: 110, render: (l) => amount(l.unitPrice) },
            {
              key: 'discount', header: 'Remise', align: 'right', width: 100,
              render: (l) => l.discount > 0 ? amount(l.discount) : <span className="muted">-</span>,
            },
            { key: 'tax', header: 'Taxe', align: 'right', width: 70, render: (l) => `${l.taxRate} %` },
            { key: 'ht', header: 'Total HT', align: 'right', width: 110, render: (l) => <strong>{amount(l.lineHT)}</strong> },
          ]}
        />

        <div className="row" style={{ alignItems: 'flex-start', gap: 20 }}>
          <div style={{ flex: 1 }}>
            {inv.notes && (
              <>
                <div className="section-title">Notes</div>
                <p className="small" style={{ whiteSpace: 'pre-wrap' }} data-selectable>{inv.notes}</p>
              </>
            )}
            {(inv.customerAddress || inv.customerTaxId) && (
              <>
                <div className="section-title" style={{ marginTop: 14 }}>Client</div>
                <p className="small muted" data-selectable>
                  {inv.customerAddress}{inv.customerAddress && inv.customerTaxId && ' · '}
                  {inv.customerTaxId && `NIF ${inv.customerTaxId}`}
                </p>
              </>
            )}
            <p className="small muted" style={{ marginTop: 14 }}>
              Créée le {formatDateTime(inv.createdAt)}
            </p>
          </div>

          <div style={{ width: 300 }}>
            <SummaryRow label="Sous-total HT" value={amount(inv.subtotalHT)} />
            {inv.globalDiscount > 0 && <SummaryRow label="Remise globale" value={`− ${amount(inv.globalDiscount)}`} />}
            <SummaryRow label="Taxes" value={amount(inv.taxTotal)} />
            <div className="divider" style={{ margin: '8px 0' }} />
            <SummaryRow label="Net à payer" value={money(inv.total)} strong />
            <SummaryRow label="Réglé" value={amount(inv.amountPaid)} />
            <SummaryRow
              label="Reste dû"
              value={amount(inv.balance)}
              strong
              accent={inv.balance > 0}
            />
            {props.canSeeCost && (
              <>
                <div className="divider" style={{ margin: '8px 0' }} />
                <SummaryRow label="Coût des ventes" value={amount(inv.costTotal)} muted />
                <SummaryRow label="Marge" value={amount(inv.margin)} muted />
              </>
            )}
          </div>
        </div>
      </div>
    </Modal>
  )
}

function SummaryRow(props: { label: string; value: string; strong?: boolean; accent?: boolean; muted?: boolean }) {
  return (
    <div className="row" style={{ justifyContent: 'space-between', padding: '3px 0' }}>
      <span className={props.muted ? 'muted small' : 'small'}>{props.label}</span>
      <span
        className="tabular"
        style={{
          fontWeight: props.strong ? 650 : 400,
          fontSize: props.strong ? 14 : 13,
          color: props.accent ? 'var(--accent)' : props.muted ? 'var(--muted)' : undefined,
        }}
      >{props.value}</span>
    </div>
  )
}

const PAYMENTS: { value: PaymentMethod; label: string }[] = [
  { value: 'CASH', label: 'Espèces' },
  { value: 'MOBILE', label: 'Mobile money' },
  { value: 'TRANSFER', label: 'Virement' },
  { value: 'CHECK', label: 'Chèque' },
  { value: 'CREDIT', label: 'Crédit' },
]

function PaymentModal(props: { invoice: Invoice; onClose: () => void; onPaid: (next: Invoice) => void }) {
  const { money, decimals, symbol } = useSession()
  const [amountPaid, setAmountPaid] = useState(props.invoice.balance)
  const [method, setMethod] = useState<PaymentMethod>(props.invoice.paymentMethod)
  const [note, setNote] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function save() {
    setBusy(true)
    setError(null)
    try {
      props.onPaid(await Sales.registerPayment({
        invoiceId: props.invoice.id, amount: amountPaid, method, note,
      }))
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Enregistrer un règlement"
      subtitle={`Facture ${props.invoice.number}, reste dû ${money(props.invoice.balance)}`}
      onClose={props.onClose}
      onSubmit={save}
      footer={
        <>
          <button type="button" className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? 'Enregistrement…' : 'Enregistrer'}
          </button>
        </>
      }
    >
      <div className="stack" style={{ gap: 14 }}>
        {error && <Alert tone="danger">{error}</Alert>}
        <Field label="Montant reçu" required hint={`Maximum : ${money(props.invoice.balance)}`}>
          <MoneyInput value={amountPaid} onChange={setAmountPaid} decimals={decimals} symbol={symbol} autoFocus />
        </Field>
        <Field label="Mode de règlement">
          <Select value={method} onChange={setMethod} options={PAYMENTS} />
        </Field>
        <Field label="Note" hint="Ajoutée aux notes de la facture.">
          <TextInput value={note} onChange={setNote} placeholder="Reçu n° 42, versement partiel…" />
        </Field>
      </div>
    </Modal>
  )
}

function CancelModal(props: { invoice: Invoice; onClose: () => void; onCancelled: () => void }) {
  const { money } = useSession()
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit() {
    setBusy(true)
    setError(null)
    try {
      await Sales.cancel(props.invoice.id, reason)
      props.onCancelled()
    } catch (err) {
      setError(messageOf(err))
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Annuler la facture"
      subtitle={`${props.invoice.number}, ${money(props.invoice.total)}`}
      onClose={props.onClose}
      onSubmit={submit}
      footer={
        <>
          <button type="button" className="btn" onClick={props.onClose} disabled={busy}>Revenir</button>
          <button type="submit" className="btn btn-danger" disabled={busy}>
            {busy ? 'Annulation…' : "Confirmer l'annulation"}
          </button>
        </>
      }
    >
      <div className="stack" style={{ gap: 14 }}>
        {error && <Alert tone="danger">{error}</Alert>}
        <Alert tone="warn">
          La marchandise sera remise en stock et la facture conservée avec le statut « Annulée » :
          une numérotation ne doit jamais comporter de trou.
          {props.invoice.amountPaid > 0 && (
            <> L'acompte de {money(props.invoice.amountPaid)} déjà encaissé restera signalé comme à rembourser.</>
          )}
        </Alert>
        <Field label="Motif de l'annulation" required hint="Inscrit dans le journal d'audit et sur le document.">
          <TextArea value={reason} onChange={setReason} rows={2}
            placeholder="Erreur de saisie, retour intégral du client…" />
        </Field>
      </div>
    </Modal>
  )
}

// Réexporté pour la page Achats, qui affiche les mêmes totaux.
export { SummaryRow }
