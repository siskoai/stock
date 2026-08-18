// Catégories : le classement du catalogue, avec ce qu'il pèse.

import { useState } from 'react'
import { Catalog, messageOf } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { useToast } from '../lib/toast'
import { formatNumber } from '../lib/money'
import {
  Alert, Badge, Card, ConfirmDialog, DataTable, Empty, Field,
  Loading, Modal, Select, TextArea, TextInput,
} from '../components/UI'
import { IconPlus } from '../components/Icons'
import type { CategoryView } from '../lib/types'
import type { PageContext } from '../App'

const COLORS = [
  { value: 'blue', label: 'Bleu' },
  { value: 'aqua', label: 'Turquoise' },
  { value: 'green', label: 'Vert' },
  { value: 'violet', label: 'Violet' },
  { value: 'orange', label: 'Orange' },
  { value: 'yellow', label: 'Jaune' },
  { value: 'magenta', label: 'Magenta' },
  { value: 'red', label: 'Rouge' },
  { value: 'muted', label: 'Gris' },
]

export function CategoriesPage(_: PageContext) {
  const { amount, can } = useSession()
  const toast = useToast()
  const categories = useAsync(() => Catalog.listCategories(), [])
  const [editing, setEditing] = useState<CategoryView | 'new' | null>(null)
  const [deleting, setDeleting] = useState<CategoryView | null>(null)
  const [busy, setBusy] = useState(false)

  async function remove() {
    if (!deleting) return
    setBusy(true)
    try {
      await Catalog.deleteCategory(deleting.id)
      toast.success('Catégorie supprimée.')
      setDeleting(null)
      categories.reload()
    } catch (err) {
      toast.error(messageOf(err))
      setDeleting(null)
    } finally {
      setBusy(false)
    }
  }

  const rows = categories.data ?? []
  const showCost = can('finance')

  return (
    <div className="stack">
      {can('catalog') && (
        <div className="row">
          <div className="spacer" />
          <button className="btn btn-primary" onClick={() => setEditing('new')}>
            <IconPlus />Nouvelle catégorie
          </button>
        </div>
      )}

      {categories.error && <Alert tone="danger">{categories.error}</Alert>}

      <Card flush>
        {categories.loading ? <Loading /> : (
          <DataTable
            rows={rows}
            rowKey={(c) => c.id}
            onRowClick={(c) => setEditing(c)}
            empty={
              <Empty
                title="Aucune catégorie"
                text="Les catégories servent à classer les articles et à lire les ventes par famille."
                action={can('catalog')
                  ? <button className="btn btn-primary" onClick={() => setEditing('new')}><IconPlus />Nouvelle catégorie</button>
                  : undefined}
              />
            }
            columns={[
              {
                key: 'name', header: 'Catégorie',
                render: (c) => (
                  <>
                    <Badge tone={c.color}>{c.name}</Badge>
                    {c.description && <div className="cell-secondary" style={{ marginTop: 3 }}>{c.description}</div>}
                  </>
                ),
              },
              { key: 'count', header: 'Articles', align: 'right', render: (c) => formatNumber(c.productCount) },
              { key: 'units', header: 'Unités en stock', align: 'right', render: (c) => formatNumber(c.stockUnits) },
              ...(showCost ? [{
                key: 'value', header: 'Valeur du stock', align: 'right' as const,
                render: (c: CategoryView) => amount(c.stockValue),
              }] : []),
              {
                key: 'actions', header: '', align: 'right', width: 150,
                render: (c) => can('catalog') ? (
                  <div className="row" style={{ justifyContent: 'flex-end', gap: 4 }}>
                    <button className="btn btn-sm btn-ghost"
                      onClick={(e) => { e.stopPropagation(); setEditing(c) }}>Modifier</button>
                    {can('delete') && c.productCount === 0 && (
                      <button className="btn btn-sm btn-ghost"
                        onClick={(e) => { e.stopPropagation(); setDeleting(c) }}>Supprimer</button>
                    )}
                  </div>
                ) : null,
              },
            ]}
          />
        )}
      </Card>

      {editing && (
        <CategoryModal
          category={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); categories.reload(); toast.success('Catégorie enregistrée.') }}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title="Supprimer la catégorie ?"
          danger
          busy={busy}
          confirmLabel="Supprimer"
          message={`« ${deleting.name} » sera retirée du classement. Les articles qu'elle contiendrait doivent d'abord être déplacés.`}
          onConfirm={remove}
          onCancel={() => setDeleting(null)}
        />
      )}
    </div>
  )
}

function CategoryModal(props: {
  category: CategoryView | null
  onClose: () => void
  onSaved: () => void
}) {
  const [name, setName] = useState(props.category?.name ?? '')
  const [description, setDescription] = useState(props.category?.description ?? '')
  const [color, setColor] = useState(props.category?.color ?? 'blue')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function save() {
    setBusy(true)
    setError(null)
    try {
      await Catalog.saveCategory({ id: props.category?.id, name, description, color })
      props.onSaved()
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={props.category ? 'Modifier la catégorie' : 'Nouvelle catégorie'}
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
        <Field label="Nom" required>
          <TextInput value={name} onChange={setName} autoFocus placeholder="Imprimantes" />
        </Field>
        <Field label="Description">
          <TextArea value={description} onChange={setDescription} rows={2}
            placeholder="Jet d'encre, laser, multifonctions" />
        </Field>
        <Field label="Couleur du badge">
          <Select value={color} onChange={setColor} options={COLORS} />
        </Field>
        <div className="row"><span className="small muted">Aperçu :</span><Badge tone={color}>{name || 'Catégorie'}</Badge></div>
      </div>
    </Modal>
  )
}
