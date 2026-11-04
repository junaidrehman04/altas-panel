import { useCallback, useEffect, useState } from "react"
import { formatBytes, formatPercent, formatRate, formatUptime } from "../lib/format"
import { api, ApiError } from "../lib/api"
import type { HostSnapshot } from "../lib/host"
import { Button } from "./ui/button"

const pollMs = 5000

export function HostMetrics() {
  const [data, setData] = useState<HostSnapshot | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [updated, setUpdated] = useState<Date | null>(null)

  const load = useCallback(async () => {
    try {
      const snap = await api<HostSnapshot>("/api/metrics/host")
      setData(snap)
      setError(null)
      setUpdated(new Date())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not read host metrics.")
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const tick = async () => {
      try {
        const snap = await api<HostSnapshot>("/api/metrics/host")
        if (cancelled) return
        setData(snap)
        setError(null)
        setUpdated(new Date())
      } catch (err) {
        if (cancelled) return
        setError(err instanceof ApiError ? err.message : "Could not read host metrics.")
      }
    }
    void tick()
    const timer = window.setInterval(() => void tick(), pollMs)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [])

  const memoryPercent = data && data.memory.total_bytes > 0 ? data.memory.used_percent : null

  return (
    <section className="mt-8" aria-labelledby="host-metrics-heading">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="font-mono text-[11px] tracking-[0.16em] text-signal uppercase">True state</p>
          <h2 id="host-metrics-heading" className="mt-2 text-xl font-semibold tracking-tight">
            This host
          </h2>
        </div>
        <div className="flex items-center gap-2">
          <p className="font-mono text-[11px] text-muted">
            {updated ? `Updated ${updated.toLocaleTimeString()}` : "Waiting"}
          </p>
          <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
            Refresh
          </Button>
        </div>
      </div>
      {error ? (
        <p role="alert" className="mt-4 border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </p>
      ) : null}
      {data ? (
        <>
          <p className="mt-3 max-w-2xl text-sm leading-relaxed text-muted">
            {data.hostname || "Local host"} · up {formatUptime(data.uptime_seconds)} · source{" "}
            {data.source === "proc" ? "/proc" : data.source}. Used memory is total minus available.
            Cache is shown on its own and is not treated as a full box.
          </p>
          {data.note ? <p className="mt-2 text-sm text-muted">{data.note}</p> : null}
          <div className="mt-4 grid gap-px border border-line bg-line lg:grid-cols-3">
            <Meter
              label="CPU"
              percent={data.cpu.usage_percent}
              detail={`${data.cpu.cores} cores · load ${data.load.load1.toFixed(2)} ${data.load.load5.toFixed(2)} ${data.load.load15.toFixed(2)}`}
            />
            <Meter
              label="Memory"
              percent={memoryPercent}
              detail={`${formatBytes(data.memory.used_bytes)} used · ${formatBytes(data.memory.available_bytes)} available · cache ${formatBytes(data.memory.cache_bytes)}`}
            />
            <Meter
              label="Disk"
              percent={data.disks[0] ? data.disks[0].used_percent : null}
              detail={
                data.disks[0]
                  ? `${data.disks[0].mount} · ${formatBytes(data.disks[0].available_bytes)} free of ${formatBytes(data.disks[0].total_bytes)}`
                  : "No disk mounts reported"
              }
            />
          </div>
          <div className="mt-4 grid gap-4 lg:grid-cols-2">
            <div className="corner border border-line bg-card p-4">
              <h3 className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">Disks</h3>
              {data.disks.length === 0 ? (
                <p className="mt-3 text-sm text-muted">No disk mounts reported.</p>
              ) : (
                <ul className="mt-3 grid gap-3">
                  {data.disks.slice(0, 4).map((disk) => (
                    <li key={`${disk.mount}-${disk.filesystem}`} className="font-mono text-xs text-ink">
                      <span className="text-muted">{disk.mount}</span> {disk.filesystem} ·{" "}
                      {formatPercent(disk.used_percent)} · {formatBytes(disk.available_bytes)} free
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div className="corner border border-line bg-card p-4">
              <h3 className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">Network</h3>
              {data.network.length === 0 ? (
                <p className="mt-3 text-sm text-muted">No interfaces reported.</p>
              ) : (
                <ul className="mt-3 grid gap-3">
                  {data.network.slice(0, 4).map((iface) => (
                    <li key={iface.name} className="font-mono text-xs text-ink">
                      <span className="text-muted">{iface.name}</span> rx {formatRate(iface.rx_bps)} · tx{" "}
                      {formatRate(iface.tx_bps)}
                      {iface.rx_errors + iface.tx_errors > 0
                        ? ` · ${iface.rx_errors + iface.tx_errors} errors`
                        : ""}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </>
      ) : null}
    </section>
  )
}

function Meter({ label, percent, detail }: { label: string; percent: number | null; detail: string }) {
  const width = percent == null ? 0 : Math.max(0, Math.min(100, percent))
  return (
    <div className="bg-card px-4 py-4">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{label}</h3>
        <span className="font-mono text-sm text-ink">{formatPercent(percent)}</span>
      </div>
      <div
        className="mt-3 h-1.5 bg-base"
        role="meter"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(width)}
        aria-valuetext={formatPercent(percent)}
      >
        <div className="h-full bg-signal" style={{ width: `${width}%` }} />
      </div>
      <p className="mt-2 font-mono text-[11px] leading-relaxed text-muted">{detail}</p>
    </div>
  )
}
