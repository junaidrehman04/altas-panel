import { useState } from "react"
import type { HardwareView } from "../../lib/panel"
import { Capacity, HeatLegend, Heatmap, TimeSeries } from "../charts"
import { fmt, fmtInt, gb } from "../data"
import { Badge, Icon, Panel, SourceTag, Tabs } from "../ui"

export function HardwarePage({ data, loading, initialTab }: { data: HardwareView | null; loading: boolean; initialTab?: string }) {
  const [tab, setTab] = useState<"overview" | "storage" | "kernel">(() => (["overview", "storage", "kernel"] as const).find((t) => t === initialTab) ?? "overview")
  const [showAll, setShowAll] = useState(false)
  const st = loading && !data ? "loading" : "ready"
  const id = data?.identity
  const mem = data?.memory
  const ports = data?.listeners.ports ?? []
  const shown = showAll ? ports : ports.slice(0, 6)
  const exposed = ports.filter((p) => p.exposure === "public" && ["6379", "5432", "3306", "27017", "9200", "11211", "2375"].some((port) => p.addr.endsWith(":" + port))).length

  return (
    <div className="page">
      <Tabs label="Diagnostics sections" value={tab} onChange={setTab} items={[{ value: "overview", label: "Overview" }, { value: "storage", label: "Storage & sensors" }, { value: "kernel", label: "Kernel & security", badge: exposed || undefined }]} />
      {tab === "overview" && (
        <>
          <div className="facts">
            {[
              ["Platform", id?.virt || "—", id?.hypervisor ? "Hypervisor flag set" : "No hypervisor flag"],
              ["CPU", id?.cpu || "—", `${id?.cores ?? "—"} cores`],
              ["Memory", id ? gb(id.ram_bytes, 1) + " GB" : "—", "From /proc/meminfo"],
              ["Kernel", id?.kernel || "—", id?.reboot_pending ? "Reboot pending" : "No reboot flag"],
              ["OS", id?.os || "—", id?.uptime ? `Uptime ${id.uptime}` : "Uptime unavailable"],
              ["Load avg", id ? `${fmt(id.load1, 2)} · ${fmt(id.load5, 2)} · ${fmt(id.load15, 2)}` : "—", `${id?.cores ?? "—"} cores · 1 / 5 / 15 min`],
            ].map(([k, v, sub]) => (
              <div key={k} className="fact"><span>{k}</span><b className={k === "Kernel" || k === "Load avg" ? "mono" : ""}>{v}</b><small>{sub}</small></div>
            ))}
            <div className="fact-legend">
              <SourceTag kind="measured" /><SourceTag kind="calculated" /><SourceTag kind="estimated" /><SourceTag kind="unavailable" />
            </div>
          </div>
          <div className="grid g-12">
            <Panel className="span-7" title="Memory composition" definition="From /proc/meminfo. Page cache is reclaimable; MemAvailable is the kernel's estimate of memory usable without swapping."
              meta={<SourceTag kind="measured" detail="/proc/meminfo" />} state={st} minHeight={340}>
              <div className="mem-summary">
                <div className="mem-cell"><span><i className="sw sw-rss" />Application RSS</span><b className="mono">{mem ? gb(mem.rss_bytes, 1) + " GB" : "—"}</b></div>
                <div className="mem-cell"><span><i className="sw sw-buf" />Buffers</span><b className="mono">{mem ? gb(mem.buffers_bytes, 1) + " GB" : "—"}</b></div>
                <div className="mem-cell"><span><i className="sw sw-cache" />Page cache <em>reclaimable</em></span><b className="mono">{mem ? gb(mem.cache_bytes, 1) + " GB" : "—"}</b></div>
                <div className="mem-cell"><span><i className="sw sw-free" />Free</span><b className="mono">{mem ? gb(mem.free_bytes, 1) + " GB" : "—"}</b></div>
                <div className="mem-cell"><span><i className="sw sw-swap" />Swap used</span><b className="mono">{mem ? `${gb(mem.swap_used_bytes, 2)} / ${gb(mem.swap_total_bytes, 1)} GB` : "—"}</b></div>
                <div className={`mem-cell emph ${mem && mem.available_percent < 10 ? "tone-critical" : mem && mem.available_percent < 20 ? "tone-warning" : ""}`}><span>MemAvailable</span><b className="mono">{mem ? fmt(mem.available_percent, 1) + "%" : "—"}</b>{mem?.min_available_percent != null && <small>min {fmt(mem.min_available_percent, 1)}% in range</small>}</div>
              </div>
              {mem && mem.labels.length >= 2 ? (
                <TimeSeries labels={mem.labels} stacked height={200} unit="GB" format={(v) => fmt(v, 0)} yMax={id ? id.ram_bytes / (1024 ** 3) : undefined}
                  series={[
                    { key: "rss", label: "RSS", color: "var(--m-rss)", values: mem.rss },
                    { key: "buf", label: "Buffers", color: "var(--m-buf)", values: mem.buffers },
                    { key: "cache", label: "Page cache", color: "var(--m-cache)", values: mem.cache },
                    { key: "avail", label: "MemAvailable", color: "var(--heading)", values: mem.available, overlay: true, dashed: true },
                  ]} legend={false} />
              ) : <p className="note">History starts after two samples.</p>}
              <div className="psi-row">
                <span className="muted">Pressure stall (PSI)</span>
                {mem?.psi.available ? (
                  <>
                    <span className="mono">memory some avg10 <b>{fmt(mem.psi.memory_some, 2)}%</b></span>
                    <span className="mono">full avg10 <b>{fmt(mem.psi.memory_full, 2)}%</b></span>
                    <span className="mono">io some avg10 <b>{fmt(mem.psi.io_some, 2)}%</b></span>
                  </>
                ) : <span>PSI is not exposed by this kernel.</span>}
              </div>
            </Panel>
            <Panel className="span-5" title="Per-core CPU utilisation" definition="user + system + irq + softirq per logical core." state={data && data.heat.labels.length >= 2 ? "ready" : st === "loading" ? "loading" : "empty"} stateTitle="Not enough per-core samples" stateText="A second sample is required before the heatmap has a time axis." minHeight={340} meta={<SourceTag kind="measured" detail="/proc/stat" />}>
              {data && data.heat.labels.length >= 2 && (
                <>
                  <Heatmap data={data.heat.values} rowLabels={data.heat.cores} colLabels={data.heat.labels} cell={data.heat.cores.length > 8 ? 14 : 22} />
                  <HeatLegend />
                </>
              )}
            </Panel>
            <Panel className="span-12" title="CPU steal time" definition="Time the hypervisor ran other tenants while this vCPU was runnable."
              state={data?.steal.applicable ? (data.steal.labels.length >= 2 ? "ready" : "empty") : "unavailable"}
              stateTitle={data?.steal.applicable ? "Not enough samples yet" : "Not applicable on bare metal"}
              stateText={data?.steal.reason || "Steal time is only reported inside virtual machines."}
              meta={data?.steal.applicable ? <SourceTag kind="measured" detail="/proc/stat steal" /> : undefined} minHeight={220}>
              {data?.steal.applicable && data.steal.labels.length >= 2 && (
                <>
                  <div className="inline-stats">
                    <div><span>Current</span><b className="mono">{data.steal.current == null ? "—" : `${fmt(data.steal.current, 1)}%`}</b></div>
                  </div>
                  <TimeSeries labels={data.steal.labels} height={170} unit="%" format={(v) => fmt(v, 0)} series={[{ key: "st", label: "Steal", color: "var(--c-warn-line)", values: data.steal.values }]} />
                </>
              )}
            </Panel>
          </div>
        </>
      )}
      {tab === "storage" && (
        <div className="grid g-12">
          <Panel className="span-12" title="Filesystems, inodes & I/O wait" definition="Space and inode usage per mount (statvfs). Warning 80%, critical 90%." state={st} minHeight={280} flush>
            <div className="split">
              <div className="table-wrap"><table className="table">
                <thead><tr><th>Mount</th><th>Space</th><th>Inodes</th></tr></thead>
                <tbody>
                  {data?.filesystems.map((f) => (
                    <tr key={f.mount}>
                      <td><div className="cell-stack"><span className="mono strong">{f.mount}</span><small className="mono">{f.device} · {f.fs}</small></div></td>
                      <td className="cap-cell"><Capacity value={f.used_percent} max={100} warn={80} crit={90} display={`${fmt(f.used_percent, 0)}%`} sub={<span className="mono">{gb(f.used_bytes, 1)} / {gb(f.total_bytes, 1)} GB</span>} /></td>
                      <td className="cap-cell">{f.inode_percent == null ? "—" : <Capacity value={f.inode_percent} max={100} warn={80} crit={90} display={`${fmt(f.inode_percent, 0)}%`} />}</td>
                    </tr>
                  ))}
                </tbody>
              </table></div>
              <div className="split-side">
                <div className="side-title">CPU iowait</div>
                {data && data.iowait.labels.length >= 2 ? (
                  <TimeSeries labels={data.iowait.labels} height={150} unit="%" format={(v) => fmt(v, 0)} legend={false} series={[{ key: "io", label: "iowait", color: "var(--c-warn-line)", values: data.iowait.values }]} />
                ) : <p className="note">iowait history starts after two samples.</p>}
                <p className="note">{data?.iowait.peak != null ? <>Peak <b className="mono">{fmt(data.iowait.peak, 1)}%</b> in this range.</> : "No iowait samples yet."}</p>
              </div>
            </div>
          </Panel>
          <Panel className="span-8" title="NVMe, RAID & SMART health" definition="md arrays from /proc/mdstat and NVMe identity from sysfs. Wear is shown only when the kernel exposes it." flush
            state={data?.raid.available || data?.nvme.available ? "ready" : "unavailable"} stateTitle="Physical disk telemetry is not visible" stateText={data?.raid.reason || data?.nvme.reason} minHeight={260}>
            {!!data?.raid.arrays.length && (
              <div className="raid-strip">
                {data.raid.arrays.map((raid) => (
                  <div className="raid" key={raid.name}><b className="mono">{raid.name}</b><span>{raid.level}</span><span className="mono">{raid.members}</span><Badge tone="ok">{raid.state}</Badge></div>
                ))}
              </div>
            )}
            {!!data?.nvme.devices.length && (
              <div className="table-wrap"><table className="table">
                <thead><tr><th>Device</th><th>Model</th><th className="num">Temp</th><th>Wear</th></tr></thead>
                <tbody>
                  {data.nvme.devices.map((dev) => (
                    <tr key={dev.name}><td className="mono strong">{dev.name}</td><td>{dev.model || "—"}</td><td className="num mono">{dev.temp_c == null ? "—" : `${fmt(dev.temp_c, 0)} °C`}</td><td><SourceTag kind="unavailable" /></td></tr>
                  ))}
                </tbody>
              </table></div>
            )}
          </Panel>
          <Panel className="span-4" title="IPMI sensors" definition="BMC readings are not collected by running ipmitool." flush state="unavailable" stateTitle="IPMI readings are not available" stateText={data?.ipmi.reason} minHeight={260} />
        </div>
      )}
      {tab === "kernel" && (
        <div className="grid g-12">
          <Panel className="span-12" title="Kernel & system health timeline" definition="Lines read from a kernel log when this process can open it." state={data?.kernel.available && data.kernel.events.length ? "ready" : data?.kernel.reason ? "unavailable" : "empty"} stateTitle={data?.kernel.reason ? "Kernel log is not readable" : "No matching kernel events"} stateText={data?.kernel.reason || "No OOM, hung-task, NVMe, or SYN-flood lines were in the readable tail."} minHeight={160}>
            <ul className="event-list">
              {data?.kernel.events.map((e) => (
                <li key={e.label} className={`event sev-${e.tone}`}><span className="mono event-time">{e.lane}</span><div><b>{e.tone}</b><p>{e.label}</p></div></li>
              ))}
            </ul>
          </Panel>
          <Panel className="span-6" title="Deleted but still open files" definition="Unlinked paths still held open by a process this user can see." flush state={st} minHeight={200} meta={(data?.deleted.files.length ?? 0) > 0 ? <Badge tone="warning">{data?.deleted.files.length} open</Badge> : <Badge tone="ok">None</Badge>}>
            {data && data.deleted.files.length === 0 ? (
              <div className="panel-state state-empty"><Icon name="check" size={18} /><b>No deleted-but-open files</b><p>{data.deleted.note || "All unlinked files visible to this user have been released."}</p></div>
            ) : (
              <div className="table-wrap"><table className="table">
                <thead><tr><th>Process</th><th>Path</th><th className="num">Size</th></tr></thead>
                <tbody>
                  {data?.deleted.files.map((g) => (
                    <tr key={g.pid + g.path}><td><div className="cell-stack"><span className="mono strong">{g.process}</span><small className="mono">PID {g.pid}</small></div></td><td><code className="truncate path">{g.path}</code></td><td className="num mono">{gb(g.bytes, 2)} GB</td></tr>
                  ))}
                </tbody>
              </table></div>
            )}
          </Panel>
          <Panel className="span-6" title="Listening ports" definition="Sockets from /proc/net. Firewall rules are not read, so exposure is the bind address only." flush state={st} minHeight={200} meta={exposed ? <Badge tone="critical">{exposed} sensitive</Badge> : undefined}>
            <div className="table-wrap"><table className="table compact">
              <thead><tr><th>Proto</th><th>Local address</th><th>Process</th><th>User</th><th>Exposure</th><th>Firewall</th></tr></thead>
              <tbody>
                {shown.map((p) => (
                  <tr key={p.proto + p.addr}>
                    <td className="mono muted">{p.proto}</td>
                    <td className="mono strong">{p.addr}</td>
                    <td><span className="mono">{p.process || "—"}</span> {p.pid ? <small className="muted mono">{p.pid}</small> : null}</td>
                    <td className="mono muted">{p.user || "—"}</td>
                    <td>{p.exposure === "public" ? <Badge tone="info">Public</Badge> : <Badge tone="muted">{p.exposure === "local" ? "Loopback" : "Private"}</Badge>}</td>
                    <td className="mono muted">not visible</td>
                  </tr>
                ))}
              </tbody>
            </table></div>
            <div className="panel-foot">
              <span>{data?.listeners.note || "Firewall rules are not read by this panel."}</span>
              {ports.length > 6 && <button className="link" onClick={() => setShowAll((v) => !v)}>{showAll ? "Show fewer" : `Show all ${ports.length}`}</button>}
            </div>
          </Panel>
          <Panel className="span-12" title="SSH authentication & intrusion activity" definition="Failed authentication lines from a readable auth log. Nothing is copied into the panel database." state={data?.ssh.available ? "ready" : "unavailable"} stateTitle="SSH log is not readable" stateText={data?.ssh.reason} minHeight={220} meta={<SourceTag kind={data?.ssh.available ? "measured" : "unavailable"} />}>
            {data?.ssh.available && (
              <div className="ssh-layout">
                <div>
                  <div className="inline-stats">
                    <div><span>Failed attempts in the readable tail</span><b className="mono">{fmtInt(data.ssh.failed)}</b></div>
                    <div><span>Sources listed</span><b className="mono">{fmtInt(data.ssh.sources.length)}</b></div>
                  </div>
                </div>
                <div className="table-wrap"><table className="table compact">
                  <thead><tr><th>Source</th><th className="num">Failures</th><th>Usernames</th></tr></thead>
                  <tbody>
                    {data.ssh.sources.map((r) => (
                      <tr key={r.ip}><td className="mono strong">{r.ip}</td><td className="num mono">{fmtInt(r.fails)}</td><td className="mono muted">{r.users || "—"}</td></tr>
                    ))}
                  </tbody>
                </table></div>
              </div>
            )}
          </Panel>
        </div>
      )}
    </div>
  )
}
