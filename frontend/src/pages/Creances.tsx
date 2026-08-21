// Créances : qui doit combien, et depuis quand.
//
// Le total des impayés est une information de surveillance ; l'ancienneté est
// une information d'action. Cet écran met donc le classement par retard au
// premier plan, et l'ordre de la liste est celui dans lequel on relance.

import { useMemo, useState } from 'react'
import { Creances, Documents, Sales, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDate, statusLabel, statusTone } from '../lib/format'
import {
  Alert, Badge, Card, DataTable, Empty, Field, KPI,
  Loading, Modal, MoneyInput, SearchInput, Select, TextInput,
} from '../components/UI'
import { useDocumentPreview } from '../components/DocumentPreview'
import { IconPrint } from '../components/Icons'
import type { Creance, DebiteurTotal, PaymentMethod, Tranche } from '../lib/types'
import type { PageContext } from '../App'

/** Teinte de chaque tranche : du vert rassurant au rouge inquiétant. */
const TON: Record<Tranche, string> = {
  NON_ECHUE: 'green',
  SANS_TERME: 'muted',
  J1_30: 'yellow',
  J31_60: 'orange',
  J61_90: 'magenta',
  J90_PLUS: 'red',
}

export function CreancesPage({ navigate }: PageContext) {
  const { money, amount } = useSession()
  const doc = useDocumentPreview()
  const [search, setSearch] = useState('')
  const [seulementEnRetard, setSeulementEnRetard] = useState(false)
  const [vue, setVue] = useState<'factures' | 'clients'>('factures')
  const [regler, setRegler] = useState<Creance | null>(null)
  const [reporter, setReporter] = useState<Creance | null>(null)

  const debounced = useDebounced(search)
  const query = useMemo(() => ({ search: debounced, seulementEnRetard }), [debounced, seulementEnRetard])
  const etat = useAsync(() => Creances.etat(query), [query])

  const d = etat.data
  const partRetard = d && d.total > 0 ? (d.enRetard / d.total) * 100 : 0

  return (
    <div className="stack">
      <div className="grid grid-4">
        <KPI
          label="Total dû"
          value={money(d?.total ?? 0)}
          hint={`${d?.nombre ?? 0} facture${(d?.nombre ?? 0) > 1 ? 's' : ''} non soldée${(d?.nombre ?? 0) > 1 ? 's' : ''}`}
        />
        <KPI
          label="Échéances dépassées"
          value={money(d?.enRetard ?? 0)}
          hint={`${d?.nombreRetard ?? 0} facture${(d?.nombreRetard ?? 0) > 1 ? 's' : ''} en retard`}
          accent={(d?.enRetard ?? 0) > 0}
        />
        <KPI
          label="Part en retard"
          value={`${partRetard.toFixed(0)} %`}
          hint="du total des créances"
        />
        <KPI
          label="Débiteurs"
          value={String(d?.debiteurs?.length ?? 0)}
          hint="clients avec un solde ouvert"
        />
      </div>

      {d && (d.tranches ?? []).length > 0 && (
        <Card title="Ancienneté des créances" note="Ce qui est dû, classé par retard">
          <div className="tranches">
            {(d.tranches ?? []).map((t) => (
              <div className="tranche" key={t.tranche}>
                <Badge tone={TON[t.tranche]}>{t.libelle}</Badge>
                <div className="tranche-montant">{amount(t.montant)}</div>
                <div className="tranche-detail">
                  {t.nombre} facture{t.nombre > 1 ? 's' : ''} · {t.part.toFixed(0)} %
                </div>
                <div className="tranche-barre">
                  <div style={{ width: `${Math.max(2, t.part)}%` }} />
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SearchInput value={search} onChange={setSearch} placeholder="Client, numéro de facture, téléphone…" />
          <button
            className={`btn ${seulementEnRetard ? 'btn-primary' : ''}`}
            onClick={() => setSeulementEnRetard(!seulementEnRetard)}
          >Seulement les retards</button>
          <div className="spacer" />
          <div className="btn-group">
            <button className={vue === 'factures' ? 'active' : ''} onClick={() => setVue('factures')}>Par facture</button>
            <button className={vue === 'clients' ? 'active' : ''} onClick={() => setVue('clients')}>Par client</button>
          </div>
        </div>
      </Card>

      {etat.error && <Alert tone="danger">{etat.error}</Alert>}

      <Card flush>
        {etat.loading ? <Loading /> : vue === 'factures' ? (
          <DataTable
            rows={d?.lignes ?? []}
            rowKey={(c) => c.invoiceId}
            onRowClick={(c) => navigate('sales', c.invoiceId)}
            empty={
              <Empty
                title={seulementEnRetard ? 'Aucune échéance dépassée' : 'Aucune créance'}
                text={seulementEnRetard
                  ? 'Tous vos clients à crédit sont dans les temps.'
                  : 'Tout ce que vous avez vendu a été encaissé.'}
              />
            }
            columns={[
              { key: 'number', header: 'Facture', width: 130, render: (c) => <span className="mono">{c.number}</span> },
              {
                key: 'client', header: 'Client',
                render: (c) => (
                  <>
                    <div className="cell-primary">{c.customerName}</div>
                    {c.customerPhone && <div className="cell-secondary">{c.customerPhone}</div>}
                  </>
                ),
              },
              { key: 'date', header: 'Vendue le', width: 100, render: (c) => formatDate(c.date) },
              {
                key: 'echeance', header: 'Échéance', width: 160,
                render: (c) => (
                  <>
                    <div>{c.dueDate ? formatDate(c.dueDate) : <span className="muted">non convenue</span>}</div>
                    <Badge tone={TON[c.tranche]}>
                      {c.joursDeRetard > 0
                        ? `${c.joursDeRetard} jour${c.joursDeRetard > 1 ? 's' : ''} de retard`
                        : c.tranche === 'SANS_TERME' ? 'sans échéance' : 'dans les temps'}
                    </Badge>
                  </>
                ),
              },
              { key: 'total', header: 'Total', align: 'right', width: 110, render: (c) => amount(c.total) },
              { key: 'paid', header: 'Réglé', align: 'right', width: 110, render: (c) => amount(c.paid) },
              {
                key: 'balance', header: 'Reste dû', align: 'right', width: 120,
                render: (c) => <strong style={{ color: 'var(--accent)' }}>{amount(c.balance)}</strong>,
              },
              {
                key: 'actions', header: '', align: 'right', width: 190,
                render: (c) => (
                  <div className="row" style={{ justifyContent: 'flex-end', gap: 4 }}>
                    <button className="btn btn-sm btn-ghost"
                      onClick={(e) => { e.stopPropagation(); setRegler(c) }}>Encaisser</button>
                    <button className="btn btn-sm btn-ghost"
                      onClick={(e) => { e.stopPropagation(); setReporter(c) }}>Échéance</button>
                  </div>
                ),
              },
            ]}
          />
        ) : (
          <DataTable
            rows={d?.debiteurs ?? []}
            rowKey={(x: DebiteurTotal) => x.partyId || x.nom}
            empty={<Empty title="Aucun débiteur" />}
            columns={[
              {
                key: 'nom', header: 'Client',
                render: (x) => (
                  <>
                    <div className="cell-primary">{x.nom}</div>
                    {x.telephone && <div className="cell-secondary">{x.telephone}</div>}
                  </>
                ),
              },
              { key: 'nombre', header: 'Factures', align: 'right', width: 90, render: (x) => x.nombre },
              { key: 'solde', header: 'Doit', align: 'right', width: 130, render: (x) => <strong>{amount(x.solde)}</strong> },
              {
                key: 'retard', header: 'Dont en retard', align: 'right', width: 140,
                render: (x) => x.enRetard > 0
                  ? <span style={{ color: 'var(--accent)', fontWeight: 600 }}>{amount(x.enRetard)}</span>
                  : <span className="muted">—</span>,
              },
              {
                key: 'anciennete', header: 'Retard le plus ancien', align: 'right', width: 160,
                render: (x) => x.plusAncien > 0
                  ? `${x.plusAncien} jour${x.plusAncien > 1 ? 's' : ''}`
                  : <span className="muted">aucun</span>,
              },
              {
                key: 'actions', header: '', align: 'right', width: 110,
                render: (x) => x.partyId ? (
                  <button className="btn btn-sm btn-ghost" disabled={doc.busy}
                    onClick={() => doc.open(`Relance, ${x.nom}`, () => Documents.reminder(x.partyId))}>
                    <IconPrint />Relance
                  </button>
                ) : <span className="small muted" title="Vente au comptoir sans fiche client">sans fiche</span>,
              },
            ]}
          />
        )}
      </Card>

      {regler && (
        <ReglementModal creance={regler} onClose={() => setRegler(null)}
          onPaid={() => { setRegler(null); etat.reload() }} />
      )}
      {reporter && (
        <EcheanceModal creance={reporter} onClose={() => setReporter(null)}
          onDone={() => { setReporter(null); etat.reload() }} />
      )}
      {doc.element}
    </div>
  )
}

const PAYMENTS: { value: PaymentMethod; label: string }[] = [
  { value: 'CASH', label: 'Espèces' },
  { value: 'MOBILE', label: 'Mobile money' },
  { value: 'TRANSFER', label: 'Virement' },
  { value: 'CHECK', label: 'Chèque' },
]

function ReglementModal(props: { creance: Creance; onClose: () => void; onPaid: () => void }) {
  const { money, decimals, symbol } = useSession()
  const toast = useToast()
  const [montant, setMontant] = useState(props.creance.balance)
  const [methode, setMethode] = useState<PaymentMethod>('CASH')
  const [note, setNote] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function enregistrer() {
    setBusy(true)
    setError(null)
    try {
      await Sales.registerPayment({
        invoiceId: props.creance.invoiceId, amount: montant, method: methode, note,
      })
      toast.success('Règlement enregistré.')
      props.onPaid()
    } catch (err) {
      setError(messageOf(err))
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Encaisser une créance"
      subtitle={`${props.creance.customerName} · facture ${props.creance.number}`}
      onClose={props.onClose}
      onSubmit={enregistrer}
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
        {props.creance.joursDeRetard > 0 && (
          <Alert tone="warn">
            Cette facture est en retard de {props.creance.joursDeRetard} jour
            {props.creance.joursDeRetard > 1 ? 's' : ''}.
          </Alert>
        )}
        <Field label="Montant reçu" required hint={`Reste dû : ${money(props.creance.balance)}`}>
          <MoneyInput value={montant} onChange={setMontant} decimals={decimals} symbol={symbol} autoFocus />
        </Field>
        <Field label="Mode de règlement">
          <Select value={methode} onChange={setMethode} options={PAYMENTS} />
        </Field>
        <Field label="Note" hint="Ajoutée aux notes de la facture.">
          <TextInput value={note} onChange={setNote} placeholder="Reçu n° 42, versement partiel…" />
        </Field>
      </div>
    </Modal>
  )
}

function EcheanceModal(props: { creance: Creance; onClose: () => void; onDone: () => void }) {
  const toast = useToast()
  const [date, setDate] = useState(props.creance.dueDate?.slice(0, 10) ?? '')
  const [note, setNote] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function enregistrer() {
    setBusy(true)
    setError(null)
    try {
      await Creances.fixerEcheance({ invoiceId: props.creance.invoiceId, dueDate: date, note })
      toast.success('Échéance enregistrée.')
      props.onDone()
    } catch (err) {
      setError(messageOf(err))
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Convenir d'une échéance"
      subtitle={`${props.creance.customerName} · facture ${props.creance.number}`}
      onClose={props.onClose}
      onSubmit={enregistrer}
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
        <div className="row" style={{ gap: 8 }}>
          <Badge tone={statusTone.PARTIAL}>{statusLabel.PARTIAL}</Badge>
          <span className="small muted">
            Échéance actuelle : {props.creance.dueDate ? formatDate(props.creance.dueDate) : 'aucune'}
          </span>
        </div>
        <Field label="Nouvelle échéance" hint="Laissez vide pour retirer toute date convenue.">
          <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
        <Field label="Motif du report" hint="Inscrit sur la facture et dans le journal d'audit.">
          <TextInput value={note} onChange={setNote} placeholder="Accord verbal, difficulté passagère…" />
        </Field>
        <Alert tone="info">
          Reporter une échéance est une décision commerciale : elle reste
          inscrite dans les notes de la facture, quel que soit le vendeur qui la
          consultera plus tard.
        </Alert>
      </div>
    </Modal>
  )
}
