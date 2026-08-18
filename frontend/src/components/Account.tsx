// Fiche du compte connecté : ce qu'il voit, et le changement de son mot de passe.
//
// Cet écran existe pour tous les rôles. Sans lui, un vendeur ne pourrait jamais
// changer son propre mot de passe — il devrait le demander à un administrateur,
// qui le lui fixerait, ce qui revient à ce que deux personnes le connaissent.

import { useState } from 'react'
import { Session, messageOf } from '../lib/api'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDateTime, roleLabel } from '../lib/format'
import { Alert, Badge, Field, Modal, TextInput } from './UI'
import { Attribution } from './Attribution'

const SCOPE_LABELS: Record<string, string> = {
  catalog: 'Catalogue et fiches articles',
  stock: 'Mouvements de stock et inventaire',
  sales: 'Ventes et facturation',
  purchases: 'Achats et réceptions',
  expenses: "Charges d'exploitation",
  finance: 'Rapports financiers, coûts et marges',
  users: 'Comptes et journal d’audit',
  settings: "Paramètres de l'application",
  backup: 'Sauvegardes et restauration',
  delete: 'Suppressions définitives',
}

export function Account({ onClose }: { onClose: () => void }) {
  const { state, setState } = useSession()
  const toast = useToast()
  const [changing, setChanging] = useState(false)
  const [oldPwd, setOldPwd] = useState('')
  const [newPwd, setNewPwd] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const user = state.user
  if (!user) return null

  async function submit() {
    if (newPwd !== confirm) {
      setError('Les deux mots de passe ne correspondent pas.')
      return
    }
    setBusy(true)
    setError(null)
    try {
      setState(await Session.changePassword(oldPwd, newPwd))
      toast.success('Mot de passe modifié.')
      onClose()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  if (changing) {
    return (
      <Modal
        title="Changer mon mot de passe"
        subtitle="Il ne transite nulle part : seul son empreinte est conservée sur ce poste."
        onClose={() => setChanging(false)}
        onSubmit={submit}
        footer={
          <>
            <button type="button" className="btn" onClick={() => setChanging(false)} disabled={busy}>
              Revenir
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? 'Enregistrement…' : 'Changer le mot de passe'}
            </button>
          </>
        }
      >
        <div className="stack" style={{ gap: 14 }}>
          {error && <Alert tone="danger">{error}</Alert>}
          <Field label="Mot de passe actuel" required>
            <TextInput type="password" value={oldPwd} onChange={setOldPwd} autoFocus />
          </Field>
          <Field label="Nouveau mot de passe" required hint="Au moins 8 caractères, dont une lettre et un chiffre.">
            <TextInput type="password" value={newPwd} onChange={setNewPwd} />
          </Field>
          <Field label="Confirmer" required>
            <TextInput type="password" value={confirm} onChange={setConfirm} />
          </Field>
        </div>
      </Modal>
    )
  }

  return (
    <Modal
      title="Mon compte"
      subtitle={user.fullName}
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={() => setChanging(true)}>Changer mon mot de passe</button>
          <div className="spacer" />
          <button className="btn btn-primary" onClick={onClose}>Fermer</button>
        </>
      }
    >
      <div className="stack" style={{ gap: 16 }}>
        <div className="grid grid-2">
          <Row label="Identifiant" value={<span className="mono">{user.username}</span>} />
          <Row label="Rôle" value={<Badge tone={user.role === 'ADMIN' ? 'violet' : user.role === 'MANAGER' ? 'blue' : 'muted'}>{roleLabel[user.role]}</Badge>} />
          <Row label="Compte créé le" value={formatDateTime(user.createdAt)} />
          <Row label="Connexion précédente" value={user.lastLogin ? formatDateTime(user.lastLogin) : 'première connexion'} />
        </div>

        <div>
          <div className="section-title">Ce que votre rôle autorise</div>
          {state.scopes.length === 0 ? (
            <p className="small muted">Consultation du catalogue et du stock uniquement.</p>
          ) : (
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {state.scopes.map((scope) => (
                <li key={scope} className="small" style={{ lineHeight: 1.7 }}>
                  {SCOPE_LABELS[scope] ?? scope}
                </li>
              ))}
            </ul>
          )}
          <p className="small muted" style={{ marginTop: 10, lineHeight: 1.5 }}>
            Les domaines absents de cette liste sont refusés par le moteur, pas seulement
            masqués à l'écran.
          </p>
        </div>

        <Attribution />
      </div>
    </Modal>
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <div className="field-label">{label}</div>
      <div style={{ marginTop: 3 }}>{value}</div>
    </div>
  )
}
