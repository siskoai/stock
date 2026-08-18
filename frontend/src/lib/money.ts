// Formatage et saisie des montants.
//
// Le backend travaille en centièmes d'unité monétaire : 1 500 FCFA valent
// 150000. La conversion se fait ici, et nulle part ailleurs, pour qu'aucun
// composant n'ait à se demander dans quelle unité il manipule un nombre.

/** Espace fine insécable : la convention francophone pour les milliers. */
const ESPACE_FINE = ' '

/**
 * Espace insécable ordinaire, plus large, qui sépare le montant de son symbole.
 * La typographie française distingue les deux, et la nuance se voit : avec une
 * espace fine, « 228 920 FCFA » se lit collé, surtout dans les gros chiffres du
 * tableau de bord où le crénage est resserré.
 */
const ESPACE_INSECABLE = ' '

/**
 * formatMoney rend un montant en centièmes sous forme lisible.
 * 150000 avec 0 décimale → « 1 500 ».
 */
export function formatMoney(amount: number, decimals = 0): string {
  const negative = amount < 0
  const abs = Math.abs(Math.round(amount))
  const units = Math.floor(abs / 100)
  const cents = abs % 100

  let out = String(units).replace(/\B(?=(\d{3})+(?!\d))/g, ESPACE_FINE)
  if (decimals > 0) out += ',' + String(cents).padStart(2, '0').slice(0, decimals)
  return negative ? '-' + out : out
}

/** formatWithSymbol ajoute le symbole monétaire : « 1 500 FCFA ». */
export function formatWithSymbol(amount: number, decimals: number, symbol: string): string {
  return `${formatMoney(amount, decimals)}${ESPACE_INSECABLE}${symbol}`
}

/**
 * parseMoney lit une saisie utilisateur et renvoie des centièmes.
 * Accepte « 1 500 », « 1500,50 », « 1.500,50 » et « 1500.50 ».
 * Renvoie null si la saisie n'est pas un nombre.
 */
export function parseMoney(input: string, decimals = 0): number | null {
  const cleaned = input
    .replace(/[\s  ]/g, '')
    .replace(/[^\d,.\-]/g, '')
  if (cleaned === '' || cleaned === '-') return null

  // Le dernier séparateur rencontré est le séparateur décimal ; les autres
  // sont des séparateurs de milliers.
  const lastComma = cleaned.lastIndexOf(',')
  const lastDot = cleaned.lastIndexOf('.')
  const decimalPos = Math.max(lastComma, lastDot)

  let integerPart = cleaned
  let decimalPart = ''
  if (decimalPos > -1) {
    const tail = cleaned.slice(decimalPos + 1)
    // Trois chiffres après le dernier séparateur : c'est un millier, pas une
    // décimale (« 1.500 » vaut mille cinq cents).
    if (!(tail.length === 3 && decimals === 0)) {
      integerPart = cleaned.slice(0, decimalPos)
      decimalPart = tail
    }
  }
  integerPart = integerPart.replace(/[,.]/g, '')
  decimalPart = decimalPart.replace(/[,.]/g, '').padEnd(2, '0').slice(0, 2)

  const sign = integerPart.startsWith('-') ? -1 : 1
  const units = parseInt(integerPart.replace('-', '') || '0', 10)
  const cents = decimals > 0 ? parseInt(decimalPart || '0', 10) : 0
  if (Number.isNaN(units) || Number.isNaN(cents)) return null
  return sign * (units * 100 + cents)
}

/** toInput rend un montant pour un champ de saisie : sans séparateur de milliers. */
export function toInput(amount: number, decimals = 0): string {
  if (amount === 0) return ''
  const units = Math.floor(Math.abs(amount) / 100)
  const cents = Math.abs(amount) % 100
  const sign = amount < 0 ? '-' : ''
  if (decimals > 0) return `${sign}${units},${String(cents).padStart(2, '0')}`
  return `${sign}${units}`
}

/** formatPercent rend un taux avec une décimale : « 34,2 % ». */
export function formatPercent(value: number, decimals = 1): string {
  return `${value.toFixed(decimals).replace('.', ',')}${ESPACE_INSECABLE}%`
}

/** formatNumber groupe les milliers d'un entier simple (quantités). */
export function formatNumber(value: number): string {
  return String(Math.round(value)).replace(/\B(?=(\d{3})+(?!\d))/g, ESPACE_FINE)
}

/** formatBytes rend une taille de fichier lisible. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} Ko`
  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.', ',')} Mo`
}
