// Rapports : évolution des ventes, compte de résultat, situation, statistiques.

import { useMemo, useState } from 'react'
import { Documents, Export, Reports } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { daysAgo, isoDate, startOfMonth } from '../lib/format'
import { formatNumber, formatPercent } from '../lib/money'
import {
  Alert, BarList, Card, DataTable, Empty, KPI, Loading, SegmentedControl,
} from '../components/UI'
import { BarChart, LineChart } from '../components/Chart'
import { useDocumentPreview } from '../components/DocumentPreview'
import { IconDownload, IconPrint } from '../components/Icons'
import type { Granularity } from '../lib/types'
import type { PageContext } from '../App'

type Tab = 'evolution' | 'result' | 'situation' | 'statistics'

const RANGES = [
  { key: '30d', label: '30 jours', from: () => daysAgo(29), gran: 'day' as Granularity },
  { key: 'month', label: 'Ce mois', from: () => startOfMonth(), gran: 'day' as Granularity },
  { key: '12m', label: '12 mois', from: () => daysAgo(364), gran: 'month' as Granularity },
  { key: 'year', label: 'Cette année', from: () => `${new Date().getFullYear()}-01-01`, gran: 'month' as Granularity },
  { key: 'all', label: 'Tout', from: () => '', gran: 'month' as Granularity },
]

export function ReportsPage(_: PageContext) {
  const [tab, setTab] = useState<Tab>('evolution')
  const [rangeKey, setRangeKey] = useState('30d')
  const range = RANGES.find((r) => r.key === rangeKey) ?? RANGES[0]
  const from = range.from()
  const to = isoDate()

  return (
    <div className="stack">
      <div className="row row-wrap">
        {/* La base de 420 px force le sélecteur de période à passer sous les
            onglets quand la fenêtre se resserre, au lieu de les rogner. */}
        <div className="tabs" style={{ marginBottom: 0, flex: '1 1 420px', minWidth: 0 }}>
          <button className={`tab ${tab === 'evolution' ? 'active' : ''}`} onClick={() => setTab('evolution')}>Évolution</button>
          <button className={`tab ${tab === 'result' ? 'active' : ''}`} onClick={() => setTab('result')}>Compte de résultat</button>
          <button className={`tab ${tab === 'situation' ? 'active' : ''}`} onClick={() => setTab('situation')}>Situation</button>
          <button className={`tab ${tab === 'statistics' ? 'active' : ''}`} onClick={() => setTab('statistics')}>Statistiques</button>
        </div>
        <SegmentedControl
          value={rangeKey}
          onChange={setRangeKey}
          options={RANGES.map((r) => ({ value: r.key, label: r.label }))}
        />
      </div>

      {/* La clé force le remontage au changement de période : le pas
          d'agrégation doit repartir de celui que la période appelle, sinon
          « 12 mois » resterait affiché au jour. */}
      {tab === 'evolution' && (
        <Evolution key={rangeKey} from={from} to={to} granularity={range.gran} />
      )}
      {tab === 'result' && <Result from={from} to={to} />}
      {tab === 'situation' && <Situation asOf={to} />}
      {tab === 'statistics' && <Stats from={from} to={to} />}
    </div>
  )
}

// --- Évolution -------------------------------------------------------------

function Evolution({ from, to, granularity }: { from: string; to: string; granularity: Granularity }) {
  const { money, amount } = useSession()
  const doc = useDocumentPreview()
  const [gran, setGran] = useState<Granularity>(granularity)
  const query = useMemo(() => ({ from, to, granularity: gran }), [from, to, gran])
  const report = useAsync(() => Reports.salesReport(query), [query])

  if (report.loading) return <Loading />
  if (report.error) return <Alert tone="danger">{report.error}</Alert>
  if (!report.data) return null
  const r = report.data

  return (
    <div className="stack">
      <div className="grid grid-4">
        <KPI label="Chiffre d'affaires HT" value={money(r.total.revenueHT)}
          hint={`${r.total.invoiceCount} facture(s), ${formatNumber(r.total.unitsSold)} article(s)`} />
        <KPI label="Marge brute" value={money(r.total.grossMargin)}
          hint={`${formatPercent(r.total.marginRate)} du chiffre d'affaires`} />
        <KPI label="Charges" value={money(r.total.expenses)} hint="hors coût des marchandises" />
        <KPI label="Résultat" value={money(r.total.netResult)}
          hint={`panier moyen ${amount(r.averageTicket)}`} accent={r.total.netResult < 0} />
      </div>

      <Card
        title="Chiffre d'affaires par période"
        note={r.best ? `Meilleure période : ${r.best.label} (${money(r.best.revenueHT)})` : undefined}
        actions={
          <>
            <SegmentedControl
              value={gran}
              onChange={setGran}
              options={[
                { value: 'day' as Granularity, label: 'Jour' },
                { value: 'week' as Granularity, label: 'Semaine' },
                { value: 'month' as Granularity, label: 'Mois' },
                { value: 'quarter' as Granularity, label: 'Trimestre' },
              ]}
            />
            <button className="btn btn-sm" disabled={doc.busy}
              onClick={() => doc.open('Rapport de ventes', () => Documents.salesReport(query))}>
              <IconPrint />Imprimer
            </button>
            <button className="btn btn-sm" disabled={doc.busy}
              onClick={() => doc.download(() => Export.salesReport(query))}>
              <IconDownload />Exporter
            </button>
          </>
        }
      >
        {gran === 'day' && r.points.length > 40
          ? <LineChart points={r.points.map((p) => ({ label: p.label, value: p.revenueHT }))} format={amount} />
          : <BarChart points={r.points.map((p) => ({ label: p.label, value: p.revenueHT }))} format={amount} />}
      </Card>

      <Card title="Détail par période" flush>
        <DataTable
          rows={r.points}
          rowKey={(p) => p.key}
          empty={<Empty title="Aucune activité sur la période" />}
          footer={
            <tr>
              <td>Total</td>
              <td className="num">{r.total.invoiceCount}</td>
              <td className="num">{formatNumber(r.total.unitsSold)}</td>
              <td className="num">{amount(r.total.revenueHT)}</td>
              <td className="num">{amount(r.total.costOfSales)}</td>
              <td className="num">{amount(r.total.grossMargin)}</td>
              <td className="num">{formatPercent(r.total.marginRate)}</td>
              <td className="num">{amount(r.total.expenses)}</td>
              <td className="num">{amount(r.total.netResult)}</td>
            </tr>
          }
          columns={[
            { key: 'label', header: 'Période', render: (p) => p.label },
            { key: 'invoices', header: 'Factures', align: 'right', render: (p) => p.invoiceCount || '-' },
            { key: 'units', header: 'Articles', align: 'right', render: (p) => formatNumber(p.unitsSold) },
            { key: 'revenue', header: 'CA HT', align: 'right', render: (p) => <strong>{amount(p.revenueHT)}</strong> },
            { key: 'cost', header: 'Coût', align: 'right', render: (p) => amount(p.costOfSales) },
            { key: 'margin', header: 'Marge', align: 'right', render: (p) => amount(p.grossMargin) },
            { key: 'rate', header: 'Taux', align: 'right', render: (p) => p.revenueHT > 0 ? formatPercent(p.marginRate) : '-' },
            { key: 'expenses', header: 'Charges', align: 'right', render: (p) => amount(p.expenses) },
            {
              key: 'result', header: 'Résultat', align: 'right',
              render: (p) => <span className={p.netResult < 0 ? 'neg' : p.netResult > 0 ? 'pos' : ''}>{amount(p.netResult)}</span>,
            },
          ]}
        />
      </Card>

      {doc.element}
    </div>
  )
}

// --- Compte de résultat ----------------------------------------------------

function Result({ from, to }: { from: string; to: string }) {
  const { money, amount } = useSession()
  const doc = useDocumentPreview()
  const statement = useAsync(() => Reports.incomeStatement(from, to), [from, to])

  if (statement.loading) return <Loading />
  if (statement.error) return <Alert tone="danger">{statement.error}</Alert>
  if (!statement.data) return null
  const s = statement.data

  return (
    <div className="stack">
      <div className="grid grid-4">
        <KPI label="Chiffre d'affaires HT" value={money(s.revenueHT)}
          hint={`${s.invoiceCount} facture(s)`} />
        <KPI label="Marge brute" value={money(s.grossMargin)} hint={formatPercent(s.marginRate)} />
        <KPI label="Charges" value={money(s.totalExpenses)}
          hint={`${s.expenseLines.length} rubrique(s)`} />
        <KPI label="Résultat d'exploitation" value={money(s.operatingResult)}
          hint={formatPercent(s.resultRate)} accent={s.operatingResult < 0} />
      </div>

      <div className="row">
        <div className="spacer" />
        <button className="btn" disabled={doc.busy}
          onClick={() => doc.open('Compte de résultat', () => Documents.incomeStatement(from, to))}>
          <IconPrint />Imprimer le compte de résultat
        </button>
      </div>

      <div className="grid grid-2">
        <Card title="Produits">
          <Line label="Ventes hors taxes" value={amount(s.revenueHT)} />
          <Line label="Remises accordées" value={`− ${amount(s.discountsGiven)}`} muted />
          <Line label="Taxes facturées" value={amount(s.taxCollected)} muted
            hint="collectées pour l'État, à reverser" />
          <div className="divider" />
          <Line label="Chiffre d'affaires net HT" value={amount(s.revenueHT)} strong />
        </Card>

        <Card title="Charges">
          <Line label="Coût des marchandises vendues" value={amount(s.costOfSales)} />
          {s.expenseLines.map((l) => (
            <Line key={l.category} label={l.category} value={amount(l.amount)} muted
              hint={formatPercent(l.share, 0)} />
          ))}
          {s.scrapLoss > 0 && <Line label="Pertes sur rebuts" value={amount(s.scrapLoss)} muted />}
          <div className="divider" />
          <Line label="Total des charges" value={amount(s.costOfSales + s.totalExpenses + s.scrapLoss)} strong />
        </Card>
      </div>

      <div className="grid grid-2">
        <Card title="Répartition des charges d'exploitation">
          {s.expenseLines.length === 0
            ? <Empty title="Aucune charge sur la période" />
            : <BarList items={s.expenseLines.map((l) => ({
                label: l.category, value: l.amount,
                display: `${amount(l.amount)} · ${formatPercent(l.share, 0)}`,
              }))} />}
        </Card>

        <Card
          title="Trésorerie estimée"
          note="Les achats fournisseurs sont supposés réglés à la réception"
        >
          <Line label="Encaissements clients" value={amount(s.cashCollected)} />
          <Line label="Achats fournisseurs" value={`− ${amount(s.purchasesPaid)}`} />
          <Line label="Charges d'exploitation" value={`− ${amount(s.totalExpenses)}`} />
          <div className="divider" />
          <Line label="Flux estimé" value={amount(s.estimatedCashFlow)} strong
            accent={s.estimatedCashFlow < 0} />
          <p className="small muted" style={{ marginTop: 12, lineHeight: 1.45 }}>
            Comptoir ne suit pas les échéances fournisseurs : ce chiffre est une estimation,
            utile pour situer la tendance, pas pour arrêter une trésorerie exacte.
          </p>
        </Card>
      </div>

      {doc.element}
    </div>
  )
}

// --- Situation -------------------------------------------------------------

function Situation({ asOf }: { asOf: string }) {
  const { money, amount } = useSession()
  const sheet = useAsync(() => Reports.balanceSheet(asOf), [asOf])

  if (sheet.loading) return <Loading />
  if (sheet.error) return <Alert tone="danger">{sheet.error}</Alert>
  if (!sheet.data) return null
  const b = sheet.data

  return (
    <div className="stack">
      <Alert tone="info">
        Comptoir suit le stock, les créances clients et le résultat d'exploitation.
        Il ne tient ni comptes bancaires, ni immobilisations, ni dettes fournisseurs :
        ce n'est pas un logiciel comptable certifié. Les rubriques absentes le sont volontairement.
      </Alert>

      <div className="grid grid-4">
        <KPI label="Valeur du stock" value={money(b.stockValueSound)}
          hint={`${formatNumber(b.stockUnits)} unités au coût moyen`} />
        <KPI label="Stock défectueux" value={money(b.stockValueDefective)}
          hint={`${formatNumber(b.defectiveUnits)} unités isolées`} />
        <KPI label="Créances clients" value={money(b.receivables)}
          hint={`${b.receivableCount} facture(s) impayée(s)`} accent={b.receivables > 0} />
        <KPI label="Total de l'actif suivi" value={money(b.totalAssets)} />
      </div>

      <div className="grid grid-2">
        <Card title="Actif suivi">
          <Line label="Stock vendable" value={amount(b.stockValueSound)} />
          <Line label="Stock défectueux" value={amount(b.stockValueDefective)} muted />
          <Line label="Créances clients" value={amount(b.receivables)} />
          <div className="divider" />
          <Line label="Total" value={amount(b.totalAssets)} strong />
        </Card>

        <Card title="Cumul depuis l'ouverture">
          <Line label="Chiffre d'affaires" value={amount(b.cumulativeRevenue)} />
          <Line label="Coût des marchandises vendues" value={`− ${amount(b.cumulativeCostOfSales)}`} muted />
          <Line label="Charges d'exploitation" value={`− ${amount(b.cumulativeExpenses)}`} muted />
          <Line label="Pertes sur rebuts" value={`− ${amount(b.cumulativeScrapLoss)}`} muted />
          <div className="divider" />
          <Line label="Résultat cumulé" value={amount(b.cumulativeResult)} strong
            accent={b.cumulativeResult < 0} />
          <div className="divider" />
          <Line label="Taxes facturées à reverser" value={amount(b.taxCollected)} muted />
        </Card>
      </div>
    </div>
  )
}

// --- Statistiques ----------------------------------------------------------

function Stats({ from, to }: { from: string; to: string }) {
  const { money, amount, can } = useSession()
  const query = useMemo(() => ({ from, to }), [from, to])
  const stats = useAsync(() => Reports.statistics(query), [query])

  if (stats.loading) return <Loading />
  if (stats.error) return <Alert tone="danger">{stats.error}</Alert>
  if (!stats.data) return null
  const s = stats.data

  return (
    <div className="stack">
      <div className="grid grid-2">
        <Card title="Ventes par catégorie">
          {s.byCategory.length === 0 ? <Empty title="Aucune vente sur la période" /> : (
            <BarList items={s.byCategory.map((c) => ({
              label: c.label, value: c.amount,
              display: `${amount(c.amount)} · ${formatPercent(c.share, 0)}`,
            }))} />
          )}
        </Card>

        <Card title="Modes de règlement">
          {s.byPayment.length === 0 ? <Empty title="Aucun règlement" /> : (
            <BarList items={s.byPayment.map((p) => ({
              label: p.label, value: p.amount,
              display: `${amount(p.amount)} · ${p.count}×`,
            }))} />
          )}
        </Card>
      </div>

      <div className="grid grid-2">
        <Card title="Meilleurs clients" note="Par chiffre d'affaires hors taxes">
          {s.byCustomer.length === 0 ? <Empty title="Aucun client sur la période" /> : (
            <BarList items={s.byCustomer.map((c) => ({
              label: c.label || 'Client comptoir', value: c.amount,
              display: `${amount(c.amount)} · ${c.count} fact.`,
            }))} />
          )}
        </Card>

        <Card title="Ventes par jour de la semaine">
          <BarChart
            height={170}
            viewWidth={460}
            points={s.byWeekday.map((d) => ({ label: d.label.slice(0, 3), value: d.amount }))}
            format={amount}
          />
        </Card>
      </div>

      <div className="grid grid-2">
        <Card title="Articles les plus vendus" flush>
          <DataTable
            rows={s.topProducts}
            rowKey={(p) => p.productId}
            empty={<Empty title="Aucune vente" />}
            columns={[
              {
                key: 'name', header: 'Article',
                render: (p) => (
                  <>
                    <div className="cell-primary truncate" style={{ maxWidth: 220 }}>{p.name}</div>
                    <div className="cell-secondary">{p.category}</div>
                  </>
                ),
              },
              { key: 'units', header: 'Vendus', align: 'right', render: (p) => formatNumber(p.unitsSold) },
              { key: 'revenue', header: 'CA HT', align: 'right', render: (p) => <strong>{amount(p.revenue)}</strong> },
              ...(can('finance') ? [{
                key: 'margin', header: 'Marge', align: 'right' as const,
                render: (p: typeof s.topProducts[number]) => (
                  <>
                    <div>{amount(p.margin)}</div>
                    <div className="cell-secondary">{formatPercent(p.marginRate, 0)}</div>
                  </>
                ),
              }] : []),
              { key: 'left', header: 'Reste', align: 'right', render: (p) => formatNumber(p.stockLeft) },
            ]}
          />
        </Card>

        <Card
          title="Articles dormants"
          note="En stock, aucune vente sur la période, de la trésorerie immobilisée"
          flush
        >
          <DataTable
            rows={s.slowProducts}
            rowKey={(p) => p.productId}
            empty={<Empty title="Tout le stock tourne" text="Chaque article en stock a été vendu au moins une fois." />}
            columns={[
              {
                key: 'name', header: 'Article',
                render: (p) => (
                  <>
                    <div className="cell-primary truncate" style={{ maxWidth: 240 }}>{p.name}</div>
                    <div className="cell-secondary">{p.category}</div>
                  </>
                ),
              },
              { key: 'stock', header: 'En stock', align: 'right', render: (p) => formatNumber(p.stockLeft) },
              ...(can('finance') ? [{
                key: 'capital', header: 'Capital immobilisé', align: 'right' as const,
                render: (p: typeof s.slowProducts[number]) => <strong>{money(p.margin)}</strong>,
              }] : []),
            ]}
          />
        </Card>
      </div>

      {can('finance') && s.bySeller.length > 1 && (
        <Card title="Ventes par vendeur">
          <BarList items={s.bySeller.map((v) => ({
            label: v.label, value: v.amount,
            display: `${amount(v.amount)} · ${v.count} fact.`,
          }))} />
        </Card>
      )}
    </div>
  )
}

function Line(props: {
  label: string; value: string
  strong?: boolean; muted?: boolean; accent?: boolean; hint?: string
}) {
  return (
    <div className="ligne-total">
      <span className={`ligne-total-libelle ${props.muted ? 'small muted' : 'small'}`}>
        {props.label}
        {props.hint && <span className="muted"> · {props.hint}</span>}
      </span>
      <span className="ligne-total-valeur" style={{
        fontWeight: props.strong ? 650 : 400,
        fontSize: props.strong ? 15 : 13,
        color: props.accent ? 'var(--red)' : undefined,
      }}>{props.value}</span>
    </div>
  )
}
