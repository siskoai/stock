// Libellés et formats de dates, en français.

import type { DocStatus, MovementType, PaymentMethod, Role } from './types'

const MOIS = ['janvier', 'février', 'mars', 'avril', 'mai', 'juin',
  'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre']

/** formatDate rend « 18/08/2026 ». */
export function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit', year: 'numeric' })
}

/** formatDateTime rend « 18/08/2026 à 14h30 ». */
export function formatDateTime(iso: string | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const heure = d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' }).replace(':', 'h')
  return `${formatDate(iso)} à ${heure}`
}

/** formatLongDate rend « 18 août 2026 ». */
export function formatLongDate(iso: string | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return `${d.getDate()} ${MOIS[d.getMonth()]} ${d.getFullYear()}`
}

/** isoDate rend une date au format attendu par le backend (AAAA-MM-JJ). */
export function isoDate(d: Date = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** startOfMonth rend le premier jour du mois courant, au format ISO. */
export function startOfMonth(d: Date = new Date()): string {
  return isoDate(new Date(d.getFullYear(), d.getMonth(), 1))
}

/** daysAgo rend la date d'il y a n jours, au format ISO. */
export function daysAgo(n: number, d: Date = new Date()): string {
  const copy = new Date(d)
  copy.setDate(copy.getDate() - n)
  return isoDate(copy)
}

export const statusLabel: Record<DocStatus, string> = {
  DRAFT: 'Devis',
  ISSUED: 'Émise',
  PARTIAL: 'Partiellement réglée',
  PAID: 'Réglée',
  CANCELLED: 'Annulée',
}

export const statusTone: Record<DocStatus, string> = {
  DRAFT: 'muted', ISSUED: 'blue', PARTIAL: 'orange',
  PAID: 'green', CANCELLED: 'red',
}

export const paymentLabel: Record<PaymentMethod, string> = {
  CASH: 'Espèces', MOBILE: 'Mobile money', TRANSFER: 'Virement',
  CHECK: 'Chèque', CREDIT: 'Crédit',
}

export const movementLabel: Record<MovementType, string> = {
  IN: 'Entrée', OUT: 'Sortie',
  RETURN_CUSTOMER: 'Retour client', RETURN_SUPPLIER: 'Retour fournisseur',
  DEFECT: 'Défectueux', REPAIR: 'Réparation',
  SCRAP: 'Rebut', ADJUST: 'Inventaire',
}

export const movementTone: Record<MovementType, string> = {
  IN: 'green', OUT: 'blue',
  RETURN_CUSTOMER: 'aqua', RETURN_SUPPLIER: 'orange',
  DEFECT: 'red', REPAIR: 'violet',
  SCRAP: 'red', ADJUST: 'muted',
}

/** movementSign indique si le type augmente ou diminue le stock vendable. */
export function movementSign(type: MovementType): 1 | -1 | 0 {
  switch (type) {
    case 'IN': case 'REPAIR': return 1
    case 'OUT': case 'RETURN_SUPPLIER': case 'DEFECT': return -1
    default: return 0
  }
}

export const roleLabel: Record<Role, string> = {
  ADMIN: 'Administrateur', MANAGER: 'Gérant', SELLER: 'Vendeur',
}

/** pluralize accorde un mot simple selon la quantité. */
export function pluralize(n: number, singular: string, plural?: string): string {
  return n > 1 ? (plural ?? singular + 's') : singular
}
