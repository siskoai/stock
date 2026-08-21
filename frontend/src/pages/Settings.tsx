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
  Loading, Modal, NumberInput, Select, TextArea, TextInput,
} from '../components/UI'
import { IconFolder, IconRefresh, IconTrash } from '../components/Icons'
import type { Settings } from '../lib/types'
import type { PageContext } from '../App'

type Tab = 'company' | 'money' | 'documents' | 'backups' | 'poste'

export function SettingsPage(_: PageContext) {
  const [tab, setTab] = useState<Tab>('company')
  const settings = useAsync(() => Config.get(), [])
  const presets = useAsync(() => Config.presets(), [])

  if (settings.loading) return <Loading />
  if (settings.error) return <Alert tone="danger">{settings.error}</Alert>
  if (!settings.data) return null

  return (
    <div className="content-narrow stack">
      <div className="tabs" style={{ flexWrap: 'wrap' }}>
        <button className={`tab ${tab === 'company' ? 'active' : ''}`} onClick={() => setTab('company')}>Entreprise</button>
        <button className={`tab ${tab === 'money' ? 'active' : ''}`} onClick={() => setTab('money')}>Monnaie et taxes</button>
        <button className={`tab ${tab === 'documents' ? 'active' : ''}`} onClick={() => setTab('documents')}>Documents</button>
        <button className={`tab ${tab === 'backups' ? 'active' : ''}`} onClick={() => setTab('backups')}>Sauvegardes</button>
        <button className={`tab ${tab === 'poste' ? 'active' : ''}`} onClick={() => setTab('poste')}>Ce poste</button>
      </div>

      {tab === 'backups' ? <BackupsTab />
        : tab === 'poste' ? <PosteTab />
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
              <Field
                label="Délai de règlement d'une vente à crédit (jours)"
                hint="Sert à proposer une échéance quand un client emporte sans payer. Zéro pour n'en proposer aucune."
              >
                <NumberInput value={form.defaultPaymentTermDays}
                  onChange={(v) => set('defaultPaymentTermDays', v)} min={0} max={365} />
              </Field>
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

// --- Ce poste ---------------------------------------------------------------

/**
 * Emplacement des données, désinstallation, effacement.
 *
 * Ces trois sujets tiennent ensemble : ils répondent tous à la question
 * « comment je m'en vais ». Ils vivent dans leur propre onglet plutôt qu'au bas
 * des sauvegardes, pour qu'on ne tombe pas sur le bouton d'effacement en
 * cherchant à restaurer.
 */
const EMPLACEMENTS: { cle: string; libelle: string }[] = [
  { cle: 'root', libelle: 'Dossier principal' },
  { cle: 'data', libelle: 'Données' },
  { cle: 'backups', libelle: 'Sauvegardes' },
  { cle: 'exports', libelle: 'Exports' },
]

function PosteTab() {
  const { state } = useSession()
  const toast = useToast()
  const location = useAsync(() => Config.dataLocation(), [])
  const [effacer, setEffacer] = useState(false)

  return (
    <div className="stack">
      <Card title="Emplacement des données">
        <div className="stack" style={{ gap: 8 }}>
          {EMPLACEMENTS.map(({ cle, libelle }) => (
            <div className="ligne-total" key={cle}>
              <span className="ligne-total-libelle small muted">{libelle}</span>
              <span className="mono small truncate" data-selectable
                title={location.data?.[cle]} style={{ maxWidth: 460, direction: 'rtl', textAlign: 'right' }}>
                {location.data?.[cle] ?? '…'}
              </span>
            </div>
          ))}
          <div className="row" style={{ marginTop: 6 }}>
            <button className="btn btn-sm" onClick={() => Host.openDataFolder('root').catch((e) => toast.error(messageOf(e)))}>
              <IconFolder />Ouvrir le dossier
            </button>
          </div>
          <p className="small muted" style={{ marginTop: 4, lineHeight: 1.5 }}>
            Les données sont des fichiers lisibles : elles restent exploitables
            même sans Comptoir. Le chiffrement du disque relève du système
            d'exploitation.
          </p>
        </div>
      </Card>

      <Card title="Désinstaller l'application">
        <div className="stack" style={{ gap: 12 }}>
          <Alert tone="info">
            Désinstaller Comptoir retire le programme, jamais vos données. Elles
            restent dans le dossier ci-dessus, prêtes à être retrouvées par une
            réinstallation ou par une version plus récente.
          </Alert>
          <div>
            <div className="section-title">Windows</div>
            <p className="small">
              Paramètres, Applications, Applications installées, Comptoir,
              Désinstaller. Ou depuis le Panneau de configuration.
            </p>
          </div>
          <div>
            <div className="section-title">macOS</div>
            <p className="small">
              Glissez Comptoir depuis le dossier Applications vers la corbeille.
            </p>
          </div>
          <div>
            <div className="section-title">Linux</div>
            <p className="small">
              Supprimez l'exécutable et le raccourci que vous aviez créés.
            </p>
          </div>
          <p className="small muted" style={{ lineHeight: 1.5 }}>
            Pour partir sans rien laisser, effacez d'abord les données ci-dessous,
            puis désinstallez.
          </p>
        </div>
      </Card>

      <Card title="Effacer toutes les données de ce poste">
        <div className="stack" style={{ gap: 12 }}>
          <Alert tone="danger">
            Articles, ventes, clients, mouvements, charges et comptes sont
            supprimés définitivement. Comptoir repartira sur un premier
            démarrage, comme au premier jour.
          </Alert>
          <p className="small">
            À faire pour repartir d'une base propre après une période d'essai, ou
            pour céder cet ordinateur sans y laisser le fichier de vos clients.
          </p>
          <div className="row">
            <button className="btn btn-danger" onClick={() => setEffacer(true)}>
              <IconTrash />Effacer toutes les données
            </button>
          </div>
        </div>
      </Card>

      {effacer && (
        <EffacementModal
          companyName={state.companyName}
          onClose={() => setEffacer(false)}
        />
      )}
    </div>
  )
}

function EffacementModal(props: { companyName: string; onClose: () => void }) {
  const toast = useToast()
  const [confirmation, setConfirmation] = useState('')
  const [garder, setGarder] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const correspond = confirmation.trim().toLowerCase() === props.companyName.trim().toLowerCase()

  async function effacer() {
    setBusy(true)
    setError(null)
    try {
      const resultat = await Config.eraseAllData({
        confirmation, garderUneSauvegarde: garder,
      })
      toast.success(resultat.sauvegarde
        ? `Données effacées. Une dernière sauvegarde a été conservée dans ${resultat.sauvegarde}`
        : 'Toutes les données ont été effacées.')
      window.setTimeout(() => { Host.quit().catch(() => {}) }, 3000)
    } catch (err) {
      setError(messageOf(err))
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Effacer toutes les données"
      subtitle="Cette opération est irréversible"
      onClose={props.onClose}
      footer={
        <>
          <button className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          <button className="btn btn-danger" onClick={effacer} disabled={busy || !correspond}>
            {busy ? 'Effacement…' : 'Effacer définitivement'}
          </button>
        </>
      }
    >
      <div className="stack" style={{ gap: 14 }}>
        {error && <Alert tone="danger">{error}</Alert>}
        <Alert tone="danger">
          Tout sera supprimé : articles, stock, factures, clients, charges,
          comptes et journal d'audit. Il n'y a pas de retour en arrière.
        </Alert>
        <Checkbox
          checked={garder}
          onChange={setGarder}
          label="Conserver une dernière sauvegarde"
          hint="Décochez seulement pour céder ou mettre au rebut cet ordinateur : il ne resterait alors plus rien à récupérer."
        />
        <Field
          label="Saisissez le nom de votre entreprise pour confirmer"
          required
          hint={`Exactement : ${props.companyName}`}
        >
          <TextInput value={confirmation} onChange={setConfirmation} autoFocus />
        </Field>
      </div>
    </Modal>
  )
}
