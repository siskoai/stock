// Charges d'exploitation : loyer, salaires, électricité — tout sauf l'achat de
// marchandise, qui passe par les bons d'entrée.

import { useMemo, useState } from 'react'
import { Expenses, Export, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDate, isoDate, paymentLabel, startOfMonth } from '../lib/format'
import { formatPercent } from '../lib/money'
import {
  Alert, BarList, Card, ConfirmDialog, DataTable, Empty, Field,
  Loading, Modal, MoneyInput, SearchInput, Select, TextArea, TextInput,
} from '../components/UI'
import { useDocumentPreview } from '../components/DocumentPreview'
import { IconDownload, IconPlus } from '../components/Icons'
import type { Expense, ExpenseInput, PaymentMethod } from '../lib/types'
import type { PageContext } from '../App'

const PAYMENTS: { value: PaymentMethod; label: string }[] = [
  { value: 'CASH', label: 'Espèces' },
  { value: 'MOBILE', label: 'Mobile money' },
  { value: 'TRANSFER', label: 'Virement' },
  { value: 'CHECK', label: 'Chèque' },
  { value: 'CREDIT', label: 'Crédit' },
]

export function ExpensesPage(_: PageContext) {
  const { money, amount, decimals, symbol } = useSession()
  const toast = useToast()
  const doc = useDocumentPreview()

  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('')
  const [from, setFrom] = useState(startOfMonth())
  const [to, setTo] = useState(isoDate())
  const [editing, setEditing] = useState<Expense | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Expense | null>(null)
  const [busy, setBusy] = useState(false)

  const debounced = useDebounced(search)
  const query = useMemo(() => ({ search: debounced, category, from, to }), [debounced, category, from, to])

  const list = useAsync(() => Expenses.list(query), [query])
  const breakdown = useAsync(() => Expenses.breakdown(from, to), [from, to])
  const categories = useAsync(() => Expenses.categories(), [])

  const total = useMemo(() => (list.data ?? []).reduce((s, e) => s + e.amount, 0), [list.data])

  async function remove() {
    if (!deleting) return
    setBusy(true)
    try {
      await Expenses.remove(deleting.id)
      toast.success('Charge supprimée.')
      setDeleting(null)
      list.reload(); breakdown.reload()
    } catch (err) {
      toast.error(messageOf(err))
      setDeleting(null)
    } finally {
      setBusy(false)
    }
  }

  function afterSave() {
    setEditing(null)
    list.reload(); breakdown.reload(); categories.reload()
    toast.success('Charge enregistrée.')
  }

  return (
    <div className="stack">
      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SearchInput value={search} onChange={setSearch} placeholder="Libellé, bénéficiaire…" />
          <div style={{ width: 170 }}>
            <Select
              value={category} onChange={setCategory} placeholder="Toutes les rubriques"
              options={(categories.data ?? []).map((c) => ({ value: c, label: c }))}
            />
          </div>
          <div className="row" style={{ gap: 6 }}>
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ width: 148 }} />
            <span className="muted small">au</span>
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ width: 148 }} />
          </div>
          <div className="spacer" />
          <button className="btn" disabled={doc.busy} onClick={() => doc.download(() => Export.expenses(query))}>
            <IconDownload />Exporter
          </button>
          <button className="btn btn-primary" onClick={() => setEditing('new')}>
            <IconPlus />Nouvelle charge
          </button>
        </div>
      </Card>

      {list.error && <Alert tone="danger">{list.error}</Alert>}

      <div className="grid" style={{ gridTemplateColumns: 'minmax(0, 2fr) minmax(0, 1fr)' }}>
        <Card title="Écritures" note={`${(list.data ?? []).length} charge(s) sur la période`} flush>
          {list.loading ? <Loading /> : (
            <DataTable
              rows={list.data ?? []}
              rowKey={(e) => e.id}
              onRowClick={(e) => setEditing(e)}
              empty={
                <Empty
                  title="Aucune charge sur la période"
                  text="Loyer, salaires, électricité, transport : ce sont ces montants qui font la différence entre la marge et le résultat."
                  action={<button className="btn btn-primary" onClick={() => setEditing('new')}><IconPlus />Nouvelle charge</button>}
                />
              }
              footer={
                <tr>
                  <td colSpan={3}>Total</td>
                  <td className="num">{amount(total)}</td>
                  <td colSpan={2} />
                </tr>
              }
              columns={[
                { key: 'date', header: 'Date', width: 100, render: (e) => formatDate(e.date) },
                { key: 'category', header: 'Rubrique', width: 140, render: (e) => e.category },
                {
                  key: 'label', header: 'Libellé',
                  render: (e) => (
                    <>
                      <div className="cell-primary">{e.label}</div>
                      {e.beneficiary && <div className="cell-secondary">{e.beneficiary}</div>}
                    </>
                  ),
                },
                { key: 'amount', header: 'Montant', align: 'right', width: 120, render: (e) => <strong>{amount(e.amount)}</strong> },
                { key: 'method', header: 'Règlement', width: 120, render: (e) => <span className="small">{paymentLabel[e.paymentMethod]}</span> },
                {
                  key: 'actions', header: '', align: 'right', width: 90,
                  render: (e) => (
                    <button className="btn btn-sm btn-ghost"
                      onClick={(ev) => { ev.stopPropagation(); setDeleting(e) }}>Supprimer</button>
                  ),
                },
              ]}
            />
          )}
        </Card>

        <Card title="Répartition" note="Part de chaque rubrique sur la période">
          {breakdown.loading ? <Loading /> : (breakdown.data ?? []).length === 0 ? (
            <Empty title="Rien à répartir" />
          ) : (
            <>
              <BarList
                items={(breakdown.data ?? []).map((b) => ({
                  label: b.category,
                  value: b.amount,
                  display: `${amount(b.amount)} · ${formatPercent(b.share, 0)}`,
                }))}
              />
              <div className="divider" />
              <div className="row" style={{ justifyContent: 'space-between' }}>
                <span className="muted small">Total des charges</span>
                <strong className="tabular">{money(total)}</strong>
              </div>
            </>
          )}
        </Card>
      </div>

      {editing && (
        <ExpenseModal
          expense={editing === 'new' ? null : editing}
          categories={categories.data ?? []}
          decimals={decimals}
          symbol={symbol}
          onClose={() => setEditing(null)}
          onSaved={afterSave}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title="Supprimer cette charge ?"
          danger busy={busy} confirmLabel="Supprimer"
          message={`« ${deleting.label} » de ${money(deleting.amount)} sera effacée du journal des charges.`}
          onConfirm={remove}
          onCancel={() => setDeleting(null)}
        />
      )}
    </div>
  )
}

function ExpenseModal(props: {
  expense: Expense | null
  categories: string[]
  decimals: number
  symbol: string
  onClose: () => void
  onSaved: () => void
}) {
  const [form, setForm] = useState<ExpenseInput>(() => props.expense
    ? {
        id: props.expense.id, date: props.expense.date.slice(0, 10),
        category: props.expense.category, label: props.expense.label,
        amount: props.expense.amount, paymentMethod: props.expense.paymentMethod,
        beneficiary: props.expense.beneficiary, notes: props.expense.notes,
      }
    : {
        date: isoDate(), category: props.categories[0] ?? 'Divers', label: '',
        amount: 0, paymentMethod: 'CASH', beneficiary: '', notes: '',
      })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const set = <K extends keyof ExpenseInput>(k: K, v: ExpenseInput[K]) => setForm((f) => ({ ...f, [k]: v }))

  async function save() {
    setBusy(true)
    setError(null)
    try {
      await Expenses.save(form)
      props.onSaved()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={props.expense ? 'Modifier la charge' : 'Nouvelle charge'}
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
        <div className="grid grid-2">
          <Field label="Date" required>
            <input type="date" value={form.date ?? ''} onChange={(e) => set('date', e.target.value)} />
          </Field>
          <Field label="Rubrique" required>
            <Select
              value={form.category}
              onChange={(v) => set('category', v)}
              options={props.categories.map((c) => ({ value: c, label: c }))}
            />
          </Field>
        </div>
        <Field label="Libellé" required>
          <TextInput value={form.label} onChange={(v) => set('label', v)} autoFocus placeholder="Loyer du mois de mars" />
        </Field>
        <div className="grid grid-2">
          <Field label="Montant" required>
            <MoneyInput value={form.amount} onChange={(v) => set('amount', v)}
              decimals={props.decimals} symbol={props.symbol} />
          </Field>
          <Field label="Mode de règlement">
            <Select value={form.paymentMethod ?? 'CASH'} onChange={(v) => set('paymentMethod', v)} options={PAYMENTS} />
          </Field>
        </div>
        <Field label="Bénéficiaire">
          <TextInput value={form.beneficiary ?? ''} onChange={(v) => set('beneficiary', v)} placeholder="Nom du propriétaire, du fournisseur…" />
        </Field>
        <Field label="Notes">
          <TextArea value={form.notes ?? ''} onChange={(v) => set('notes', v)} rows={2} />
        </Field>
      </div>
    </Modal>
  )
}
