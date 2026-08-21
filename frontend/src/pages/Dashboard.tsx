// Tableau de bord : ce qu'il faut savoir en ouvrant la boutique le matin.

import { Reports } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { useSession } from '../lib/session'
import { formatDate, formatDateTime, movementLabel, movementTone, statusLabel, statusTone } from '../lib/format'
import { formatNumber } from '../lib/money'
import { Alert, Badge, BarList, Card, DataTable, Empty, KPI, Loading } from '../components/UI'
import { LineChart } from '../components/Chart'
import { CompanyLogo, useCompanyLogo } from '../components/CompanyLogo'
import type { PageContext } from '../App'

export function DashboardPage({ navigate }: PageContext) {
  const { money, amount, can, state } = useSession()
  const logo = useCompanyLogo()
  const { data, loading, error } = useAsync(() => Reports.dashboard(), [])

  if (loading) return <Loading />
  if (error) return <Alert tone="danger">{error}</Alert>
  if (!data) return null

  const showFinance = can('finance')

  return (
    <div className="stack">
      {/* Bandeau d'identité : le commerçant reconnaît sa boutique avant de lire
          ses chiffres, et voit du premier coup d'œil que son logo est bien
          celui qui partira sur ses factures. */}
      <div className={`shopfront ${logo ? '' : 'shopfront-plain'}`}>
        <CompanyLogo className="shopfront-logo" />
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className="shopfront-name truncate">{state.companyName}</div>
          <div className="shopfront-date">{aujourdhui()}</div>
        </div>
        <div className="shopfront-figure">
          <div className="kpi-label">Encaissé aujourd'hui</div>
          <div className="shopfront-amount">{money(data.todayRevenue.value)}</div>
        </div>
      </div>

      <div className="grid grid-4">
        <KPI
          label="Ventes du jour"
          value={money(data.todayRevenue.value)}
          change={data.todayRevenue.change}
          hint={`${data.todayRevenue.count} facture${data.todayRevenue.count > 1 ? 's' : ''}`}
        />
        <KPI
          label="Ventes du mois"
          value={money(data.monthRevenue.value)}
          change={data.monthRevenue.change}
          hint={`${data.monthRevenue.count} facture${data.monthRevenue.count > 1 ? 's' : ''}`}
        />
        {showFinance ? (
          <KPI label="Marge du mois" value={money(data.monthMargin.value)} change={data.monthMargin.change} />
        ) : (
          <KPI label="Unités en stock" value={formatNumber(data.stockUnits)}
            hint={`${data.defectiveUnits} défectueuse${data.defectiveUnits > 1 ? 's' : ''}`} />
        )}
        <KPI
          label={data.overdue > 0 ? 'Impayés, dont en retard' : 'Impayés'}
          value={money(data.outstanding)}
          hint={data.overdue > 0
            ? `${money(data.overdue)} en retard sur ${data.overdueCount} facture${data.overdueCount > 1 ? 's' : ''}`
            : `${data.outstandingCount} facture${data.outstandingCount > 1 ? 's' : ''} en attente`}
          accent={data.overdue > 0}
        />
      </div>

      {showFinance && (
        <div className="grid grid-3">
          <KPI label="Charges du mois" value={money(data.monthExpenses.value)} change={data.monthExpenses.change} />
          <KPI label="Résultat du mois" value={money(data.monthResult.value)} change={data.monthResult.change} />
          <KPI label="Valeur du stock" value={money(data.stockValue)}
            hint={`${formatNumber(data.stockUnits)} unités au coût moyen`} />
        </div>
      )}

      <Card
        title="Ventes des 30 derniers jours"
        note="Chiffre d'affaires hors taxes, factures émises uniquement"
      >
        <LineChart
          points={data.last30Days.map((p) => ({ label: p.label, value: p.revenueHT }))}
          format={(v) => amount(v)}
        />
      </Card>

      <div className="grid grid-2">
        <Card title="Douze derniers mois">
          <LineChart
            height={170}
            points={data.last12Months.map((p) => ({ label: p.label, value: p.revenueHT }))}
            format={(v) => amount(v)}
            maxLabels={6}
          />
        </Card>

        <Card title="Meilleures ventes du mois" note="Classées par chiffre d'affaires">
          {data.topProducts.length === 0 ? (
            <Empty title="Aucune vente ce mois-ci" />
          ) : (
            <BarList
              items={data.topProducts.map((p) => ({
                label: p.name,
                value: p.revenue,
                display: `${amount(p.revenue)} · ${p.unitsSold}×`,
              }))}
            />
          )}
        </Card>
      </div>

      <div className="grid grid-2">
        <Card
          title="Stock à surveiller"
          note="Articles au seuil d'alerte ou en rupture"
          actions={<button className="btn btn-sm" onClick={() => navigate('products')}>Voir les articles</button>}
          flush
        >
          <DataTable
            rows={data.lowStock}
            rowKey={(p) => p.id}
            onRowClick={(p) => navigate('products', p.id)}
            empty={<Empty title="Rien à signaler" text="Aucun article n'est sous son seuil d'alerte." />}
            columns={[
              {
                key: 'name', header: 'Article',
                render: (p) => (
                  <>
                    <div className="cell-primary truncate" style={{ maxWidth: 240 }}>{p.name}</div>
                    <div className="cell-secondary mono">{p.sku}</div>
                  </>
                ),
              },
              {
                key: 'qty', header: 'Stock', align: 'right',
                render: (p) => p.outOfStock
                  ? <Badge tone="red">Rupture</Badge>
                  : <Badge tone="orange">{p.quantity} restant{p.quantity > 1 ? 's' : ''}</Badge>,
              },
            ]}
          />
        </Card>

        <Card
          title="Dernières factures"
          actions={<button className="btn btn-sm" onClick={() => navigate('sales')}>Voir les ventes</button>}
          flush
        >
          <DataTable
            rows={data.recentInvoices}
            rowKey={(i) => i.id}
            onRowClick={(i) => navigate('sales', i.id)}
            empty={<Empty title="Aucune facture" text="Les ventes enregistrées apparaîtront ici." />}
            columns={[
              {
                key: 'number', header: 'Facture',
                render: (i) => (
                  <>
                    <div className="cell-primary mono">{i.number}</div>
                    <div className="cell-secondary truncate" style={{ maxWidth: 160 }}>{i.customerName}</div>
                  </>
                ),
              },
              { key: 'date', header: 'Date', render: (i) => <span className="small">{formatDate(i.date)}</span> },
              {
                key: 'total', header: 'Total', align: 'right',
                render: (i) => (
                  <>
                    <div className="strong">{amount(i.total)}</div>
                    <Badge tone={statusTone[i.status]}>{statusLabel[i.status]}</Badge>
                  </>
                ),
              },
            ]}
          />
        </Card>
      </div>

      <Card
        title="Derniers mouvements de stock"
        note={`Généré le ${formatDateTime(data.generatedAt)}`}
        actions={<button className="btn btn-sm" onClick={() => navigate('stock')}>Journal complet</button>}
        flush
      >
        <DataTable
          rows={data.recentMovements}
          rowKey={(m) => m.id}
          empty={<Empty title="Aucun mouvement" />}
          columns={[
            { key: 'type', header: 'Type', render: (m) => <Badge tone={movementTone[m.type]}>{movementLabel[m.type]}</Badge> },
            {
              key: 'product', header: 'Article',
              render: (m) => (
                <>
                  <div className="cell-primary truncate" style={{ maxWidth: 280 }}>{m.productName}</div>
                  {m.reason && <div className="cell-secondary truncate" style={{ maxWidth: 280 }}>{m.reason}</div>}
                </>
              ),
            },
            { key: 'qty', header: 'Quantité', align: 'right', render: (m) => formatNumber(m.quantity) },
            { key: 'after', header: 'Stock après', align: 'right', render: (m) => <span className="muted">{formatNumber(m.stockAfter)}</span> },
            { key: 'date', header: 'Date', align: 'right', render: (m) => <span className="small muted">{formatDate(m.date)}</span> },
          ]}
        />
      </Card>
    </div>
  )
}

/** aujourdhui rend « lundi 18 août 2026 », en tête du tableau de bord. */
function aujourdhui(): string {
  const jour = new Date().toLocaleDateString('fr-FR', {
    weekday: 'long', day: 'numeric', month: 'long', year: 'numeric',
  })
  return jour.charAt(0).toUpperCase() + jour.slice(1)
}
