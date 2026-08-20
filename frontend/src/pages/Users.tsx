// Comptes du poste et journal d'audit. Réservé au rôle Administrateur.

import { useMemo, useState } from 'react'
import { Users, messageOf } from '../lib/api'
import { useAsync, useDebounced } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDateTime, roleLabel } from '../lib/format'
import {
  Alert, Badge, Card, ConfirmDialog, DataTable, Empty, Field,
  Loading, Modal, SearchInput, Select, TextInput,
} from '../components/UI'
import { IconPlus } from '../components/Icons'
import type { Role, UserInput, UserView } from '../lib/types'
import type { PageContext } from '../App'

type Tab = 'accounts' | 'audit'

export function UsersPage(_: PageContext) {
  const [tab, setTab] = useState<Tab>('accounts')
  return (
    <div className="stack">
      <div className="tabs" style={{ flexWrap: 'wrap' }}>
        <button className={`tab ${tab === 'accounts' ? 'active' : ''}`} onClick={() => setTab('accounts')}>Comptes</button>
        <button className={`tab ${tab === 'audit' ? 'active' : ''}`} onClick={() => setTab('audit')}>Journal d'audit</button>
      </div>
      {tab === 'accounts' ? <Accounts /> : <Audit />}
    </div>
  )
}

// --- Comptes ---------------------------------------------------------------

function Accounts() {
  const { money } = useSession()
  const toast = useToast()
  const users = useAsync(() => Users.list(), [])
  const roles = useAsync(() => Users.roles(), [])
  const [editing, setEditing] = useState<UserView | 'new' | null>(null)
  const [resetting, setResetting] = useState<UserView | null>(null)
  const [deleting, setDeleting] = useState<UserView | null>(null)
  const [busy, setBusy] = useState(false)

  async function toggleActive(u: UserView) {
    try {
      await Users.setActive(u.id, !u.active)
      toast.success(u.active ? 'Compte désactivé.' : 'Compte réactivé.')
      users.reload()
    } catch (err) {
      toast.error(messageOf(err))
    }
  }

  async function remove() {
    if (!deleting) return
    setBusy(true)
    try {
      await Users.remove(deleting.id)
      toast.success('Compte supprimé.')
      setDeleting(null)
      users.reload()
    } catch (err) {
      toast.error(messageOf(err))
      setDeleting(null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="stack">
      <div className="row">
        <div className="spacer" />
        <button className="btn btn-primary" onClick={() => setEditing('new')}>
          <IconPlus />Nouveau compte
        </button>
      </div>

      {users.error && <Alert tone="danger">{users.error}</Alert>}

      <Card flush>
        {users.loading ? <Loading /> : (
          <DataTable
            rows={users.data ?? []}
            rowKey={(u) => u.id}
            dimmed={(u) => !u.active}
            empty={<Empty title="Aucun compte" />}
            columns={[
              {
                key: 'name', header: 'Compte',
                render: (u) => (
                  <>
                    <div className="cell-primary">
                      {u.fullName}
                      {u.isCurrent && <span className="muted small"> (vous)</span>}
                    </div>
                    <div className="cell-secondary mono">{u.username}</div>
                  </>
                ),
              },
              { key: 'role', header: 'Rôle', width: 150, render: (u) => <Badge tone={u.role === 'ADMIN' ? 'violet' : u.role === 'MANAGER' ? 'blue' : 'muted'}>{roleLabel[u.role]}</Badge> },
              {
                key: 'status', header: 'État', width: 150,
                render: (u) => u.mustChangePwd
                  ? <Badge tone="orange">Mot de passe à changer</Badge>
                  : u.active ? <Badge tone="green">Actif</Badge> : <Badge tone="red">Désactivé</Badge>,
              },
              { key: 'activity', header: 'Ventes', align: 'right', width: 130,
                render: (u) => u.invoiceCount > 0
                  ? <>
                      <div>{money(u.revenue)}</div>
                      <div className="cell-secondary">{u.invoiceCount} facture(s)</div>
                    </>
                  : <span className="muted">-</span> },
              { key: 'last', header: 'Dernière connexion', align: 'right', width: 170,
                render: (u) => <span className="small muted">{u.lastLogin ? formatDateTime(u.lastLogin) : 'jamais'}</span> },
              {
                key: 'actions', header: '', align: 'right', width: 290,
                render: (u) => (
                  <div className="row" style={{ justifyContent: 'flex-end', gap: 4 }}>
                    <button className="btn btn-sm btn-ghost" onClick={() => setEditing(u)}>Modifier</button>
                    <button className="btn btn-sm btn-ghost" onClick={() => setResetting(u)}>Mot de passe</button>
                    {!u.isCurrent && (
                      <button className="btn btn-sm btn-ghost" onClick={() => toggleActive(u)}>
                        {u.active ? 'Désactiver' : 'Réactiver'}
                      </button>
                    )}
                    {!u.isCurrent && u.invoiceCount === 0 && (
                      <button className="btn btn-sm btn-ghost" onClick={() => setDeleting(u)}>Supprimer</button>
                    )}
                  </div>
                ),
              },
            ]}
          />
        )}
      </Card>

      <Card title="Ce que chaque rôle autorise">
        {roles.loading ? <Loading /> : (
          <div className="stack" style={{ gap: 12 }}>
            {(roles.data ?? []).map((r) => (
              <div key={r.role} className="row" style={{ alignItems: 'flex-start', gap: 14 }}>
                <div style={{ width: 130, flex: '0 0 130px' }}>
                  <Badge tone={r.role === 'ADMIN' ? 'violet' : r.role === 'MANAGER' ? 'blue' : 'muted'}>{r.label}</Badge>
                </div>
                <p className="small" style={{ flex: 1, lineHeight: 1.5 }}>{r.description}</p>
              </div>
            ))}
          </div>
        )}
      </Card>

      {editing && (
        <UserModal
          user={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); users.reload(); toast.success('Compte enregistré.') }}
        />
      )}

      {resetting && (
        <ResetPasswordModal
          user={resetting}
          onClose={() => setResetting(null)}
          onDone={() => { setResetting(null); users.reload() }}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title="Supprimer le compte ?"
          danger busy={busy} confirmLabel="Supprimer"
          message={`Le compte « ${deleting.username} » sera effacé. Un compte figurant sur des documents doit être désactivé plutôt que supprimé.`}
          onConfirm={remove}
          onCancel={() => setDeleting(null)}
        />
      )}
    </div>
  )
}

const ROLES: { value: Role; label: string }[] = [
  { value: 'SELLER', label: 'Vendeur' },
  { value: 'MANAGER', label: 'Gérant' },
  { value: 'ADMIN', label: 'Administrateur' },
]

function UserModal(props: { user: UserView | null; onClose: () => void; onSaved: () => void }) {
  const isNew = props.user === null
  const [form, setForm] = useState<UserInput>(() => props.user
    ? { id: props.user.id, username: props.user.username, fullName: props.user.fullName, role: props.user.role }
    : { username: '', fullName: '', password: '', role: 'SELLER' })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const set = <K extends keyof UserInput>(k: K, v: UserInput[K]) => setForm((f) => ({ ...f, [k]: v }))

  async function save() {
    setBusy(true)
    setError(null)
    try {
      if (isNew) await Users.create(form)
      else await Users.update(form)
      props.onSaved()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={isNew ? 'Nouveau compte' : `Modifier ${props.user?.username}`}
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
        <Field label="Nom complet" required>
          <TextInput value={form.fullName} onChange={(v) => set('fullName', v)} autoFocus />
        </Field>
        {isNew && (
          <>
            <Field label="Identifiant de connexion" required hint="Au moins 3 caractères, sans espace.">
              <TextInput value={form.username ?? ''} onChange={(v) => set('username', v)} />
            </Field>
            <Field label="Mot de passe" required hint="Au moins 8 caractères, dont une lettre et un chiffre.">
              <TextInput type="password" value={form.password ?? ''} onChange={(v) => set('password', v)} />
            </Field>
          </>
        )}
        <Field label="Rôle" required>
          <Select value={form.role} onChange={(v) => set('role', v)} options={ROLES} />
        </Field>
        <Alert tone="info">
          {form.role === 'SELLER' && "Un vendeur enregistre des ventes et consulte le stock. Il ne voit ni les prix d'achat, ni les marges, ni les charges."}
          {form.role === 'MANAGER' && "Un gérant gère le stock, les achats, les charges et lit tous les rapports financiers. Il ne touche pas aux comptes ni aux paramètres."}
          {form.role === 'ADMIN' && "Un administrateur a accès à tout, y compris la gestion des comptes, les paramètres et les sauvegardes."}
        </Alert>
      </div>
    </Modal>
  )
}

function ResetPasswordModal(props: { user: UserView; onClose: () => void; onDone: () => void }) {
  const toast = useToast()
  const [mode, setMode] = useState<'generate' | 'manual'>('generate')
  const [password, setPassword] = useState('')
  const [generated, setGenerated] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit() {
    setBusy(true)
    setError(null)
    try {
      const result = await Users.resetPassword(props.user.id, mode === 'generate' ? '' : password)
      if (mode === 'generate') setGenerated(result)
      else {
        toast.success('Mot de passe réinitialisé.')
        props.onDone()
      }
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  if (generated) {
    return (
      <Modal
        title="Mot de passe provisoire"
        subtitle={`Compte ${props.user.username}`}
        onClose={props.onDone}
        footer={<button className="btn btn-primary" onClick={props.onDone}>J'ai noté</button>}
      >
        <div className="stack" style={{ gap: 14 }}>
          <Alert tone="warn">
            Ce mot de passe ne sera plus affiché. Transmettez-le de vive voix :
            il devra être changé à la première connexion.
          </Alert>
          <div
            data-selectable
            style={{
              padding: 18, textAlign: 'center', background: 'var(--wash)',
              borderRadius: 'var(--radius)', fontFamily: 'var(--mono)',
              fontSize: 24, letterSpacing: '.14em', fontWeight: 600,
            }}
          >{generated}</div>
        </div>
      </Modal>
    )
  }

  return (
    <Modal
      title="Réinitialiser le mot de passe"
      subtitle={`Compte ${props.user.username}, ${props.user.fullName}`}
      onClose={props.onClose}
      onSubmit={submit}
      footer={
        <>
          <button type="button" className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? 'Réinitialisation…' : 'Réinitialiser'}
          </button>
        </>
      }
    >
      <div className="stack" style={{ gap: 14 }}>
        {error && <Alert tone="danger">{error}</Alert>}
        <Field label="Méthode">
          <Select
            value={mode}
            onChange={setMode}
            options={[
              { value: 'generate' as const, label: 'Générer un mot de passe provisoire' },
              { value: 'manual' as const, label: 'Saisir un mot de passe' },
            ]}
          />
        </Field>
        {mode === 'manual' && (
          <Field label="Nouveau mot de passe" required hint="Au moins 8 caractères, dont une lettre et un chiffre.">
            <TextInput type="password" value={password} onChange={setPassword} autoFocus />
          </Field>
        )}
        <Alert tone="info">
          Dans les deux cas, le titulaire devra choisir un nouveau mot de passe à sa prochaine
          connexion : tant qu'il ne l'a pas fait, son compte n'ouvre rien.
        </Alert>
      </div>
    </Modal>
  )
}

// --- Journal d'audit -------------------------------------------------------

function Audit() {
  const [search, setSearch] = useState('')
  const [action, setAction] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')

  const debounced = useDebounced(search)
  const query = useMemo(() => ({ search: debounced, action, from, to, limit: 500 }), [debounced, action, from, to])
  const entries = useAsync(() => Users.audit(query), [query])
  const actions = useAsync(() => Users.auditActions(), [])

  return (
    <div className="stack">
      <Alert tone="info">
        Le journal est en ajout seul : aucune action ne peut l'effacer ni le modifier.
        Il enregistre qui a fait quoi, et quand.
      </Alert>

      <Card flush>
        <div className="row row-wrap" style={{ padding: 12, gap: 10 }}>
          <SearchInput value={search} onChange={setSearch} placeholder="Détail, utilisateur, entité…" />
          <div style={{ width: 160 }}>
            <Select
              value={action} onChange={setAction} placeholder="Toutes les actions"
              options={(actions.data ?? []).map((a) => ({ value: a, label: a }))}
            />
          </div>
          <div className="row" style={{ gap: 6 }}>
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ width: 148 }} />
            <span className="muted small">au</span>
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ width: 148 }} />
          </div>
        </div>
      </Card>

      {entries.error && <Alert tone="danger">{entries.error}</Alert>}

      <Card flush>
        {entries.loading ? <Loading /> : (
          <DataTable
            rows={entries.data ?? []}
            rowKey={(e) => e.id}
            empty={<Empty title="Aucune entrée" text="Le journal se remplit au fil des actions." />}
            columns={[
              { key: 'at', header: 'Date', width: 170, render: (e) => <span className="small">{formatDateTime(e.at)}</span> },
              { key: 'user', header: 'Utilisateur', width: 160, render: (e) => e.userName },
              { key: 'action', header: 'Action', width: 110, render: (e) => <Badge>{e.action}</Badge> },
              { key: 'entity', header: 'Objet', width: 100, render: (e) => <span className="small muted">{e.entity}</span> },
              { key: 'details', header: 'Détail', render: (e) => <span data-selectable>{e.details}</span> },
            ]}
          />
        )}
      </Card>
    </div>
  )
}
