// Assistant de premier démarrage.
//
// Sept étapes, dans l'ordre où les questions se posent réellement à quelqu'un
// qui ouvre une boutique : qui tient la caisse, quelle entreprise, quelle
// monnaie, quel catalogue, quelles sauvegardes. Rien n'est écrit avant la
// dernière étape — on peut revenir en arrière autant qu'on veut, et fermer
// l'application en cours de route ne laisse pas un poste à moitié configuré.

import { useMemo, useState } from 'react'
import { Session, messageOf } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { Alert, Badge, Checkbox, Field, Select, TextArea, TextInput } from '../components/UI'
import { IconCheck } from '../components/Icons'
import type { SetupInput } from '../lib/types'

type StepKey = 'welcome' | 'account' | 'company' | 'money' | 'catalog' | 'backup' | 'review'

const STEPS: { key: StepKey; title: string; short: string }[] = [
  { key: 'welcome', title: 'Bienvenue', short: 'Bienvenue' },
  { key: 'account', title: 'Le compte qui tient la caisse', short: 'Compte' },
  { key: 'company', title: 'Votre entreprise', short: 'Entreprise' },
  { key: 'money', title: 'Monnaie et taxes', short: 'Monnaie' },
  { key: 'catalog', title: 'Catalogue de départ', short: 'Catalogue' },
  { key: 'backup', title: 'Sauvegardes', short: 'Sauvegardes' },
  { key: 'review', title: 'Tout est prêt', short: 'Récapitulatif' },
]

const empty: SetupInput = {
  username: '', fullName: '', password: '',
  companyName: '', legalForm: '', taxId: '', rccm: '',
  address: '', city: '', country: 'Mali', phone: '', email: '',
  currency: 'XOF', currencySymbol: 'FCFA', decimals: 0,
  defaultTaxRate: 18, pricesIncludeTax: false,
  seedCategories: true,
  autoBackup: true, backupsToKeep: 30,
  theme: 'light',
}

export function Onboarding() {
  const { setState } = useSession()
  const [index, setIndex] = useState(0)
  const [form, setForm] = useState<SetupInput>(empty)
  const [confirmPwd, setConfirmPwd] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const brand = useAsync(() => Session.brand(), [])
  const currencies = useAsync(() => Session.currencies(), [])
  const categories = useAsync(() => Session.defaultCategories(), [])

  const step = STEPS[index]
  const set = <K extends keyof SetupInput>(k: K, v: SetupInput[K]) =>
    setForm((f) => ({ ...f, [k]: v }))

  // Chaque étape dit elle-même si elle est complète : le bouton « Suivant »
  // reste inactif tant qu'il manque quelque chose, plutôt que d'accepter puis
  // de refuser à la fin.
  const blocker = useMemo<string | null>(() => {
    switch (step.key) {
      case 'account':
        if (form.fullName.trim() === '') return 'Indiquez votre nom complet.'
        if (form.username.trim().length < 3) return "L'identifiant doit contenir au moins 3 caractères."
        if (form.password.length < 8) return 'Le mot de passe doit contenir au moins 8 caractères.'
        if (!/[a-zA-Z]/.test(form.password) || !/[0-9]/.test(form.password)) {
          return 'Le mot de passe doit contenir au moins une lettre et un chiffre.'
        }
        if (form.password !== confirmPwd) return 'Les deux mots de passe ne correspondent pas.'
        return null
      case 'company':
        if (form.companyName.trim() === '') return "Le nom de l'entreprise est obligatoire."
        return null
      case 'money':
        if (form.defaultTaxRate < 0 || form.defaultTaxRate > 100) {
          return 'Le taux de taxe doit être compris entre 0 et 100 %.'
        }
        return null
      default:
        return null
    }
  }, [step.key, form, confirmPwd])

  async function finish() {
    setBusy(true)
    setError(null)
    try {
      setState(await Session.setup(form))
    } catch (err) {
      setError(messageOf(err))
      setBusy(false)
    }
  }

  return (
    <div className="gate" style={{ alignItems: 'stretch', padding: 0 }}>
      <div className="wizard">
        <aside className="wizard-rail">
          <div className="wizard-brand">
            {brand.data && (
              <img src={brand.data.logoDataUrl} alt="SISKO" className="wizard-logo" />
            )}
            <div className="wizard-product">Comptoir</div>
            <div className="wizard-author">
              {brand.data ? `un logiciel ${brand.data.author}` : ''}
            </div>
          </div>

          <ol className="wizard-steps">
            {STEPS.map((s, i) => (
              <li
                key={s.key}
                className={`wizard-step ${i === index ? 'active' : ''} ${i < index ? 'done' : ''}`}
              >
                <span className="wizard-step-mark">
                  {i < index ? <IconCheck size={12} /> : i + 1}
                </span>
                {s.short}
              </li>
            ))}
          </ol>

          <div className="wizard-rail-foot">
            Étape {index + 1} sur {STEPS.length}
          </div>
        </aside>

        <section className="wizard-body">
          <header className="wizard-head">
            <h1 className="gate-title">{step.title}</h1>
          </header>

          <div className="wizard-content">
            {error && <Alert tone="danger">{error}</Alert>}

            {step.key === 'welcome' && (
              <div className="stack">
                <p className="wizard-lead">
                  Comptoir tient le stock, les ventes, les achats et les comptes de votre boutique.
                  Cette configuration prend deux minutes et ne se refait pas.
                </p>
                <div className="wizard-facts">
                  <Fact title="Tout reste chez vous">
                    Aucune donnée ne part sur internet. Il n'y a ni compte à créer, ni abonnement,
                    ni connexion requise pour travailler.
                  </Fact>
                  <Fact title="Vos données vous appartiennent">
                    Elles sont enregistrées en fichiers lisibles sur ce poste, et restent
                    exploitables même sans ce logiciel.
                  </Fact>
                  <Fact title="Rien ne se perd">
                    Une sauvegarde complète est créée automatiquement au premier démarrage de
                    chaque journée.
                  </Fact>
                  <Fact title="Chacun son rôle">
                    Administrateur, gérant, vendeur. Un vendeur ne voit ni les prix d'achat,
                    ni les marges, ni les charges.
                  </Fact>
                </div>
                {brand.data && !brand.data.intact && (
                  <Alert tone="warn">
                    L'identité visuelle de l'éditeur a été modifiée dans cette copie.
                    Voir {brand.data.licenseRef}.
                  </Alert>
                )}
              </div>
            )}

            {step.key === 'account' && (
              <div className="stack" style={{ gap: 14 }}>
                <p className="wizard-lead">
                  Ce premier compte est administrateur : lui seul pourra ensuite créer les
                  comptes de vos vendeurs et gérants.
                </p>
                <Field label="Votre nom complet" required>
                  <TextInput value={form.fullName} onChange={(v) => set('fullName', v)}
                    autoFocus placeholder="Aïssata Traoré" />
                </Field>
                <div className="grid grid-2">
                  <Field label="Identifiant de connexion" required
                    hint="Court et sans espace : il se tape plusieurs fois par jour.">
                    <TextInput value={form.username}
                      onChange={(v) => set('username', v.toLowerCase().replace(/\s/g, ''))}
                      placeholder="aissata" />
                  </Field>
                  <Field label="Nom affiché sur les documents">
                    <TextInput value={form.fullName} onChange={(v) => set('fullName', v)} disabled />
                  </Field>
                </div>
                <div className="grid grid-2">
                  <Field label="Mot de passe" required
                    hint="8 caractères minimum, dont une lettre et un chiffre.">
                    <TextInput type="password" value={form.password} onChange={(v) => set('password', v)} />
                  </Field>
                  <Field label="Confirmer le mot de passe" required>
                    <TextInput type="password" value={confirmPwd} onChange={setConfirmPwd} />
                  </Field>
                </div>
                <Alert tone="info">
                  Ce mot de passe n'est enregistré nulle part en clair, et ne peut pas être
                  retrouvé. Notez-le quelque part de sûr avant de continuer.
                </Alert>
              </div>
            )}

            {step.key === 'company' && (
              <div className="stack" style={{ gap: 14 }}>
                <p className="wizard-lead">
                  Ces informations apparaissent en tête de vos factures. Tout sauf le nom peut
                  être complété plus tard.
                </p>
                <div className="grid grid-2">
                  <Field label="Nom commercial" required>
                    <TextInput value={form.companyName} onChange={(v) => set('companyName', v)}
                      autoFocus placeholder="Sahel Informatique" />
                  </Field>
                  <Field label="Forme juridique">
                    <TextInput value={form.legalForm} onChange={(v) => set('legalForm', v)}
                      placeholder="SARL, entreprise individuelle…" />
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
                  <TextArea value={form.address} onChange={(v) => set('address', v)} rows={2} />
                </Field>
                <div className="grid grid-3">
                  <Field label="Ville">
                    <TextInput value={form.city} onChange={(v) => set('city', v)} />
                  </Field>
                  <Field label="Pays">
                    <TextInput value={form.country} onChange={(v) => set('country', v)} />
                  </Field>
                  <Field label="Téléphone">
                    <TextInput value={form.phone} onChange={(v) => set('phone', v)} />
                  </Field>
                </div>
                <Field label="E-mail">
                  <TextInput type="email" value={form.email} onChange={(v) => set('email', v)} />
                </Field>
              </div>
            )}

            {step.key === 'money' && (
              <div className="stack" style={{ gap: 14 }}>
                <p className="wizard-lead">
                  Le choix de la monnaie fixe le symbole affiché et le nombre de décimales.
                  Le franc CFA n'en utilise pas.
                </p>
                <Field label="Monnaie" required>
                  <Select
                    value={form.currency}
                    onChange={(code) => {
                      const preset = (currencies.data ?? []).find((c) => c.code === code)
                      setForm((f) => ({
                        ...f, currency: code,
                        currencySymbol: preset?.symbol ?? code,
                        decimals: preset?.decimals ?? f.decimals,
                      }))
                    }}
                    options={(currencies.data ?? []).map((c) => ({
                      value: c.code, label: `${c.label} — ${c.symbol}`,
                    }))}
                  />
                </Field>
                <div className="grid grid-2">
                  <Field label="Symbole affiché">
                    <TextInput value={form.currencySymbol} onChange={(v) => set('currencySymbol', v)} />
                  </Field>
                  <Field label="Décimales" hint="0 pour le franc CFA, 2 pour l'euro.">
                    <Select
                      value={String(form.decimals)}
                      onChange={(v) => set('decimals', parseInt(v, 10))}
                      options={[
                        { value: '0', label: 'Aucune — 1 500' },
                        { value: '2', label: 'Deux — 1 500,00' },
                      ]}
                    />
                  </Field>
                </div>
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
                  label="Mes prix de vente sont affichés taxe comprise"
                  hint="La base hors taxe est alors extraite du prix, au lieu d'y être ajoutée."
                />
                <div className="wizard-preview">
                  <span className="muted small">Un article à 1 500 s'affichera</span>
                  <strong className="tabular">
                    1 500{form.decimals > 0 ? ',00' : ''} {form.currencySymbol}
                  </strong>
                  <span className="muted small">
                    {form.pricesIncludeTax ? 'taxe comprise' : `+ ${form.defaultTaxRate} % de taxe`}
                  </span>
                </div>
              </div>
            )}

            {step.key === 'catalog' && (
              <div className="stack" style={{ gap: 14 }}>
                <p className="wizard-lead">
                  Les catégories servent à classer vos articles et à lire vos ventes par famille.
                  Vous pourrez les renommer, en ajouter ou en supprimer à tout moment.
                </p>
                <Checkbox
                  checked={form.seedCategories}
                  onChange={(v) => set('seedCategories', v)}
                  label="Créer des catégories de départ"
                  hint="Recommandé : il est plus rapide de renommer que de partir de rien."
                />
                {form.seedCategories && (
                  <div className="row row-wrap" style={{ gap: 6 }}>
                    {(categories.data ?? []).map((c) => <Badge key={c} tone="blue">{c}</Badge>)}
                  </div>
                )}
                <Alert tone="info">
                  Vos articles s'ajoutent ensuite un par un, ou d'un coup en important le fichier
                  de votre tableur — depuis l'écran Articles, bouton « Importer ».
                </Alert>
              </div>
            )}

            {step.key === 'backup' && (
              <div className="stack" style={{ gap: 14 }}>
                <p className="wizard-lead">
                  Une sauvegarde est une archive complète de vos données. Elle se restaure en
                  deux clics depuis les paramètres.
                </p>
                <Checkbox
                  checked={form.autoBackup}
                  onChange={(v) => set('autoBackup', v)}
                  label="Sauvegarder automatiquement au premier démarrage de chaque journée"
                  hint="L'archive reflète alors la journée précédente complète."
                />
                <Field label="Nombre d'archives à conserver"
                  hint="Au-delà, les plus anciennes sont supprimées.">
                  <input className="num" type="number" min={1} max={500}
                    value={form.backupsToKeep}
                    onChange={(e) => set('backupsToKeep', parseInt(e.target.value, 10) || 1)} />
                </Field>
                <Field label="Apparence">
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
                <Alert tone="warn">
                  Une sauvegarde qui vit sur le même disque que les données ne protège pas d'une
                  panne de ce disque. Copiez-en une sur une clé USB de temps en temps.
                </Alert>
              </div>
            )}

            {step.key === 'review' && (
              <div className="stack" style={{ gap: 14 }}>
                <p className="wizard-lead">
                  Vérifiez, puis lancez la boutique. Tout reste modifiable ensuite dans les
                  paramètres — sauf l'identifiant de connexion.
                </p>
                <div className="wizard-review">
                  <ReviewRow label="Entreprise" value={form.companyName} />
                  <ReviewRow label="Adresse"
                    value={[form.address, form.city, form.country].filter(Boolean).join(', ') || '—'} />
                  <ReviewRow label="Administrateur"
                    value={`${form.fullName} (${form.username})`} />
                  <ReviewRow label="Monnaie"
                    value={`${form.currencySymbol} — ${form.decimals} décimale${form.decimals > 1 ? 's' : ''}`} />
                  <ReviewRow label="Taxe par défaut"
                    value={`${form.defaultTaxRate} %${form.pricesIncludeTax ? ', prix TTC' : ', prix HT'}`} />
                  <ReviewRow label="Catégories de départ"
                    value={form.seedCategories ? `${(categories.data ?? []).length} créées` : 'aucune'} />
                  <ReviewRow label="Sauvegarde automatique"
                    value={form.autoBackup ? `oui, ${form.backupsToKeep} archives conservées` : 'non'} />
                </div>
              </div>
            )}
          </div>

          <footer className="wizard-foot">
            {index > 0 && (
              <button className="btn" onClick={() => { setIndex(index - 1); setError(null) }} disabled={busy}>
                Retour
              </button>
            )}
            <div className="spacer" />
            {blocker && <span className="small muted">{blocker}</span>}
            {index < STEPS.length - 1 ? (
              <button
                className="btn btn-primary"
                onClick={() => { setIndex(index + 1); setError(null) }}
                disabled={!!blocker}
              >
                {index === 0 ? 'Commencer' : 'Suivant'}
              </button>
            ) : (
              <button className="btn btn-primary" onClick={finish} disabled={busy}>
                {busy ? 'Configuration…' : 'Ouvrir la boutique'}
              </button>
            )}
          </footer>
        </section>
      </div>
    </div>
  )
}

function Fact({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="wizard-fact">
      <div className="wizard-fact-title">{title}</div>
      <p className="wizard-fact-text">{children}</p>
    </div>
  )
}

function ReviewRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="wizard-review-row">
      <span className="muted small">{label}</span>
      <strong>{value}</strong>
    </div>
  )
}
