// Aperçu d'un document produit par le backend, avec enregistrement.
//
// Le PDF est affiché tel qu'il sera imprimé : ce que l'on voit est exactement
// le fichier, et non une reconstitution HTML qui pourrait en différer.

import { useState } from 'react'
import { Export, dataURL, messageOf, saveDocument } from '../lib/api'
import type { FileResult } from '../lib/types'
import { useToast } from '../lib/toast'
import { Modal } from './UI'
import { IconDownload } from './Icons'

export function DocumentPreview({ file, title, onClose }: {
  file: FileResult
  title: string
  onClose: () => void
}) {
  const toast = useToast()
  const [busy, setBusy] = useState(false)

  async function save() {
    setBusy(true)
    try {
      const path = await saveDocument(file)
      if (path) toast.success(`Enregistré dans ${path}`)
    } catch (err) {
      toast.error(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  // Dépôt sans dialogue, toujours au même endroit : c'est ce qu'on veut quand
  // on imprime vingt factures d'affilée.
  async function saveToExports() {
    setBusy(true)
    try {
      toast.success(`Enregistré dans ${await Export.save(file)}`)
    } catch (err) {
      toast.error(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={title}
      subtitle={file.name}
      size="wide"
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={onClose}>Fermer</button>
          <button className="btn" onClick={saveToExports} disabled={busy} title="Dossier exports de Comptoir">
            Dossier exports
          </button>
          <button className="btn btn-primary" onClick={save} disabled={busy}>
            <IconDownload />{busy ? 'Enregistrement…' : 'Enregistrer sous…'}
          </button>
        </>
      }
    >
      <iframe className="pdf-frame" src={dataURL(file)} title={title} />
    </Modal>
  )
}

/**
 * useDocument gère le cycle « produire le document, l'afficher, le fermer ».
 * Les erreurs remontent en notification : une impression qui échoue ne doit
 * pas laisser l'écran figé.
 */
export function useDocumentPreview() {
  const toast = useToast()
  const [file, setFile] = useState<FileResult | null>(null)
  const [title, setTitle] = useState('')
  const [busy, setBusy] = useState(false)

  async function open(label: string, produce: () => Promise<FileResult>) {
    setBusy(true)
    try {
      setTitle(label)
      setFile(await produce())
    } catch (err) {
      toast.error(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  /** Enregistre directement, sans aperçu — pour les exports tableur. */
  async function download(produce: () => Promise<FileResult>) {
    setBusy(true)
    try {
      const result = await produce()
      const path = await saveDocument(result)
      if (path) toast.success(`Enregistré dans ${path}`)
    } catch (err) {
      toast.error(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  const element = file
    ? <DocumentPreview file={file} title={title} onClose={() => setFile(null)} />
    : null

  return { open, download, busy, element }
}
