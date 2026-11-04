import { useEffect, useState } from "react"
import { DashboardLayout } from "../components/dashboard-layout"
import { RangePicker } from "../components/range-picker"
import { SeriesChart } from "../components/series-chart"
import { formatBytes } from "../lib/format"
import type { RangeId } from "../lib/ranges"
import { hasNumbers, usePanelQuery, type AnalyticsPlace, type AnalyticsSummary, type AnalyticsTop } from "../lib/series"

export function AnalyticsPage() {
  const [range, setRange] = useState<RangeId>("24h")
  const { data, error, loading } = usePanelQuery<AnalyticsSummary>("/api/analytics/summary", range)

  useEffect(() => {
    document.title = "Analytics · Altas Panel"
  }, [])

  const totals = data?.totals
  const requests = data?.series.map((point) => point.requests) ?? []

  return (
    <DashboardLayout section="Analytics">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">04 / Analytics</p>
          <h1 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">Traffic</h1>
        </div>
        <RangePicker value={range} onChange={setRange} />
      </div>
      <p className="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
        Requests this panel handled. Nothing on this page is sample data. An access log can be
        tailed later, and it stays off until <span className="font-mono text-ink">ALTAS_ACCESS_LOG</span> is
        set. Visitor counts use a daily hash, so a return visit on another day counts again.
      </p>
      {error ? (
        <p role="alert" className="mt-6 border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </p>
      ) : null}
      {loading && !data ? <p className="mt-8 font-mono text-xs text-muted">Loading analytics.</p> : null}
      {data && totals ? (
        <>
          <dl className="mt-8 grid gap-px border border-line bg-line sm:grid-cols-2 xl:grid-cols-5">
            <Kpi label="Requests" value={formatCount(totals.requests)} />
            <Kpi label="Visitors" value={formatCount(totals.visitors)} />
            <Kpi label="Bandwidth" value={formatBytes(totals.bytes)} />
            <Kpi label="Error rate" value={formatErrorRate(totals.error_rate)} />
            <Kpi label="p95 latency" value={totals.p95_ms == null ? "—" : `${Math.round(totals.p95_ms)} ms`} />
          </dl>
          <section className="mt-4 border border-line bg-card p-4 sm:p-5">
            <h2 className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">Requests</h2>
            {hasNumbers(requests) && totals.requests > 0 ? (
              <SeriesChart
                label="Requests"
                lines={[{ name: "Requests", color: "var(--color-signal)", values: requests }]}
                formatValue={(value) => formatCount(value)}
              />
            ) : (
              <p className="mt-6 text-sm leading-relaxed text-muted">No requests in this range.</p>
            )}
          </section>
          <div className="mt-4 grid gap-4 lg:grid-cols-3">
            <RankTable title="Top hosts" rows={data.top_hosts} empty="No hosts in this range." />
            <RankTable title="Top applications" rows={data.top_apps} empty="No applications in this range." />
            <RankTable title="Top paths" rows={data.top_paths} empty="No paths in this range." />
          </div>
          <div className="mt-4 grid gap-4 lg:grid-cols-3">
            <PlaceTable
              title="Countries"
              rows={data.countries}
              empty={
                data.geo_configured
                  ? "No located requests in this range."
                  : "No GeoIP database is configured, so countries stay empty."
              }
            />
            <PlaceTable
              title="Cities"
              rows={data.cities}
              empty={
                data.geo_configured
                  ? "No located requests in this range."
                  : "No GeoIP database is configured, so cities stay empty."
              }
            />
            <RankTable title="Devices" rows={data.devices} empty="No requests in this range." />
          </div>
        </>
      ) : null}
    </DashboardLayout>
  )
}

function Kpi({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-card px-4 py-4">
      <dt className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{label}</dt>
      <dd className="mt-2 font-mono text-sm text-ink">{value}</dd>
    </div>
  )
}

function RankTable({ title, rows, empty }: { title: string; rows: AnalyticsTop[]; empty: string }) {
  return (
    <section className="border border-line bg-card p-4">
      <h2 className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{title}</h2>
      {rows.length === 0 ? (
        <p className="mt-3 text-sm leading-relaxed text-muted">{empty}</p>
      ) : (
        <table className="mt-3 w-full text-left">
          <caption className="sr-only">{title}</caption>
          <thead>
            <tr className="font-mono text-[10px] tracking-[0.12em] text-muted uppercase">
              <th scope="col" className="py-2 font-normal">
                Name
              </th>
              <th scope="col" className="py-2 text-right font-normal">
                Requests
              </th>
              <th scope="col" className="py-2 text-right font-normal">
                Bytes
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.name} className="border-t border-line font-mono text-xs">
                <th scope="row" title={row.name} className="py-2 pr-3 text-left font-normal break-all text-ink">
                  {row.name}
                </th>
                <td className="py-2 text-right text-ink">{formatCount(row.requests)}</td>
                <td className="py-2 text-right text-muted">{formatBytes(row.bytes)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}

function PlaceTable({ title, rows, empty }: { title: string; rows: AnalyticsPlace[]; empty: string }) {
  return (
    <section className="border border-line bg-card p-4">
      <h2 className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{title}</h2>
      {rows.length === 0 ? (
        <p className="mt-3 text-sm leading-relaxed text-muted">{empty}</p>
      ) : (
        <table className="mt-3 w-full text-left">
          <caption className="sr-only">{title}</caption>
          <thead>
            <tr className="font-mono text-[10px] tracking-[0.12em] text-muted uppercase">
              <th scope="col" className="py-2 font-normal">
                Place
              </th>
              <th scope="col" className="py-2 text-right font-normal">
                Requests
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={`${row.country ?? ""}-${row.name}`} className="border-t border-line font-mono text-xs">
                <th
                  scope="row"
                  title={row.country ? `${row.name} · ${row.country}` : row.name}
                  className="py-2 pr-3 text-left font-normal break-all text-ink"
                >
                  {row.country ? `${row.name} · ${row.country}` : row.name}
                </th>
                <td className="py-2 text-right text-ink">{formatCount(row.requests)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}

function formatCount(value: number) {
  if (!Number.isFinite(value)) return "0"
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value)
}

function formatErrorRate(rate: number) {
  if (!Number.isFinite(rate)) return "0.0%"
  return `${(rate * 100).toFixed(1)}%`
}
