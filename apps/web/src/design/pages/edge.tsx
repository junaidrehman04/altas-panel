import { useMemo, useState } from "react"
import type { EdgeView } from "../../lib/panel"
import { deltaText } from "../../lib/panel"
import { Capacity, TimeSeries } from "../charts"
import { compact, fmt, fmtInt } from "../data"
import { GeoMap, type City } from "../geomap"
import { Badge, Panel, SourceTag, Stat, Tabs } from "../ui"

function num(value: number | null | undefined, digits = 0) {
  if (value == null || Number.isNaN(value)) return "—"
  return digits === 0 ? fmt(value, 0) : fmt(value, digits)
}

export function EdgePage({ data, loading, initialTab }: { data: EdgeView | null; loading: boolean; initialTab?: string }) {
  const [tab, setTab] = useState<"overview" | "bandwidth" | "security">(() => (["overview", "bandwidth", "security"] as const).find((t) => t === initialTab) ?? "overview")
  const [city, setCity] = useState<string | null>(null)
  const state = loading && !data ? "loading" : "ready"
  const net = data?.network
  const traffic = data?.traffic
  const cities = useMemo<City[]>(() => {
    return (traffic?.countries ?? [])
      .filter((c) => c.lat != null && c.lon != null)
      .map((c) => ({
        city: c.name,
        cc: c.code,
        country: c.name,
        lat: c.lat as number,
        lon: c.lon as number,
        rps: c.requests,
        asn: "ASN unavailable",
        mix: { human: 1, bot: 0, ai: 0, mal: 0 },
      }))
  }, [traffic])
  const total = traffic?.countries.reduce((sum, c) => sum + c.requests, 0) ?? 0
  const maxCountry = traffic?.countries[0]?.requests ?? 1
  const securityCount = data?.events.filter((e) => e.sev === "critical" || e.sev === "warning").length ?? 0
  const showMap = Boolean(traffic?.geo_configured && cities.length > 0)
  const attribution = "This product includes GeoLite2 data created by MaxMind, available from https://www.maxmind.com"

  return (
    <div className="page">
      <Tabs label="Edge sections" value={tab} onChange={setTab} items={[{ value: "overview", label: "Overview" }, { value: "bandwidth", label: "Bandwidth" }, { value: "security", label: "Security", badge: securityCount || undefined }]} />
      {tab === "overview" && (
        <>
          <div className="stat-strip">
            <Stat label="Ingress" value={num(net?.ingress_mbps)} unit={net?.ingress_mbps == null ? undefined : "Mbps"} delta={deltaText(net?.ingress_delta)} context={net?.peak_ingress_mbps != null ? <>peak {fmt(net.peak_ingress_mbps, 0)} Mbps · range</> : "No interface rate yet"} source={net?.available ? "live" : "unavailable"} />
            <Stat label="Egress" value={num(net?.egress_mbps)} unit={net?.egress_mbps == null ? undefined : "Mbps"} delta={deltaText(net?.egress_delta)} deltaTone={net?.egress_delta != null && net.egress_delta > 10 ? "warning" : "ok"} context={net?.iface ? `${net.iface} · bytes/s × 8` : "No interface rate yet"} />
            <Stat label="Requests" value={traffic?.req_per_s == null ? "—" : fmt(traffic.req_per_s, traffic.req_per_s >= 10 ? 0 : 2)} unit={traffic?.req_per_s == null ? undefined : "req/s"} context={traffic ? `${fmtInt(traffic.requests)} in range · ${traffic.source}` : "No requests recorded"} />
            <Stat label="Unique client IPs" value="—" context={traffic ? `${fmtInt(traffic.visitors)} daily visitor hashes · addresses are not stored` : "Addresses are not stored"} />
            <Stat label="Blocked requests" value="—" context="No WAF or block log is configured" />
            <Stat label="Malicious share" value="—" unit={undefined} context="No classifier is configured" source="unavailable" />
          </div>
          <div className="grid g-12">
            <Panel className="span-8" title="Geo-IP traffic distribution" definition="Requests by client location when a local MaxMind database is configured. Marker area follows request count, not a live rate."
              state={!data || state === "loading" ? "loading" : traffic?.geo_configured && cities.length > 0 ? "ready" : "unavailable"}
              stateTitle={traffic?.geo_configured ? "No located requests in this range" : "GeoIP database is not configured"}
              stateText={traffic?.geo_configured ? "Requests were counted, but none resolved to a country centroid." : "Set MAXMIND_LICENSE_KEY to a MaxMind GeoLite2 license key, or set ALTAS_GEOIP_DB to an absolute .mmdb path. Missing files leave the map empty."}
              minHeight={420}
              meta={<SourceTag kind={traffic?.geo_configured ? "measured" : "unavailable"} detail={traffic?.note} />}>
              <GeoMap cities={cities} active={new Set(["human", "bot", "ai", "mal"])} mode="volume" neutral selected={city} onSelect={setCity} />
              {showMap ? <p className="note"><span style={{ minWidth: 0 }}>{attribution}</span></p> : null}
            </Panel>
            <Panel className="span-4" title="Top countries & ASNs" definition="Share of counted requests. Malicious share is not measured." state={!data || state === "loading" ? "loading" : traffic && traffic.countries.length > 0 ? "ready" : "empty"} stateTitle="No countries in this range" stateText={traffic?.note} flush minHeight={420}>
              <div className="table-wrap"><table className="table compact">
                <thead><tr><th>Country</th><th className="num">Requests</th><th>Share</th><th className="num">Malicious</th></tr></thead>
                <tbody>
                  {traffic?.countries.slice(0, 8).map((c) => (
                    <tr key={c.code}>
                      <td><div className="cell-stack"><span><b className="mono cc">{c.code}</b>{c.name}</span><small className="mono">ASN unavailable</small></div></td>
                      <td className="num mono">{fmtInt(c.requests)}</td>
                      <td><div className="mini-track"><i style={{ width: `${(c.requests / maxCountry) * 100}%` }} /></div><small className="mono">{total ? fmt((c.requests / total) * 100, 1) : "0.0"}%</small></td>
                      <td className="num mono">—</td>
                    </tr>
                  ))}
                </tbody>
              </table></div>
              <div className="panel-foot"><span style={{ minWidth: 0, whiteSpace: "normal" }}>Showing {Math.min(8, traffic?.countries.length ?? 0)} countries · addresses are not stored{traffic?.geo_configured && !showMap ? `. ${attribution}` : ""}</span></div>
            </Panel>
            <Panel className="span-12" title="Traffic authenticity" definition="Requests/s by classification." state={data?.classes.available ? "ready" : "unavailable"} stateTitle="Classification is not measured" stateText={data?.classes.reason} minHeight={220} meta={<SourceTag kind="unavailable" />} />
          </div>
        </>
      )}
      {tab === "bandwidth" && (
        <div className="grid g-12">
          <Panel className="span-8" title="Throughput" definition="Sum of interface rates from /proc/net/dev. Egress is transmit, ingress is receive." state={!data || state === "loading" ? "loading" : (net?.labels.length ?? 0) >= 2 ? "ready" : "empty"} stateTitle="Not enough samples yet" stateText="The collector records about every 15 seconds. Two buckets are needed before the line is drawn." minHeight={260}>
            {net && net.labels.length >= 2 && (
              <TimeSeries labels={net.labels} height={200} unit="Mbps" format={(v) => fmt(v, 0)}
                series={[{ key: "eg", label: "Egress", color: "var(--c-egress)", values: net.egress }, { key: "in", label: "Ingress", color: "var(--c-ingress)", values: net.ingress }]} />
            )}
          </Panel>
          <Panel className="span-4" title="Bandwidth by protocol" definition="Egress split by protocol." state="unavailable" stateTitle="Protocol split is not measured" stateText={data?.protocols.reason} minHeight={260} />
          <Panel className="span-5" title="Monthly egress forecast" definition="Cumulative egress against a configured plan." state="unavailable" stateTitle="No egress plan configured" stateText={data?.forecast.reason} minHeight={260} meta={<SourceTag kind="unavailable" />}>
            <Capacity value={0} max={1} warn={90} crit={100} />
          </Panel>
          <Panel className="span-7" title="Top URLs by bandwidth" definition="Normalised paths from the request rollup. Cache hit ratio is not measured." flush state={!data || state === "loading" ? "loading" : traffic && traffic.paths.length > 0 ? "ready" : "empty"} stateTitle="No paths in this range" minHeight={300}>
            <div className="table-wrap"><table className="table">
              <thead><tr><th>Path</th><th className="num">Requests</th><th className="num">Bytes</th><th className="num">Cache</th></tr></thead>
              <tbody>
                {traffic?.paths.map((u) => (
                  <tr key={u.path}>
                    <td><div className="cell-stack"><span className="mono strong truncate">{u.path}</span><small>{traffic.source}</small></div></td>
                    <td className="num mono">{compact(u.requests)}</td>
                    <td className="num mono">{compact(u.bytes)}</td>
                    <td className="num mono">—</td>
                  </tr>
                ))}
              </tbody>
            </table></div>
          </Panel>
        </div>
      )}
      {tab === "security" && (
        <div className="grid g-12">
          <Panel className="span-4" title="Security events" definition="Panel sign-in failures, lockouts, and recovery-code use. Client addresses are not stored." state={!data || state === "loading" ? "loading" : data.events.length ? "ready" : "empty"} stateTitle="No sign-in events" stateText="Failed passwords, authenticator failures, and lockouts appear here." flush minHeight={320} meta={securityCount ? <Badge tone="warning">{securityCount} recent</Badge> : undefined}>
            <ul className="event-list">
              {data?.events.map((e) => (
                <li key={e.at + e.title} className={`event sev-${e.sev}`}>
                  <span className="mono event-time">{e.at.slice(11, 19)}</span>
                  <div><b>{e.title}</b><p>{e.detail}</p><small>{e.loc}</small></div>
                </li>
              ))}
            </ul>
          </Panel>
          <Panel className="span-8" title="Top bandwidth consumers by IP" flush state="unavailable" stateTitle="Client addresses are not stored" stateText={data?.clients.reason} minHeight={300} />
        </div>
      )}
    </div>
  )
}
