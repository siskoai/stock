// Graphiques en SVG, dessinés à la main.
//
// Une bibliothèque de graphes pèserait plus lourd que tout le reste de
// l'interface pour deux formes : une courbe et un histogramme. Les échelles
// sont linéaires et démarrent à zéro — une courbe de chiffre d'affaires dont
// l'axe ne part pas de zéro ment sur l'amplitude des variations.

import { useId, useState } from 'react'

export interface Point { label: string; value: number }

const PAD = { top: 12, right: 10, bottom: 22, left: 52 }

/** niceMax arrondit le maximum vers le haut pour donner des graduations lisibles. */
function niceMax(value: number): number {
  if (value <= 0) return 1
  const magnitude = Math.pow(10, Math.floor(Math.log10(value)))
  const normalized = value / magnitude
  const step = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10
  return step * magnitude
}

interface ChartProps {
  points: Point[]
  height?: number
  format: (v: number) => string
  /** Nombre maximum d'étiquettes sur l'axe horizontal. */
  maxLabels?: number
}

export function LineChart({ points, height = 190, format, maxLabels = 8 }: ChartProps) {
  const gradientId = useId()
  const [hover, setHover] = useState<number | null>(null)
  const width = 720
  const innerW = width - PAD.left - PAD.right
  const innerH = height - PAD.top - PAD.bottom

  if (points.length === 0) {
    return <div className="empty" style={{ padding: 30 }}>Aucune donnée sur la période.</div>
  }

  const max = niceMax(Math.max(...points.map((p) => p.value), 0))
  const x = (i: number) => PAD.left + (points.length === 1 ? innerW / 2 : (i / (points.length - 1)) * innerW)
  const y = (v: number) => PAD.top + innerH - (v / max) * innerH

  const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(p.value).toFixed(1)}`).join(' ')
  const area = `${line} L${x(points.length - 1).toFixed(1)},${(PAD.top + innerH).toFixed(1)} L${x(0).toFixed(1)},${(PAD.top + innerH).toFixed(1)} Z`

  const labelStep = Math.max(1, Math.ceil(points.length / maxLabels))
  const active = hover !== null ? points[hover] : null

  return (
    <div style={{ position: 'relative' }}>
      <svg className="chart" viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" role="img"
        aria-label="Évolution sur la période">
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--brand)" stopOpacity=".18" />
            <stop offset="100%" stopColor="var(--brand)" stopOpacity="0" />
          </linearGradient>
        </defs>

        {[0, 0.25, 0.5, 0.75, 1].map((r) => (
          <g key={r}>
            <line className="chart-grid" x1={PAD.left} x2={width - PAD.right}
              y1={PAD.top + innerH * r} y2={PAD.top + innerH * r} />
            <text className="chart-axis" x={PAD.left - 7} y={PAD.top + innerH * r + 3} textAnchor="end">
              {format(max * (1 - r))}
            </text>
          </g>
        ))}

        <path d={area} fill={`url(#${gradientId})`} />
        <path className="chart-line" d={line} />

        {points.map((p, i) => (
          <g key={p.label + i}>
            {(hover === i || points.length <= 14) && (
              <circle className="chart-dot" cx={x(i)} cy={y(p.value)} r={hover === i ? 4 : 2.5} />
            )}
            <rect
              className="chart-hover"
              x={x(i) - innerW / points.length / 2}
              y={PAD.top}
              width={innerW / points.length}
              height={innerH}
              onMouseEnter={() => setHover(i)}
              onMouseLeave={() => setHover(null)}
            />
            {i % labelStep === 0 && (
              <text className="chart-axis" x={x(i)} y={height - 6} textAnchor="middle">{p.label}</text>
            )}
          </g>
        ))}
      </svg>

      {active && (
        <div style={{
          position: 'absolute', top: 4, right: 4,
          background: 'var(--surface)', border: '1px solid var(--rule)',
          borderRadius: 7, padding: '5px 9px', fontSize: 12, boxShadow: 'var(--shadow-sm)',
          pointerEvents: 'none',
        }}>
          <strong>{active.label}</strong> · <span className="tabular">{format(active.value)}</span>
        </div>
      )}
    </div>
  )
}

export function BarChart({ points, height = 190, format, maxLabels = 12 }: ChartProps) {
  const [hover, setHover] = useState<number | null>(null)
  const width = 720
  const innerW = width - PAD.left - PAD.right
  const innerH = height - PAD.top - PAD.bottom

  if (points.length === 0) {
    return <div className="empty" style={{ padding: 30 }}>Aucune donnée sur la période.</div>
  }

  const max = niceMax(Math.max(...points.map((p) => p.value), 0))
  const slot = innerW / points.length
  const barW = Math.max(3, Math.min(38, slot * 0.62))
  const labelStep = Math.max(1, Math.ceil(points.length / maxLabels))
  const active = hover !== null ? points[hover] : null

  return (
    <div style={{ position: 'relative' }}>
      <svg className="chart" viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" role="img"
        aria-label="Répartition sur la période">
        {[0, 0.5, 1].map((r) => (
          <g key={r}>
            <line className="chart-grid" x1={PAD.left} x2={width - PAD.right}
              y1={PAD.top + innerH * r} y2={PAD.top + innerH * r} />
            <text className="chart-axis" x={PAD.left - 7} y={PAD.top + innerH * r + 3} textAnchor="end">
              {format(max * (1 - r))}
            </text>
          </g>
        ))}

        {points.map((p, i) => {
          const h = Math.max(p.value > 0 ? 2 : 0, (p.value / max) * innerH)
          const cx = PAD.left + slot * i + slot / 2
          return (
            <g key={p.label + i}
              onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
              <rect
                className="chart-bar"
                x={cx - barW / 2}
                y={PAD.top + innerH - h}
                width={barW}
                height={h}
                rx={2}
                opacity={hover === null || hover === i ? 1 : 0.45}
              />
              <rect className="chart-hover" x={PAD.left + slot * i} y={PAD.top} width={slot} height={innerH} />
              {i % labelStep === 0 && (
                <text className="chart-axis" x={cx} y={height - 6} textAnchor="middle">{p.label}</text>
              )}
            </g>
          )
        })}
      </svg>

      {active && (
        <div style={{
          position: 'absolute', top: 4, right: 4,
          background: 'var(--surface)', border: '1px solid var(--rule)',
          borderRadius: 7, padding: '5px 9px', fontSize: 12, boxShadow: 'var(--shadow-sm)',
          pointerEvents: 'none',
        }}>
          <strong>{active.label}</strong> · <span className="tabular">{format(active.value)}</span>
        </div>
      )}
    </div>
  )
}
