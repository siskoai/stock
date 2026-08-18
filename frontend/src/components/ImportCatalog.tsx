// Import d'un catalogue depuis un tableur.
//
// Le fichier est d'abord analysé sans rien écrire : l'utilisateur voit
// exactement ce qui sera créé, mis à jour et écarté (et pourquoi) avant de
// confirmer. Reprendre un stock existant est le premier obstacle à l'adoption
// du logiciel ; il ne doit pas se franchir à l'aveugle.

import { useState } from 'react'
import { Catalog, Host, messageOf, saveDocument } from '../lib/api'
import { useToast } from '../lib/toast'
import { Alert, Badge, DataTable, Empty, Loading, Modal } from '../components/UI'
import { IconDownload } from './Icons'
import type { ImportReport } from '../lib/types'

export function ImportCatalog(props: { onClose: () => void; onImported: () => void }) {
  const toast = useToast()
  const [content, setContent] = useState('')
  const [preview, setPreview] = useState<ImportReport | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function choose() {
    setBusy(true)
    setError(null)
    try {
      const text = await Host.pickCatalogFile()
      if (!text) return
      setContent(text)
      setPreview(await Catalog.importProducts(text, false))
    } catch (err) {
      setError(messageOf(err))
      setPreview(null)
    } finally {
      setBusy(false)
    }
  }

  async function apply() {
    setBusy(true)
    setError(null)
    try {
      const report = await Catalog.importProducts(content, true)
      toast.success(
        `${report.created} article(s) créé(s), ${report.updated} mis à jour.`)
      props.onImported()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  async function template() {
    try {
      const file = await Catalog.importTemplate()
      const path = await saveDocument(file)
      if (path) toast.success(`Modèle enregistré dans ${path}`)
    } catch (err) {
      toast.error(messageOf(err))
    }
  }

  const tone = (action: string) =>
    action === 'CREATE' ? 'green' : action === 'UPDATE' ? 'blue' : 'red'
  const label = (action: string) =>
    action === 'CREATE' ? 'Création' : action === 'UPDATE' ? 'Mise à jour' : 'Écartée'

  return (
    <Modal
      title="Importer un catalogue"
      subtitle="Un fichier CSV exporté de votre tableur. Rien n'est écrit avant votre confirmation."
      size="xwide"
      onClose={props.onClose}
      footer={
        <>
          <button className="btn" onClick={template} disabled={busy}>
            <IconDownload />Télécharger un modèle
          </button>
          <div className="spacer" />
          <button className="btn" onClick={props.onClose} disabled={busy}>Annuler</button>
          {preview && (
            <button
              className="btn btn-primary"
              onClick={apply}
              disabled={busy || preview.created + preview.updated === 0}
            >
              {busy
                ? 'Import en cours…'
                : `Importer ${preview.created + preview.updated} article(s)`}
            </button>
          )}
        </>
      }
    >
      <div className="stack">
        {error && <Alert tone="danger">{error}</Alert>}

        {!preview && !busy && (
          <>
            <Alert tone="info">
              Les colonnes sont reconnues par leur intitulé, dans n'importe quel ordre :
              « Désignation » (obligatoire), « Référence », « Catégorie », « Prix de vente »,
              « Coût d'achat », « Stock », « Seuil d'alerte », « Marque », « Code-barres »…
              Les colonnes inconnues sont ignorées.
            </Alert>
            <div className="empty" style={{ border: '1px dashed var(--rule)', borderRadius: 'var(--radius)' }}>
              <div className="empty-title">Choisissez le fichier</div>
              <p className="empty-text">
                Dans Excel ou LibreOffice : « Enregistrer sous », format CSV.
                Le séparateur est reconnu automatiquement.
              </p>
              <button className="btn btn-primary" onClick={choose}>Parcourir…</button>
            </div>
          </>
        )}

        {busy && !preview && <Loading label="Analyse du fichier…" />}

        {preview && (
          <>
            <div className="grid grid-4">
              <Stat label="Créations" value={preview.created} tone="green" />
              <Stat label="Mises à jour" value={preview.updated} tone="blue" />
              <Stat label="Lignes écartées" value={preview.skipped} tone={preview.skipped ? 'red' : 'muted'} />
              <Stat label="Catégories créées" value={preview.categoriesCreated.length} tone="muted" />
            </div>

            {preview.categoriesCreated.length > 0 && (
              <Alert tone="info">
                Catégories qui seront créées : {preview.categoriesCreated.join(', ')}.
              </Alert>
            )}
            {preview.ignored.length > 0 && (
              <Alert tone="warn">
                Colonnes non reconnues, donc ignorées : {preview.ignored.join(', ')}.
              </Alert>
            )}
            {preview.updated > 0 && (
              <Alert tone="info">
                Les articles dont la référence existe déjà sont mis à jour. Leur stock et leur
                coût moyen ne changent pas : ils découlent des mouvements enregistrés, pas d'un fichier.
              </Alert>
            )}

            <DataTable
              rows={preview.rows}
              rowKey={(r) => String(r.line)}
              dimmed={(r) => r.action === 'SKIP'}
              empty={<Empty title="Aucune ligne exploitable" />}
              columns={[
                { key: 'line', header: 'Ligne', width: 70, align: 'right', render: (r) => r.line },
                { key: 'action', header: 'Action', width: 120, render: (r) => <Badge tone={tone(r.action)}>{label(r.action)}</Badge> },
                { key: 'sku', header: 'Référence', width: 140, render: (r) => <span className="mono small">{r.sku || '-'}</span> },
                { key: 'name', header: 'Désignation', render: (r) => r.name || <span className="muted">-</span> },
                { key: 'qty', header: 'Stock', width: 80, align: 'right', render: (r) => r.quantity || <span className="muted">-</span> },
                { key: 'message', header: 'Détail', render: (r) => <span className="small muted">{r.message}</span> },
              ]}
            />

            <button className="btn" onClick={choose} disabled={busy}>Choisir un autre fichier…</button>
          </>
        )}
      </div>
    </Modal>
  )
}

function Stat({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div className="card kpi">
      <div className="kpi-label">{label}</div>
      <div className="kpi-value" style={{ color: value > 0 ? `var(--${tone})` : undefined }}>{value}</div>
    </div>
  )
}
