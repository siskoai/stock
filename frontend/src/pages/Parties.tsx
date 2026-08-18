// Clients et fournisseurs : mêmes champs, deux listes.

import { useMemo, useState } from 'react'
import { Catalog, Documents, Export, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDate } from '../lib/format'
import {
  Alert, Card, Checkbox, ConfirmDialog, DataTable, Empty, Field,
  Loading, Modal, SearchInput, SegmentedControl, TextArea, TextInput,
} from '../components/UI'
import { useDocumentPreview } from '../components/DocumentPreview'
import { IconDownload, IconPlus, IconPrint } from '../components/Icons'
import type { PartyInput, PartyType, PartyView } from '../lib/types'
import type { PageContext } from '../App'

export function PartiesPage({ navigate }: PageContext) {
  const { money, can } = useSession()
  const toast = useToast()
  const doc = useDocumentPreview()

  const [type, setType] = useState<PartyType>('CUSTOMER')
  const [search, setSearch] = useState('')
  const [editing, setEditing] = useState<PartyView | 'new' | null>(null)
  const [deleting, setDeleting] = useState<PartyView | null>(null)
  const [busy, setBusy] = useState(false)

  const debounced = useDebounced(search)
  const parties = useAsync(() => Catalog.listParties(type, debounced), [type, debounced])
  const isCustomer = type === 'CUSTOMER'

  const totals = useMemo(() => {
    const list = parties.data ?? []
    return {
      count: list.length,
      amount: list.reduce((s, p) => s + p.totalAmount, 0),
      due: list.reduce((s, p) => s + p.outstandingBalance, 0),
    }
  }, [parties.data])

  async function remove() {
    if (!deleting) return
    setBusy(true)
    try {
      await Catalog.deleteParty(deleting.id)
      toast.success('Fiche supprimée.')
      setDeleting(null)
      parties.reload()
    } catch (err) {
      toast.error(messageOf(err))
      setDeleting(null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="stack">
      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SegmentedControl
            value={type}
            onChange={setType}
            options={[
              { value: 'CUSTOMER' as PartyType, label: 'Clients' },
              { value: 'SUPPLIER' as PartyType, label: 'Fournisseurs' },
            ]}
          />
          <SearchInput value={search} onChange={setSearch} placeholder="Nom, société, téléphone, e-mail…" />
          <div className="spacer" />
          <button className="btn" disabled={doc.busy} onClick={() => doc.download(() => Export.parties(type))}>
            <IconDownload />Exporter
          </button>
          {can('catalog') && (
            <button className="btn btn-primary" onClick={() => setEditing('new')}>
              <IconPlus />{isCustomer ? 'Nouveau client' : 'Nouveau fournisseur'}
            </button>
          )}
        </div>
      </Card>

      {parties.error && <Alert tone="danger">{parties.error}</Alert>}

      <Card flush>
        {parties.loading ? <Loading /> : (
          <DataTable
            rows={parties.data ?? []}
            rowKey={(p) => p.id}
            onRowClick={(p) => setEditing(p)}
            dimmed={(p) => !p.active}
            empty={
              <Empty
                title={isCustomer ? 'Aucun client enregistré' : 'Aucun fournisseur enregistré'}
                text={isCustomer
                  ? "Une vente au comptoir n'exige pas de fiche client. Enregistrez ceux qui reviennent ou qui achètent à crédit."
                  : 'Enregistrez les fournisseurs pour rattacher les bons d\'entrée et suivre les retours.'}
                action={can('catalog') && !search
                  ? <button className="btn btn-primary" onClick={() => setEditing('new')}><IconPlus />Créer une fiche</button>
                  : undefined}
              />
            }
            columns={[
              {
                key: 'name', header: 'Nom',
                render: (p) => (
                  <>
                    <div className="cell-primary">{p.name}</div>
                    <div className="cell-secondary">{p.company || (p.active ? '' : 'Fiche désactivée')}</div>
                  </>
                ),
              },
              {
                key: 'contact', header: 'Contact',
                render: (p) => (
                  <>
                    <div>{p.phone || '—'}</div>
                    {p.email && <div className="cell-secondary">{p.email}</div>}
                  </>
                ),
              },
              { key: 'city', header: 'Ville', render: (p) => p.city || '—' },
              { key: 'docs', header: 'Documents', align: 'right', render: (p) => p.documentCount || '—' },
              { key: 'total', header: 'Total', align: 'right', render: (p) => money(p.totalAmount) },
              ...(isCustomer ? [{
                key: 'due', header: 'Impayés', align: 'right' as const,
                render: (p: PartyView) => p.outstandingBalance > 0
                  ? <strong style={{ color: 'var(--accent)' }}>{money(p.outstandingBalance)}</strong>
                  : <span className="muted">—</span>,
              }] : []),
              {
                key: 'last', header: 'Dernière activité', align: 'right',
                render: (p) => <span className="small muted">{p.lastActivity ? formatDate(p.lastActivity) : '—'}</span>,
              },
              {
                key: 'actions', header: '', align: 'right', width: 190,
                render: (p) => (
                  <div className="row" style={{ justifyContent: 'flex-end', gap: 4 }}>
                    {isCustomer && p.documentCount > 0 && (
                      <button className="btn btn-sm btn-ghost" disabled={doc.busy}
                        onClick={(e) => {
                          e.stopPropagation()
                          doc.open(`Relevé — ${p.name}`, () => Documents.partyStatement(p.id))
                        }}><IconPrint />Relevé</button>
                    )}
                    {isCustomer && p.documentCount > 0 && (
                      <button className="btn btn-sm btn-ghost"
                        onClick={(e) => { e.stopPropagation(); navigate('sales', 'client:' + p.id) }}>Ventes</button>
                    )}
                    {can('delete') && p.documentCount === 0 && (
                      <button className="btn btn-sm btn-ghost"
                        onClick={(e) => { e.stopPropagation(); setDeleting(p) }}>Supprimer</button>
                    )}
                  </div>
                ),
              },
            ]}
          />
        )}
      </Card>

      {totals.count > 0 && (
        <div className="row small muted" style={{ justifyContent: 'flex-end', gap: 22 }}>
          <span>{totals.count} fiche{totals.count > 1 ? 's' : ''}</span>
          <span>Total : <strong className="tabular">{money(totals.amount)}</strong></span>
          {isCustomer && totals.due > 0 && (
            <span>Impayés : <strong className="tabular" style={{ color: 'var(--accent)' }}>{money(totals.due)}</strong></span>
          )}
        </div>
      )}

      {editing && (
        <PartyModal
          party={editing === 'new' ? null : editing}
          type={type}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); parties.reload(); toast.success('Fiche enregistrée.') }}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title="Supprimer la fiche ?"
          danger
          busy={busy}
          confirmLabel="Supprimer"
          message={`« ${deleting.name} » sera effacé. Une fiche liée à des documents ne peut pas être supprimée : désactivez-la.`}
          onConfirm={remove}
          onCancel={() => setDeleting(null)}
        />
      )}

      {doc.element}
    </div>
  )
}

export function PartyModal(props: {
  party: PartyView | null
  type: PartyType
  onClose: () => void
  onSaved: (id: string) => void
}) {
  const [form, setForm] = useState<PartyInput>(() => props.party ?? {
    type: props.type, name: '', company: '', phone: '', email: '',
    address: '', city: '', taxId: '', notes: '', active: true,
  })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const set = <K extends keyof PartyInput>(k: K, v: PartyInput[K]) => setForm((f) => ({ ...f, [k]: v }))
  const isCustomer = form.type === 'CUSTOMER'

  async function save() {
    setBusy(true)
    setError(null)
    try {
      const saved = await Catalog.saveParty(form)
      props.onSaved(saved.id)
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={props.party ? form.name : (isCustomer ? 'Nouveau client' : 'Nouveau fournisseur')}
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
          <Field label="Nom" required>
            <TextInput value={form.name} onChange={(v) => set('name', v)} autoFocus />
          </Field>
          <Field label="Société">
            <TextInput value={form.company ?? ''} onChange={(v) => set('company', v)} />
          </Field>
        </div>
        <div className="grid grid-2">
          <Field label="Téléphone">
            <TextInput value={form.phone ?? ''} onChange={(v) => set('phone', v)} placeholder="+223 ..." />
          </Field>
          <Field label="E-mail">
            <TextInput type="email" value={form.email ?? ''} onChange={(v) => set('email', v)} />
          </Field>
        </div>
        <div className="grid grid-2">
          <Field label="Adresse">
            <TextInput value={form.address ?? ''} onChange={(v) => set('address', v)} />
          </Field>
          <Field label="Ville">
            <TextInput value={form.city ?? ''} onChange={(v) => set('city', v)} />
          </Field>
        </div>
        <Field label="NIF / N° contribuable" hint="Reporté sur les factures.">
          <TextInput value={form.taxId ?? ''} onChange={(v) => set('taxId', v)} />
        </Field>
        <Field label="Notes">
          <TextArea value={form.notes ?? ''} onChange={(v) => set('notes', v)} rows={2} />
        </Field>
        {props.party && (
          <Checkbox
            checked={form.active}
            onChange={(v) => set('active', v)}
            label="Fiche active"
            hint="Une fiche désactivée reste consultable mais n'est plus proposée."
          />
        )}
      </div>
    </Modal>
  )
}
