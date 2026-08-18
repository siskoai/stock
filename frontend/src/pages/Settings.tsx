// Paramètres : identité de la boutique, monnaie et taxes, documents,
// sauvegardes. Réservé au rôle Administrateur.

import { useEffect, useState } from 'react'
import { Backups, Config, Host, messageOf } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatDateTime } from '../lib/format'
import { formatBytes } from '../lib/money'
import {
  Alert, Card, Checkbox, ConfirmDialog, DataTable, Empty, Field,
  Loading, NumberInput, Select, TextArea, TextInput,
} from '../components/UI'
import { IconFolder, IconRefresh, IconTrash } from '../components/Icons'
import type { Settings } from '../lib/types'
import type { PageContext } from '../App'

type Tab = 'company' | 'money' | 'documents' | 'backups'

export function SettingsPage(_: PageContext) {
  const [tab, setTab] = useState<Tab>('company')
  const settings = useAsync(() => Config.get(), [])
  const presets = useAsync(() => Config.presets(), [])

  if (settings.loading) return <Loading />
  if (settings.error) return <Alert tone="danger">{settings.error}</Alert>
  if (!settings.data) return null

  return (
    <div className="content-narrow stack">
      <div className="tabs">
        <button className={`tab ${tab === 'company' ? 'active' : ''}`} onClick={() => setTab('company')}>Entreprise</button>
        <button className={`tab ${tab === 'money' ? 'active' : ''}`} onClick={() => setTab('money')}>Monnaie et taxes</button>
        <button className={`tab ${tab === 'documents' ? 'active' : ''}`} onClick={() => setTab('documents')}>Documents</button>
        <button className={`tab ${tab === 'backups' ? 'active' : ''}`} onClick={() => setTab('backups')}>Sauvegardes</button>
      </div>

      {tab === 'backups'
        ? <BackupsTab />
        : <SettingsForm
            key={settings.data.updatedAt}
            initial={settings.data}
            currencies={presets.data?.currencies ?? []}
            tab={tab}
            onSaved={settings.reload}
          />}
    </div>
  )
}

function SettingsForm(props: {
  initial: Settings
  currencies: { code: string; symbol: string; decimals: number; label: string }[]
  tab: Tab
  onSaved: () => void
}) {
  const { refresh } = useSession()
  const toast = useToast()
  const [form, setForm] = useState<Settings>(props.initial)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [dirty, setDirty] = useState(false)

  useEffect(() => { setForm(props.initial); setDirty(false) }, [props.initial])

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => {
    setForm((f) => ({ ...f, [k]: v }))
    setDirty(true)
  }

  async function save() {
    setBusy(true)
    setError(null)
    try {
      await Config.save(form)
      await refresh()
      props.onSaved()
      setDirty(false)
      toast.success('Paramètres enregistrés.')
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  async function pickLogo() {
    try {
      const dataUrl = await Host.pickLogo()
      if (dataUrl) set('logoDataUrl', dataUrl)
    } catch (err) {
      toast.error(messageOf(err))
    }
  }

  return (
    <>
      {error && <Alert tone="danger">{error}</Alert>}

      {props.tab === 'company' && (
        <Card title="Identité de l'entreprise" note="Ces informations figurent en tête de chaque facture">
          <div className="stack" style={{ gap: 14 }}>
            <div className="grid grid-2">
              <Field label="Nom commercial" required>
                <TextInput value={form.companyName} onChange={(v) => set('companyName', v)} />
              </Field>
              <Field label="Forme juridique">
                <TextInput value={form.legalForm} onChange={(v) => set('legalForm', v)} placeholder="SARL, SA, entreprise individuelle…" />
              </Field>
            </div>
            <div className="grid grid-2">
              <Field label="NIF / N° contribuable">
                <TextInput value={form.taxId} onChange={(v) => set('taxId', v)} />
              </Field>
              <Field label="RCCM / registre du commerce">
                <TextInput value={form.rccm} onChange={(v) => set('rccm', v)} />
              </Field>
            </div>
            <Field label="Adresse">
              <TextInput value={form.address} onChange={(v) => set('address', v)} />
            </Field>
            <div className="grid grid-2">
              <Field label="Ville">
                <TextInput value={form.city} onChange={(v) => set('city', v)} />
              </Field>
              <Field label="Pays">
                <TextInput value={form.country} onChange={(v) => set('country', v)} />
              </Field>
            </div>
            <div className="grid grid-3">
              <Field label="Téléphone">
                <TextInput value={form.phone} onChange={(v) => set('phone', v)} />
              </Field>
              <Field label="E-mail">
                <TextInput type="email" value={form.email} onChange={(v) => set('email', v)} />
              </Field>
              <Field label="Site web">
                <TextInput value={form.website} onChange={(v) => set('website', v)} />
              </Field>
            </div>

            <div className="divider" />

            <Field label="Logo" hint="Affiché sur les factures. Image de moins de 380 Ko.">
              <div className="row">
                {form.logoDataUrl ? (
                  <img src={form.logoDataUrl} alt="Logo"
                    style={{ height: 52, borderRadius: 7, border: '1px solid var(--rule)', background: '#fff', padding: 4 }} />
                ) : (
                  <div style={{
                    height: 52, width: 52, borderRadius: 7, border: '1px dashed var(--rule)',
                    display: 'grid', placeItems: 'center', color: 'var(--muted)', fontSize: 11,
                  }}>vide</div>
                )}
                <button className="btn" onClick={pickLogo}>Choisir une image…</button>
                {form.logoDataUrl && (
                  <button className="btn btn-ghost" onClick={() => set('logoDataUrl', '')}>Retirer</button>
                )}
              </div>
            </Field>

            <div className="divider" />

            <div className="grid grid-2">
              <Field label="Thème de l'interface">
                <Select
                  value={form.theme}
                  onChange={(v) => set('theme', v)}
                  options={[
                    { value: 'light', label: 'Clair' },
                    { value: 'dark', label: 'Sombre' },
                    { value: 'system', label: 'Selon le système' },
                  ]}
                />
              </Field>
              <Field
                label="Verrouillage après inactivité (minutes)"
                hint="La session se ferme seule au bout de ce délai."
              >
                <NumberInput value={form.sessionTimeoutMin} onChange={(v) => set('sessionTimeoutMin', v)} min={5} max={720} />
              </Field>
            </div>
          </div>
        </Card>
      )}

      {props.tab === 'money' && (
        <div className="stack">
          <Card title="Monnaie">
            <div className="stack" style={{ gap: 14 }}>
              <Field label="Monnaie" hint="Le choix ajuste le symbole et le nombre de décimales.">
                <Select
                  value={form.currency}
                  onChange={(code) => {
                    const preset = props.currencies.find((c) => c.code === code)
                    setForm((f) => ({
                      ...f,
                      currency: code,
                      currencySymbol: preset?.symbol ?? code,
                      decimals: preset?.decimals ?? f.decimals,
                    }))
                    setDirty(true)
                  }}
                  options={props.currencies.map((c) => ({ value: c.code, label: `${c.label} (${c.symbol})` }))}
                />
              </Field>
              <div className="grid grid-2">
                <Field label="Symbole affiché">
                  <TextInput value={form.currencySymbol} onChange={(v) => set('currencySymbol', v)} />
                </Field>
                <Field label="Décimales" hint="0 pour le franc CFA, 2 pour l'euro ou le dollar.">
                  <NumberInput value={form.decimals} onChange={(v) => set('decimals', v)} min={0} max={2} />
                </Field>
              </div>
            </div>
          </Card>

          <Card title="Taxes">
            <div className="stack" style={{ gap: 14 }}>
              <Field
                label="Taux de taxe par défaut (%)"
                hint="Appliqué aux lignes dont le taux n'est pas précisé. Une ligne peut toujours être exonérée en saisissant 0."
              >
                <input className="num" type="number" min={0} max={100} step="0.5"
                  value={form.defaultTaxRate}
                  onChange={(e) => set('defaultTaxRate', parseFloat(e.target.value) || 0)} />
              </Field>
              <Checkbox
                checked={form.pricesIncludeTax}
                onChange={(v) => set('pricesIncludeTax', v)}
                label="Les prix saisis sont TTC"
                hint="La base hors taxe est alors extraite du prix affiché, au lieu d'y être ajoutée."
              />
              <Field label="Mois de début d'exercice" hint="1 pour janvier.">
                <NumberInput value={form.fiscalYearStartMonth} onChange={(v) => set('fiscalYearStartMonth', v)} min={1} max={12} />
              </Field>
            </div>
          </Card>
        </div>
      )}

      {props.tab === 'documents' && (
        <div className="stack">
          <Card title="Numérotation">
            <div className="grid grid-2">
              <Field label="Préfixe des factures" hint={`Prochaine : ${form.invoicePrefix}-${new Date().getFullYear()}-${String(form.invoiceCounter + 1).padStart(4, '0')}`}>
                <TextInput value={form.invoicePrefix} onChange={(v) => set('invoicePrefix', v)} maxLength={6} />
              </Field>
              <Field label="Préfixe des bons d'entrée" hint={`Prochain : ${form.purchasePrefix}-${new Date().getFullYear()}-${String(form.purchaseCounter + 1).padStart(4, '0')}`}>
                <TextInput value={form.purchasePrefix} onChange={(v) => set('purchasePrefix', v)} maxLength={6} />
              </Field>
            </div>
            <p className="small muted" style={{ marginTop: 12, lineHeight: 1.5 }}>
              Les compteurs ne se modifient pas à la main : les abaisser redistribuerait des numéros
              déjà attribués. Une remise à zéro en début d'exercice est possible tant qu'aucun
              document de l'année en cours n'existe.
            </p>
          </Card>

          <Card title="Mentions sur les factures">
            <div className="stack" style={{ gap: 14 }}>
              <Field label="Conditions de vente" hint="Imprimées en bas de chaque facture.">
                <TextArea value={form.invoiceTerms} onChange={(v) => set('invoiceTerms', v)} rows={3} />
              </Field>
              <Field label="Formule de fin">
                <TextInput value={form.invoiceFooter} onChange={(v) => set('invoiceFooter', v)} />
              </Field>
            </div>
          </Card>
        </div>
      )}

      {dirty && (
        <div className="row" style={{
          position: 'sticky', bottom: 0, padding: 12, background: 'var(--surface)',
          border: '1px solid var(--rule)', borderRadius: 'var(--radius)', boxShadow: 'var(--shadow)',
        }}>
          <span className="small muted">Modifications non enregistrées.</span>
          <div className="spacer" />
          <button className="btn" onClick={() => { setForm(props.initial); setDirty(false) }} disabled={busy}>
            Annuler
          </button>
          <button className="btn btn-primary" onClick={save} disabled={busy}>
            {busy ? 'Enregistrement…' : 'Enregistrer'}
          </button>
        </div>
      )}
    </>
  )
}

// --- Sauvegardes -----------------------------------------------------------

function BackupsTab() {
  const toast = useToast()
  const backups = useAsync(() => Backups.list(), [])
  const location = useAsync(() => Config.dataLocation(), [])
  const settings = useAsync(() => Config.get(), [])
  const [busy, setBusy] = useState(false)
  const [restoring, setRestoring] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<string | null>(null)

  async function create() {
    setBusy(true)
    try {
      const info = await Backups.create('manuel')
      toast.success(`Sauvegarde créée : ${info.name}`)
      backups.reload()
    } catch (err) {
      toast.error(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  async function restore() {
    if (!restoring) return
    setBusy(true)
    try {
      await Backups.restore(restoring)
      setRestoring(null)
      toast.success('Données restaurées. Comptoir va se fermer : rouvrez-le pour continuer.')
      window.setTimeout(() => { Host.quit().catch(() => {}) }, 2500)
    } catch (err) {
      toast.error(messageOf(err))
      setRestoring(null)
      setBusy(false)
    }
  }

  async function restoreFromDisk() {
    try {
      const path = await Host.pickBackupArchive()
      if (!path) return
      setBusy(true)
      await Backups.restoreFromPath(path)
      toast.success('Données restaurées. Comptoir va se fermer : rouvrez-le pour continuer.')
      window.setTimeout(() => { Host.quit().catch(() => {}) }, 2500)
    } catch (err) {
      toast.error(messageOf(err))
      setBusy(false)
    }
  }

  async function remove() {
    if (!deleting) return
    setBusy(true)
    try {
      await Backups.remove(deleting)
      toast.success('Sauvegarde supprimée.')
      setDeleting(null)
      backups.reload()
    } catch (err) {
      toast.error(messageOf(err))
      setDeleting(null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="stack">
      <Alert tone="info">
        Une sauvegarde est une archive ZIP de toutes vos données. Elle est créée automatiquement
        au premier démarrage de chaque journée. Copiez-en régulièrement une sur une clé USB :
        une sauvegarde qui vit sur le même disque que les données ne protège pas d'une panne de ce disque.
      </Alert>

      <div className="row">
        <button className="btn btn-primary" onClick={create} disabled={busy}>
          <IconRefresh />Sauvegarder maintenant
        </button>
        <button className="btn" onClick={restoreFromDisk} disabled={busy}>
          Restaurer depuis un fichier…
        </button>
        <div className="spacer" />
        <button className="btn" onClick={() => Host.openDataFolder('backups').catch(() => {})}>
          <IconFolder />Ouvrir le dossier
        </button>
      </div>

      <Card
        title="Archives disponibles"
        note={settings.data ? `Les ${settings.data.backupsToKeep} plus récentes sont conservées` : undefined}
        flush
      >
        {backups.loading ? <Loading /> : (
          <DataTable
            rows={backups.data ?? []}
            rowKey={(b) => b.name}
            empty={<Empty title="Aucune sauvegarde" text="Créez-en une dès maintenant." />}
            columns={[
              { key: 'name', header: 'Archive', render: (b) => <span className="mono small">{b.name}</span> },
              { key: 'date', header: 'Créée le', render: (b) => formatDateTime(b.createdAt) },
              { key: 'size', header: 'Taille', align: 'right', render: (b) => formatBytes(b.sizeBytes) },
              {
                key: 'actions', header: '', align: 'right', width: 190,
                render: (b) => (
                  <div className="row" style={{ justifyContent: 'flex-end', gap: 4 }}>
                    <button className="btn btn-sm" onClick={() => setRestoring(b.name)} disabled={busy}>Restaurer</button>
                    <button className="btn btn-sm btn-ghost" onClick={() => setDeleting(b.name)} disabled={busy}>
                      <IconTrash size={13} />
                    </button>
                  </div>
                ),
              },
            ]}
          />
        )}
      </Card>

      {settings.data && (
        <Card title="Sauvegarde automatique">
          <div className="stack" style={{ gap: 14 }}>
            <Checkbox
              checked={settings.data.autoBackup}
              onChange={async (v) => {
                try {
                  await Config.save({ ...settings.data!, autoBackup: v })
                  settings.reload()
                  toast.success(v ? 'Sauvegarde automatique activée.' : 'Sauvegarde automatique désactivée.')
                } catch (err) {
                  toast.error(messageOf(err))
                }
              }}
              label="Sauvegarder automatiquement au premier démarrage de chaque journée"
              hint="L'archive reflète alors la journée précédente complète."
            />
            <Field label="Nombre d'archives à conserver" hint="Les plus anciennes sont supprimées au-delà.">
              <NumberInput
                value={settings.data.backupsToKeep}
                min={1} max={500}
                onChange={async (v) => {
                  if (v < 1) return
                  try {
                    await Config.save({ ...settings.data!, backupsToKeep: v })
                    settings.reload()
                  } catch (err) {
                    toast.error(messageOf(err))
                  }
                }}
              />
            </Field>
          </div>
        </Card>
      )}

      {location.data && (
        <Card title="Emplacement des données">
          <div className="stack" style={{ gap: 8 }}>
            {Object.entries(location.data).map(([key, path]) => (
              <div className="row" key={key} style={{ justifyContent: 'space-between' }}>
                <span className="small muted" style={{ textTransform: 'capitalize' }}>{key}</span>
                <span className="mono small truncate" data-selectable style={{ maxWidth: 520 }}>{path}</span>
              </div>
            ))}
            <p className="small muted" style={{ marginTop: 6, lineHeight: 1.5 }}>
              Les données sont des fichiers JSON lisibles : elles restent récupérables même sans
              Comptoir. Le chiffrement du disque relève du système d'exploitation.
            </p>
          </div>
        </Card>
      )}

      {restoring && (
        <ConfirmDialog
          title="Restaurer cette sauvegarde ?"
          danger busy={busy} confirmLabel="Restaurer"
          message={
            <>
              <p>Toutes les données actuelles seront remplacées par celles de « {restoring} ».</p>
              <p style={{ marginTop: 8 }}>
                Une sauvegarde de sécurité de l'état actuel est prise avant l'opération.
                Comptoir se fermera ensuite : il faudra le rouvrir.
              </p>
            </>
          }
          onConfirm={restore}
          onCancel={() => setRestoring(null)}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title="Supprimer cette sauvegarde ?"
          danger busy={busy} confirmLabel="Supprimer"
          message={`L'archive « ${deleting} » sera effacée définitivement du disque.`}
          onConfirm={remove}
          onCancel={() => setDeleting(null)}
        />
      )}
    </div>
  )
}
