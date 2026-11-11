import { useState } from "react"
import type { ApmView, ProjectRow } from "../../lib/panel"
import { TimeSeries } from "../charts"
import { compact, fmt, fmtInt, gb } from "../data"
import { Badge, Icon, Panel, SourceTag, Tabs } from "../ui"

type Tab = "overview" | "performance" | "data" | "jobs"
type SortKey = "cpu" | "ram" | "rps" | "p95" | "err"

const COLS: { key: SortKey; label: string; unit: string }[] = [
  { key: "cpu", label: "CPU", unit: "% host" },
  { key: "ram", label: "RAM (RSS)", unit: "GB" },
  { key: "rps", label: "Req/s", unit: "" },
  { key: "p95", label: "p95", unit: "ms" },
  { key: "err", label: "5xx", unit: "%" },
]

function valueOf(project: ProjectRow, key: SortKey) {
  if (key === "cpu") return project.cpu
  if (key === "ram") return project.ram_bytes
  if (key === "rps") return project.rps
  if (key === "p95") return project.p95_ms
  return project.err5xx
}

function formatCell(project: ProjectRow, key: SortKey) {
  const value = valueOf(project, key)
  if (value == null) return "—"
  if (key === "ram") return gb(value, 2)
  if (key === "cpu" || key === "err") return fmt(value, key === "err" ? 2 : 1)
  if (key === "p95") return fmtInt(value)
  return fmt(value, value >= 10 ? 1 : 2)
}

export function ApmPage({ data, loading, initialTab, onSelect }: { data: ApmView | null; loading: boolean; initialTab?: string; onSelect: (id: string) => void }) {
  const [tab, setTab] = useState<Tab>(() => (["overview", "performance", "data", "jobs"] as const).find((t) => t === initialTab) ?? "overview")
  const [sort, setSort] = useState<SortKey>("cpu")
  const st = loading && !data ? "loading" : "ready"
  const projects = [...(data?.projects ?? [])].sort((a, b) => (valueOf(b, sort) ?? -1) - (valueOf(a, sort) ?? -1))
  const selected = data?.projects.find((p) => p.id === data.selected) ?? data?.projects[0]
  const hostCpu = data?.host.cpu_percent ?? 0
  const usedRam = data?.projects.reduce((sum, p) => sum + (p.ram_bytes ?? 0), 0) ?? 0
  const ram = data?.host.ram_bytes || 1

  return (
    <div className="page">
      <Tabs label="Application sections" value={tab} onChange={setTab} items={[{ value: "overview", label: "Overview" }, { value: "performance", label: "Performance" }, { value: "data", label: "Code & data" }, { value: "jobs", label: "Jobs & mail" }]} />
      {tab === "overview" && (
        <>
          <Panel title="Host resource attribution" definition="Share of this host used by the panel process and any containers visible on the Docker socket."
            meta={<SourceTag kind="measured" detail="/proc · docker socket" />} state={st} minHeight={150}>
            <div className="attribution">
              {[
                { label: "CPU", total: `${fmt(hostCpu, 1)}% of ${data?.host.threads || data?.host.cores || 1} threads`, parts: projects.filter((p) => p.cpu != null), sys: Math.max(0, hostCpu - projects.reduce((s, p) => s + (p.cpu ?? 0), 0)), free: Math.max(0, 100 - hostCpu) },
                { label: "Memory (RSS)", total: `${gb(usedRam, 1)} of ${gb(ram, 1)} GB`, parts: projects.filter((p) => p.ram_bytes != null), sys: 0, free: Math.max(0, 100 - (usedRam / ram) * 100) },
              ].map((row) => (
                <div className="attr-row" key={row.label}>
                  <div className="attr-label"><b>{row.label}</b><span className="mono">{row.total}</span></div>
                  <div className="attr-bar">
                    {row.parts.map((p) => {
                      const width = row.label === "CPU" ? (p.cpu ?? 0) : ((p.ram_bytes ?? 0) / ram) * 100
                      return (
                        <button key={p.id} style={{ width: `${Math.max(width, 0)}%`, background: p.color }} className={selected?.id === p.id ? "sel" : "dim"} onClick={() => { onSelect(p.id); setTab("performance") }} title={`${p.id}: ${fmt(width, 1)}%`}>
                          {width > 7 && <span>{p.id}</span>}
                        </button>
                      )
                    })}
                    {row.sys > 0.4 && <span className="attr-sys" style={{ width: `${row.sys}%` }} title="Other host CPU" />}
                    <span className="attr-free" style={{ width: `${Math.max(0, row.free)}%` }} />
                  </div>
                </div>
              ))}
              <div className="attr-legend">
                {projects.map((p) => <button key={p.id} className={`legend-item ${selected?.id === p.id ? "strong" : ""}`} onClick={() => onSelect(p.id)}><i style={{ background: p.color }} />{p.id}</button>)}
                <span className="legend-item"><i className="sys-swatch" />System &amp; kernel</span>
                <span className="legend-item"><i className="free-swatch" />Available</span>
              </div>
            </div>
          </Panel>
          <Panel title="Resource consumption leaderboard" definition="Current process and container samples. Cells stay blank when that source did not report a number." flush state={st === "loading" ? "loading" : projects.length ? "ready" : "empty"} stateTitle="No projects visible" minHeight={220}
            meta={<span className="muted small">{projects.length} rows · sorted by <b>{COLS.find((c) => c.key === sort)?.label}</b></span>}>
            <div className="table-wrap"><table className="table leaderboard">
              <thead>
                <tr>
                  <th>#</th><th>Project</th>
                  {COLS.map((c) => (
                    <th key={c.key} className="num" aria-sort={sort === c.key ? "descending" : "none"}>
                      <button className={`th-sort ${sort === c.key ? "on" : ""}`} onClick={() => setSort(c.key)}>{c.label}{c.unit && <small>{c.unit}</small>}<Icon name="down" size={11} /></button>
                    </th>
                  ))}
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {projects.map((p, i) => (
                  <tr key={p.id} className={`clickable ${selected?.id === p.id ? "row-selected" : ""}`} onClick={() => { onSelect(p.id); setTab("performance") }}>
                    <td className="mono muted">{i + 1}</td>
                    <td><div className="project-cell"><i style={{ background: p.color }} /><div><b>{p.id}</b><small className="mono">{p.domain} · {p.stack}</small></div></div></td>
                    {COLS.map((c) => <td key={c.key} className="num mono">{formatCell(p, c.key)}</td>)}
                    <td>{p.status === "critical" ? <Badge tone="critical">SLO breach</Badge> : p.status === "warning" ? <Badge tone="warning">Degraded</Badge> : <Badge tone="ok">Healthy</Badge>}</td>
                  </tr>
                ))}
              </tbody>
            </table></div>
          </Panel>
        </>
      )}
      {tab !== "overview" && selected && (
        <div className="investigate-bar">
          <div className="project-cell"><i style={{ background: selected.color }} /><div>
            <label className="project-select"><span className="sr-only">Project</span>
              <select value={selected.id} onChange={(e) => onSelect(e.target.value)}>{data?.projects.map((p) => <option key={p.id} value={p.id}>{p.id}</option>)}</select>
              <Icon name="down" size={13} />
            </label>
            <small className="mono">{selected.domain} · {selected.stack}</small>
          </div></div>
        </div>
      )}
      {tab === "performance" && (
        <div className="grid g-12">
          <Panel className="span-6" title="Requests per second" definition="Requests recorded for the panel API in this range." state={loading && !data ? "loading" : data?.http.available && data.http.labels.length >= 2 ? "ready" : "empty"} stateTitle="No HTTP series yet" stateText={data?.http.reason || "Two buckets are required before the chart is drawn."} minHeight={240}>
            {data && data.http.labels.length >= 2 && <TimeSeries labels={data.http.labels} height={190} unit="req/s" format={(v) => compact(v)} series={[{ key: "r", label: "Current", color: "var(--c-primary-line)", values: data.http.rps }]} />}
          </Panel>
          <Panel className="span-6" title="Response time percentiles" definition="p95 from the panel latency histogram. p50 and p99 are not stored separately." state={loading && !data ? "loading" : data?.http.available && data.http.labels.length >= 2 ? "ready" : "empty"} stateTitle="No timed requests" stateText="p95 stays empty until a request includes a latency." minHeight={240}>
            {data && data.http.labels.length >= 2 && <TimeSeries labels={data.http.labels} height={190} unit="ms" format={(v) => fmt(v, 0)} series={[{ key: "p95", label: "p95", color: "var(--c-p95)", values: data.http.p95 }]} />}
          </Panel>
          <Panel className="span-4" title="HTTP status distribution" definition="5xx versus every other status. Individual codes are not stored." state={data?.http.available ? "ready" : "empty"} stateTitle="No HTTP traffic" flush minHeight={200}>
            <div className="table-wrap"><table className="table compact"><tbody>
              <tr><td className="mono strong">2xx–4xx</td><td className="muted">Not a server error</td><td className="num mono">{fmtInt(data?.http.ok ?? 0)}</td></tr>
              <tr><td className="mono strong">5xx</td><td className="muted">Server error</td><td className="num mono">{fmtInt(data?.http.errors ?? 0)}</td></tr>
            </tbody></table></div>
          </Panel>
          <Panel className="span-8" title="Error rate" definition="Percentage of responses at status 500 or higher." state={data?.http.available && (data.http.labels.length ?? 0) >= 2 ? "ready" : "empty"} stateTitle="No HTTP series yet" minHeight={200}>
            {data && data.http.labels.length >= 2 && <TimeSeries labels={data.http.labels} height={160} unit="%" format={(v) => fmt(v, 1)} series={[{ key: "e5", label: "5xx", color: "var(--s5xx)", values: data.http.error_rate }]} />}
          </Panel>
        </div>
      )}
      {tab === "data" && (
        <div className="grid g-12">
          <Panel className="span-7" title="Slow endpoints" definition="Paths ranked by request count. Per-path latency is not stored." flush state={data?.http.paths.length ? "ready" : "empty"} stateTitle="No paths in this range" minHeight={220}>
            <div className="table-wrap"><table className="table">
              <thead><tr><th>Endpoint</th><th className="num">Calls</th><th className="num">Bytes</th><th className="num">p95</th><th className="num">DB time</th></tr></thead>
              <tbody>
                {data?.http.paths.map((e) => (
                  <tr key={e.path}><td><span className="mono strong">{e.path}</span></td><td className="num mono">{compact(e.requests)}</td><td className="num mono">{compact(e.bytes)}</td><td className="num mono">—</td><td className="num mono">—</td></tr>
                ))}
              </tbody>
            </table></div>
          </Panel>
          <Panel className="span-5" title="Cache efficiency" state="unavailable" stateTitle="No object cache measured" stateText={data?.cache.reason} minHeight={220} />
          <Panel className="span-12" title="SQL execution analysis" state="unavailable" stateTitle="No database attached" stateText={data?.sql.reason} minHeight={180} />
        </div>
      )}
      {tab === "jobs" && (
        <div className="grid g-12">
          <Panel className="span-12" title="Background workers & queues" state="unavailable" stateTitle="Queues are not measured" stateText={data?.queues.reason} minHeight={180} />
          <Panel className="span-8" title="Scheduled tasks (cron)" definition="Readable crontab lines. Last run and duration are not known." flush state={data?.cron.available && data.cron.jobs.length ? "ready" : data?.cron.available ? "empty" : "unavailable"} stateTitle={data?.cron.available ? "No crontab entries" : "Crontab is not readable"} stateText={data?.cron.reason} minHeight={220}>
            <div className="table-wrap"><table className="table">
              <thead><tr><th>Command</th><th>Schedule</th><th>Source</th><th>Last run</th></tr></thead>
              <tbody>
                {data?.cron.jobs.map((c) => (
                  <tr key={c.command + c.schedule}><td><code className="truncate">{c.command}</code></td><td className="mono">{c.schedule}</td><td className="mono muted">{c.source}</td><td className="mono">—</td></tr>
                ))}
              </tbody>
            </table></div>
          </Panel>
          <Panel className="span-4" title="Mail delivery" definition="Postfix status tokens in the readable mail log." state={data?.mail.available ? "ready" : "unavailable"} stateTitle="Mail log is not readable" stateText={data?.mail.reason} minHeight={220} meta={<SourceTag kind={data?.mail.available ? "measured" : "unavailable"} />}>
            {data?.mail.available && (
              <div className="kv-grid">
                <div className="kv"><span>Delivered</span><b className="mono">{fmtInt(data.mail.sent)}</b></div>
                <div className="kv"><span>Deferred</span><b className="mono">{fmtInt(data.mail.deferred)}</b></div>
                <div className="kv"><span>Bounced</span><b className="mono">{fmtInt(data.mail.bounced)}</b></div>
              </div>
            )}
          </Panel>
        </div>
      )}
    </div>
  )
}
