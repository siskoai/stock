// Écrans d'entrée : premier démarrage, connexion, changement de mot de passe
// imposé. Ils partagent la même carte centrée, sans barre latérale, tant que
// la session n'est pas ouverte, il n'y a rien à naviguer.

import { useState, type FormEvent } from 'react'
import { Session, messageOf } from '../lib/api'
import { useSession } from '../lib/session'
import { Alert, Field, TextInput } from '../components/UI'
import { Attribution } from '../components/Attribution'

function Shell({ title, subtitle, children, footer }: {
  title: string; subtitle: string; children: React.ReactNode; footer?: React.ReactNode
}) {
  const { state } = useSession()
  return (
    <div className="gate">
      <div className="gate-card">
        <div className="gate-mark">C</div>
        <h1 className="gate-title">{title}</h1>
        <p className="gate-sub">{subtitle}</p>
        {children}
        <div style={{ marginTop: 20 }}><Attribution compact /></div>
        <div className="gate-foot">
          {footer ?? <>Comptoir {state.appVersion}, vos données restent sur ce poste.</>}
        </div>
      </div>
    </div>
  )
}

export function LoginPage() {
  const { state, setState } = useSession()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      setState(await Session.login(username, password))
    } catch (err) {
      setError(messageOf(err))
      setPassword('')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell title="Connexion" subtitle={`${state.companyName}. Identifiez-vous pour ouvrir la caisse.`}>
      <form onSubmit={submit} className="stack-sm" style={{ gap: 13 }}>
        {error && <Alert tone="danger">{error}</Alert>}
        <Field label="Identifiant">
          <TextInput value={username} onChange={setUsername} autoFocus />
        </Field>
        <Field label="Mot de passe">
          <TextInput type="password" value={password} onChange={setPassword} />
        </Field>
        <button className="btn btn-primary btn-block" type="submit" disabled={busy} style={{ marginTop: 6 }}>
          {busy ? 'Connexion…' : 'Se connecter'}
        </button>
      </form>
    </Shell>
  )
}

export function ChangePasswordGate() {
  const { setState } = useSession()
  const [oldPwd, setOldPwd] = useState('')
  const [newPwd, setNewPwd] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (newPwd !== confirm) {
      setError('Les deux mots de passe ne correspondent pas.')
      return
    }
    setBusy(true)
    setError(null)
    try {
      setState(await Session.changePassword(oldPwd, newPwd))
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell
      title="Nouveau mot de passe"
      subtitle="Votre mot de passe a été réinitialisé par un administrateur. Choisissez-en un nouveau : c'est la seule action possible tant qu'il n'est pas changé."
    >
      <form onSubmit={submit} className="stack-sm" style={{ gap: 13 }}>
        {error && <Alert tone="danger">{error}</Alert>}
        <Field label="Mot de passe provisoire" required>
          <TextInput type="password" value={oldPwd} onChange={setOldPwd} autoFocus />
        </Field>
        <Field label="Nouveau mot de passe" required hint="Au moins 8 caractères, dont une lettre et un chiffre.">
          <TextInput type="password" value={newPwd} onChange={setNewPwd} />
        </Field>
        <Field label="Confirmer" required>
          <TextInput type="password" value={confirm} onChange={setConfirm} />
        </Field>
        <button className="btn btn-primary btn-block" type="submit" disabled={busy} style={{ marginTop: 6 }}>
          {busy ? 'Enregistrement…' : 'Changer le mot de passe'}
        </button>
      </form>
    </Shell>
  )
}
