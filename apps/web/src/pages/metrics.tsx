import { useEffect, useState } from "react"
import { DashboardLayout } from "../components/dashboard-layout"
import { RangePicker } from "../components/range-picker"
import { SeriesChart } from "../components/series-chart"
import { formatBytes } from "../lib/format"
import type { RangeId } from "../lib/ranges"
import { hasNumbers, usePanelQuery, type HostSeries } from "../lib/series"

export function MetricsPage() {
  const [range, setRange] = useState<RangeId>("live")
  const { data, error, loading } = usePanelQuery<HostSeries>("/api/metrics/host/series", range)

  useEffect(() => {
    document.title = "Metrics · Altas Panel"
  }, [])

  const points = data?.points ?? []

  return (
    <DashboardLayout section="Metrics">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">03 / Metrics</p>
          <h1 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">Host history</h1>
        </div>
        <RangePicker value={range} onChange={setRange} />
      </div>
      <p className="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
        CPU, memory, disk, and network from this machine. Live is the last 15 minutes. Months are
        daily averages grouped by calendar month. The disk line follows the largest mount.
      </p>
      {error ? (
        <p role="alert" className="mt-6 border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </p>
      ) : null}
      {loading && !data ? <p className="mt-8 font-mono text-xs text-muted">Loading host history.</p> : null}
      {data ? (
        <div className="mt-8 grid gap-4 lg:grid-cols-2">
          <ChartCard
            title="CPU"
            detail={latest(points.map((point) => point.cpu_percent), (value) => `${value.toFixed(1)}%`)}
            empty="No CPU samples in this range yet. The first reading waits for a second sample."
            lines={[
              {
                name: "CPU",
                color: "var(--color-signal)",
                values: points.map((point) => point.cpu_percent),
              },
            ]}
            formatValue={(value) => `${value.toFixed(0)}%`}
          />
          <ChartCard
            title="Memory"
            detail={latest(points.map((point) => point.mem_used_percent), (value) => `${value.toFixed(1)}%`)}
            empty="No memory samples in this range yet."
            lines={[
              {
                name: "Memory",
                color: "var(--color-signal)",
                values: points.map((point) => point.mem_used_percent),
              },
            ]}
            formatValue={(value) => `${value.toFixed(0)}%`}
          />
          <ChartCard
            title="Disk"
            detail={latest(points.map((point) => point.disk_used_percent), (value) => `${value.toFixed(1)}%`)}
            empty="No disk samples in this range yet."
            lines={[
              {
                name: "Disk",
                color: "var(--color-accent)",
                values: points.map((point) => point.disk_used_percent),
              },
            ]}
            formatValue={(value) => `${value.toFixed(0)}%`}
          />
          <ChartCard
            title="Network"
            detail={latest(points.map((point) => point.rx_bps), (value) => `${formatBytes(value)}/s receive`)}
            empty="No network rates in this range yet."
            lines={[
              {
                name: "Receive",
                color: "var(--color-signal)",
                values: points.map((point) => point.rx_bps),
              },
              {
                name: "Transmit",
                color: "var(--color-accent)",
                values: points.map((point) => point.tx_bps),
              },
            ]}
            formatValue={(value) => `${formatBytes(value)}/s`}
          />
        </div>
      ) : null}
    </DashboardLayout>
  )
}

function ChartCard({
  title,
  detail,
  empty,
  lines,
  formatValue,
}: {
  title: string
  detail: string
  empty: string
  lines: Array<{ name: string; color: string; values: Array<number | null> }>
  formatValue: (value: number) => string
}) {
  const ready = lines.some((line) => hasNumbers(line.values))
  return (
    <section className="border border-line bg-card p-4 sm:p-5">
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{title}</h2>
        <p className="font-mono text-xs text-ink">{ready ? detail : "—"}</p>
      </div>
      {ready ? (
        <SeriesChart label={title} lines={lines} formatValue={formatValue} />
      ) : (
        <p className="mt-6 text-sm leading-relaxed text-muted">{empty}</p>
      )}
    </section>
  )
}

function latest(values: Array<number | null>, format: (value: number) => string) {
  for (let index = values.length - 1; index >= 0; index -= 1) {
    const value = values[index]
    if (value != null && !Number.isNaN(value)) return format(value)
  }
  return "—"
}
