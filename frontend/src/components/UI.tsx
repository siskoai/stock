// Briques d'interface partagées : champs, tableaux, fenêtres, états vides.
// Tout est écrit à la main — aucune bibliothèque de composants — pour que le
// rendu reste identique sur macOS et sur Windows.

import {
  useEffect, useRef, useState,
  type ChangeEvent, type FormEvent, type ReactNode,
} from 'react'
import { IconAlert, IconClose, IconSearch } from './Icons'
import { formatMoney, parseMoney, toInput } from '../lib/money'

// --- Champs ----------------------------------------------------------------

interface FieldProps {
  label?: string
  hint?: string
  error?: string
  required?: boolean
  children: ReactNode
}

export function Field({ label, hint, error, required, children }: FieldProps) {
  return (
    <label className="field">
      {label && (
        <span className="field-label">
          {label}{required && <span style={{ color: 'var(--accent)' }}> *</span>}
        </span>
      )}
      {children}
      {error ? <span className="field-error">{error}</span>
        : hint ? <span className="field-hint">{hint}</span> : null}
    </label>
  )
}

export function TextInput(props: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  type?: 'text' | 'password' | 'email' | 'date'
  disabled?: boolean
  autoFocus?: boolean
  maxLength?: number
}) {
  const { value, onChange, ...rest } = props
  return (
    <input
      {...rest}
      type={props.type ?? 'text'}
      value={value}
      onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
    />
  )
}

export function NumberInput(props: {
  value: number
  onChange: (v: number) => void
  min?: number
  max?: number
  disabled?: boolean
  placeholder?: string
}) {
  const { value, onChange, ...rest } = props
  return (
    <input
      {...rest}
      className="num"
      type="number"
      value={Number.isFinite(value) ? value : 0}
      onChange={(e) => {
        const n = parseInt(e.target.value, 10)
        onChange(Number.isNaN(n) ? 0 : n)
      }}
    />
  )
}

/**
 * MoneyInput manipule des centièmes vers l'extérieur et une chaîne à
 * l'intérieur : l'utilisateur peut effacer complètement le champ ou taper un
 * séparateur décimal sans que la valeur se réécrive sous ses doigts.
 */
export function MoneyInput(props: {
  value: number
  onChange: (v: number) => void
  decimals: number
  symbol?: string
  disabled?: boolean
  autoFocus?: boolean
  placeholder?: string
}) {
  const { value, onChange, decimals, symbol, ...rest } = props
  const [text, setText] = useState(() => toInput(value, decimals))
  const editing = useRef(false)

  useEffect(() => {
    if (!editing.current) setText(toInput(value, decimals))
  }, [value, decimals])

  return (
    <div style={{ position: 'relative' }}>
      <input
        {...rest}
        className="input-money"
        inputMode="decimal"
        value={text}
        style={symbol ? { paddingRight: 8 + symbol.length * 7.5 } : undefined}
        onFocus={() => { editing.current = true }}
        onBlur={() => {
          editing.current = false
          setText(toInput(value, decimals))
        }}
        onChange={(e) => {
          setText(e.target.value)
          onChange(parseMoney(e.target.value, decimals) ?? 0)
        }}
      />
      {symbol && (
        <span style={{
          position: 'absolute', right: 9, top: '50%', transform: 'translateY(-50%)',
          fontSize: 12, color: 'var(--muted)', pointerEvents: 'none',
        }}>{symbol}</span>
      )}
    </div>
  )
}

export function Select<T extends string>(props: {
  value: T
  onChange: (v: T) => void
  options: { value: T; label: string }[]
  disabled?: boolean
  placeholder?: string
}) {
  return (
    <div className="select-wrap">
      <select
        value={props.value}
        disabled={props.disabled}
        onChange={(e) => props.onChange(e.target.value as T)}
      >
        {props.placeholder && <option value="">{props.placeholder}</option>}
        {props.options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </div>
  )
}

export function TextArea(props: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  rows?: number
  disabled?: boolean
}) {
  return (
    <textarea
      value={props.value}
      rows={props.rows ?? 3}
      placeholder={props.placeholder}
      disabled={props.disabled}
      onChange={(e) => props.onChange(e.target.value)}
    />
  )
}

export function Checkbox(props: {
  checked: boolean
  onChange: (v: boolean) => void
  label: string
  hint?: string
  disabled?: boolean
}) {
  return (
    <label className="check">
      <input
        type="checkbox"
        checked={props.checked}
        disabled={props.disabled}
        onChange={(e) => props.onChange(e.target.checked)}
      />
      <span>
        <span className="check-label">{props.label}</span>
        {props.hint && <><br /><span className="check-hint">{props.hint}</span></>}
      </span>
    </label>
  )
}

export function SearchInput(props: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  return (
    <div className="search">
      <IconSearch />
      <input
        type="text"
        value={props.value}
        placeholder={props.placeholder ?? 'Rechercher…'}
        onChange={(e) => props.onChange(e.target.value)}
      />
    </div>
  )
}

export function SegmentedControl<T extends string>(props: {
  value: T
  onChange: (v: T) => void
  options: { value: T; label: string }[]
}) {
  return (
    <div className="btn-group">
      {props.options.map((o) => (
        <button
          key={o.value}
          type="button"
          className={o.value === props.value ? 'active' : ''}
          onClick={() => props.onChange(o.value)}
        >{o.label}</button>
      ))}
    </div>
  )
}

// --- Présentation ----------------------------------------------------------

export function Badge({ tone = 'muted', children }: { tone?: string; children: ReactNode }) {
  return <span className={`badge ${tone}`}>{children}</span>
}

export function Card({ title, note, actions, children, flush }: {
  title?: string
  note?: string
  actions?: ReactNode
  children: ReactNode
  flush?: boolean
}) {
  return (
    <section className="card">
      {(title || actions) && (
        <header className="card-head">
          <div>
            {title && <div className="card-title">{title}</div>}
            {note && <div className="card-note">{note}</div>}
          </div>
          {actions && <div style={{ marginLeft: 'auto' }} className="row">{actions}</div>}
        </header>
      )}
      <div className={flush ? 'card-body-flush' : 'card-body'}>{children}</div>
    </section>
  )
}

export function Empty({ title, text, action }: { title: string; text?: string; action?: ReactNode }) {
  return (
    <div className="empty">
      <div className="empty-title">{title}</div>
      {text && <p className="empty-text">{text}</p>}
      {action}
    </div>
  )
}

export function Loading({ label = 'Chargement…' }: { label?: string }) {
  return <div className="loading"><span className="spinner" />{label}</div>
}

export function Alert({ tone = 'info', children }: { tone?: 'info' | 'warn' | 'danger' | 'success'; children: ReactNode }) {
  return (
    <div className={`alert ${tone}`}>
      {(tone === 'warn' || tone === 'danger') && <IconAlert />}
      <div>{children}</div>
    </div>
  )
}

// --- Fenêtres --------------------------------------------------------------

export function Modal(props: {
  title: string
  subtitle?: string
  size?: 'normal' | 'wide' | 'xwide'
  onClose: () => void
  footer?: ReactNode
  children: ReactNode
  onSubmit?: () => void
}) {
  // Échap ferme la fenêtre : c'est le réflexe attendu, et le seul moyen de
  // sortir quand la souris est loin.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') props.onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [props])

  const body = (
    <div className={`modal ${props.size === 'wide' ? 'wide' : props.size === 'xwide' ? 'xwide' : ''}`}
      onClick={(e) => e.stopPropagation()}>
      <header className="modal-head">
        <div style={{ flex: 1 }}>
          <div className="modal-title">{props.title}</div>
          {props.subtitle && <div className="modal-sub">{props.subtitle}</div>}
        </div>
        <button className="btn btn-ghost btn-icon" onClick={props.onClose} aria-label="Fermer">
          <IconClose />
        </button>
      </header>
      <div className="modal-body">{props.children}</div>
      {props.footer && <footer className="modal-foot">{props.footer}</footer>}
    </div>
  )

  return (
    <div className="overlay" onClick={props.onClose}>
      {props.onSubmit ? (
        <form
          style={{ width: '100%', display: 'flex', justifyContent: 'center' }}
          onSubmit={(e: FormEvent) => { e.preventDefault(); props.onSubmit?.() }}
        >{body}</form>
      ) : body}
    </div>
  )
}

export function ConfirmDialog(props: {
  title: string
  message: ReactNode
  confirmLabel?: string
  danger?: boolean
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}) {
  return (
    <Modal title={props.title} onClose={props.onCancel} footer={
      <>
        <button className="btn" onClick={props.onCancel} disabled={props.busy}>Annuler</button>
        <button
          className={`btn ${props.danger ? 'btn-danger' : 'btn-primary'}`}
          onClick={props.onConfirm}
          disabled={props.busy}
        >{props.busy ? 'En cours…' : (props.confirmLabel ?? 'Confirmer')}</button>
      </>
    }>
      <div style={{ fontSize: 13.5, lineHeight: 1.55 }}>{props.message}</div>
    </Modal>
  )
}

// --- Tableaux --------------------------------------------------------------

export interface Column<T> {
  key: string
  header: string
  align?: 'left' | 'right'
  width?: number
  sortable?: boolean
  render: (row: T) => ReactNode
}

export function DataTable<T>(props: {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  onRowClick?: (row: T) => void
  dimmed?: (row: T) => boolean
  footer?: ReactNode
  sortBy?: string
  sortDesc?: boolean
  onSort?: (key: string) => void
  empty?: ReactNode
}) {
  if (props.rows.length === 0 && props.empty) return <>{props.empty}</>
  return (
    <div className="table-wrap">
      <table className="data">
        <thead>
          <tr>
            {props.columns.map((c) => (
              <th
                key={c.key}
                className={`${c.align === 'right' ? 'num' : ''} ${c.sortable && props.onSort ? 'sortable' : ''}`}
                style={c.width ? { width: c.width } : undefined}
                onClick={c.sortable && props.onSort ? () => props.onSort?.(c.key) : undefined}
              >
                {c.header}
                {props.sortBy === c.key && <span> {props.sortDesc ? '↓' : '↑'}</span>}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => (
            <tr
              key={props.rowKey(row)}
              className={`${props.onRowClick ? 'clickable' : ''} ${props.dimmed?.(row) ? 'dimmed' : ''}`}
              onClick={props.onRowClick ? () => props.onRowClick?.(row) : undefined}
            >
              {props.columns.map((c) => (
                <td key={c.key} className={c.align === 'right' ? 'num' : ''}>{c.render(row)}</td>
              ))}
            </tr>
          ))}
        </tbody>
        {props.footer && <tfoot>{props.footer}</tfoot>}
      </table>
    </div>
  )
}

// --- Indicateurs -----------------------------------------------------------

export function KPI(props: {
  label: string
  value: string
  hint?: string
  change?: number
  accent?: boolean
}) {
  const change = props.change
  const tone = change === undefined || Math.abs(change) < 0.5 ? 'flat' : change > 0 ? 'up' : 'down'
  return (
    <div className="card kpi">
      <div className="kpi-label">{props.label}</div>
      <div className={`kpi-value ${props.accent ? 'accent' : ''}`}>{props.value}</div>
      {change !== undefined ? (
        <div className="kpi-hint">
          <span className={`kpi-delta ${tone}`}>
            {tone === 'flat' ? '—' : `${change > 0 ? '+' : ''}${change.toFixed(0)} %`}
          </span>
          {props.hint && <span> · {props.hint}</span>}
        </div>
      ) : props.hint ? <div className="kpi-hint">{props.hint}</div> : null}
    </div>
  )
}

/** BarList affiche une répartition : un libellé, une barre, une valeur. */
export function BarList(props: {
  items: { label: string; value: number; display?: string }[]
  max?: number
  color?: string
}) {
  const max = props.max ?? Math.max(1, ...props.items.map((i) => i.value))
  return (
    <div>
      {props.items.map((item, i) => (
        <div className="bar-row" key={`${item.label}-${i}`}>
          <div className="bar-label" title={item.label}>{item.label}</div>
          <div className="bar-track">
            <div
              className="bar-fill"
              style={{
                width: `${Math.max(1, (item.value / max) * 100)}%`,
                background: props.color ?? 'var(--brand)',
              }}
            />
          </div>
          <div className="bar-value">{item.display ?? formatMoney(item.value)}</div>
        </div>
      ))}
    </div>
  )
}
