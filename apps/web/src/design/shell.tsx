import { useEffect, useMemo, useState, type ReactNode } from "react"
import { RANGE_OPTIONS, type Range } from "./data"
import { NotificationsMenu, type Alert } from "./notifications"
import { CommandPalette, type Command } from "./palette"
import { Dot, Icon, Segmented, Spinner, Wordmark, type IconName } from "./ui"
import type { HostCard, PanelUser } from "../lib/panel"

export type Screen = "edge" | "apm" | "hardware" | "updates"
export type Theme = "light" | "dark"

const NAV: { id: Exclude<Screen, "updates">; label: string; icon: IconName; hint: string }[] = [
  { id: "edge", label: "Edge Traffic", icon: "globe", hint: "G E" },
  { id: "apm", label: "Projects & APM", icon: "activity", hint: "G A" },
  { id: "hardware", label: "Hardware & Kernel", icon: "cpu", hint: "G H" },
]

export const SCREEN_TITLE: Record<Screen, string> = {
  edge: "Edge Traffic & Geo-IP Intelligence",
  apm: "Hosted Projects & Client APM",
  hardware: "Hardware, Kernel & Provider Diagnostics",
  updates: "Updates",
}

export function ThemeSwitch({ theme, setTheme }: { theme: Theme; setTheme: (t: Theme) => void }) {
  return (
    <div className="theme-switch" role="radiogroup" aria-label="Colour theme">
      <button role="radio" aria-checked={theme === "light"} className={theme === "light" ? "on" : ""} onClick={() => setTheme("light")} title="Light theme"><Icon name="sun" size={14} /></button>
      <button role="radio" aria-checked={theme === "dark"} className={theme === "dark" ? "on" : ""} onClick={() => setTheme("dark")} title="Dark theme"><Icon name="moon" size={14} /></button>
    </div>
  )
}

const SECTIONS: { screen: Exclude<Screen, "updates">; tab: string; label: string; keywords?: string }[] = [
  { screen: "edge", tab: "bandwidth", label: "Edge · Bandwidth", keywords: "egress traffic" },
  { screen: "edge", tab: "security", label: "Edge · Security", keywords: "auth sign-in" },
  { screen: "apm", tab: "performance", label: "APM · Performance", keywords: "latency p95 endpoints" },
  { screen: "apm", tab: "data", label: "APM · Code & data", keywords: "paths cache sql" },
  { screen: "apm", tab: "jobs", label: "APM · Jobs & mail", keywords: "cron mail" },
  { screen: "hardware", tab: "storage", label: "Hardware · Storage & sensors", keywords: "disk raid temperature inode" },
  { screen: "hardware", tab: "kernel", label: "Hardware · Kernel & security", keywords: "ports firewall ssh" },
]

const toneOf = (status: string) => (status === "ok" ? "ok" : status === "warning" ? "warning" : status === "critical" ? "critical" : "muted")

function clock(iso: string) {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ""
  return date.toTimeString().slice(0, 8)
}

export function Shell({
  screen, setScreen, theme, setTheme, host, org, version, user, agentCpu, range, setRange, live, setLive, loading, collectedAt, syncing, onRefresh, onSignOut, onNavigate, alerts, onReadAlerts, onDismissAlert, onAccount, children,
}: {
  screen: Screen
  setScreen: (s: Screen) => void
  theme: Theme
  setTheme: (t: Theme) => void
  host: HostCard
  org: string
  version: string
  user: PanelUser
  agentCpu: number | null
  range: Range
  setRange: (r: Range) => void
  live: boolean
  setLive: (v: boolean) => void
  loading: boolean
  collectedAt: string
  syncing: boolean
  onRefresh: () => void
  onSignOut: () => void
  onNavigate: (s: Exclude<Screen, "updates">, tab?: string) => void
  alerts: Alert[]
  onReadAlerts: (ids: string[] | "all") => void
  onDismissAlert: (id: string) => void
  onAccount: () => void
  children: ReactNode
}) {
  const liveCapable = range === "1h" || range === "6h" || range === "24h"
  const isLive = live && liveCapable
  const [hostOpen, setHostOpen] = useState(false)
  const [navOpen, setNavOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const pick = (fn: () => void) => { fn(); setNavOpen(false) }

  useEffect(() => {
    if (!hostOpen) return
    const onDoc = (e: MouseEvent) => { if (!(e.target as HTMLElement).closest(".host-picker")) setHostOpen(false) }
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setHostOpen(false) }
    document.addEventListener("mousedown", onDoc)
    window.addEventListener("keydown", onKey)
    return () => { document.removeEventListener("mousedown", onDoc); window.removeEventListener("keydown", onKey) }
  }, [hostOpen])

  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      const typing = t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.tagName === "SELECT" || t.isContentEditable
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setPaletteOpen((v) => !v); return }
      if (e.key === "/" && !typing) { e.preventDefault(); setPaletteOpen(true) }
    }
    window.addEventListener("keydown", h)
    return () => window.removeEventListener("keydown", h)
  }, [])

  const commands = useMemo<Command[]>(() => [
    ...NAV.map((n) => ({ id: `go-${n.id}`, group: "Pages", icon: n.icon, label: SCREEN_TITLE[n.id], hint: n.hint, run: () => setScreen(n.id) })),
    ...SECTIONS.map((x) => ({ id: `sec-${x.screen}-${x.tab}`, group: "Sections", icon: "layers" as IconName, label: x.label, keywords: x.keywords, run: () => onNavigate(x.screen, x.tab) })),
    { id: `host-${host.id}`, group: "Servers", icon: "server" as IconName, label: `Switch to ${host.id}`, keywords: `${host.fqdn} ${host.ip} ${host.provider}`, hint: "current", run: () => undefined },
    ...RANGE_OPTIONS.map((r) => ({ id: `range-${r.value}`, group: "Time range", icon: "clock" as IconName, label: `Show last ${r.label}`, keywords: "range period", hint: r.value === range ? "current" : undefined, run: () => setRange(r.value) })),
    { id: "live", group: "Actions", icon: (live ? "pause" : "play") as IconName, label: live ? "Pause live updates" : "Resume live updates", run: () => setLive(!live) },
    { id: "theme", group: "Actions", icon: (theme === "dark" ? "sun" : "moon") as IconName, label: `Switch to ${theme === "dark" ? "light" : "dark"} theme`, run: () => setTheme(theme === "dark" ? "light" : "dark") },
    { id: "read", group: "Actions", icon: "bell" as IconName, label: "Mark all notifications as read", run: () => onReadAlerts("all") },
    { id: "account", group: "Actions", icon: "shield" as IconName, label: user.totp_enabled ? "Authenticator settings" : "Set up authenticator", run: onAccount },
    { id: "updates", group: "Actions", icon: "refresh" as IconName, label: "Updates", keywords: "auto-update release", run: () => setScreen("updates") },
    { id: "signout", group: "Actions", icon: "logout" as IconName, label: "Sign out", run: onSignOut },
  ], [host, range, live, theme, setScreen, setRange, setLive, setTheme, onNavigate, onReadAlerts, onSignOut, onAccount, user.totp_enabled])

  const versionLabel = version.startsWith("v") ? version : `v${version}`

  return (
    <div className={`shell ${navOpen ? "nav-open" : ""}`} aria-busy={loading}>
      {loading && <div className="top-progress" role="progressbar" aria-label="Loading data"><i /></div>}
      <a href="#main" className="skip-link">Skip to content</a>
      <div className="nav-scrim" onClick={() => setNavOpen(false)} aria-hidden="true" />
      <aside className="sidebar" aria-label="Primary">
        <div className="side-brand"><Wordmark /><span className="version mono">{versionLabel}</span>
          <button className="icon-btn ghost nav-close" aria-label="Close navigation" onClick={() => setNavOpen(false)}><Icon name="x" /></button>
        </div>
        <div className="side-section">
          <div className="side-label">Monitoring</div>
          {NAV.map((n) => (
            <button key={n.id} className={`nav-item ${screen === n.id ? "active" : ""}`} onClick={() => pick(() => setScreen(n.id))} aria-current={screen === n.id ? "page" : undefined}>
              <Icon name={n.icon} /><span>{n.label}</span><kbd>{n.hint}</kbd>
            </button>
          ))}
        </div>
        <div className="side-section grow">
          <div className="side-label">Servers <span className="mono">1</span></div>
          <div className="tree-group"><Icon name="layers" size={14} />{org}</div>
          <button className="tree-item active" onClick={() => pick(() => undefined)}>
            <Dot tone={toneOf(host.status)} />
            <span className="mono">{host.id}</span>
            <small>{host.virt.includes("KVM") ? "KVM" : host.kind === "vps" ? "VM" : host.kind === "dedicated" ? "Metal" : "Host"}</small>
          </button>
        </div>
        <div className="side-foot">
          <div className="agent-line"><Dot tone="ok" /><span>Agent connected</span><span className="mono">{agentCpu == null ? "—" : `${agentCpu.toFixed(1)}% CPU`}</span></div>
          <div className="user-line">
            <button className="avatar" onClick={onAccount} title="Account security" aria-label="Account security">{user.initials || "A"}</button>
            <div>
              <button className="user-id" onClick={onAccount}>
                <b>{user.label}</b>
                <small>{user.is_admin ? "Administrator" : "User"} · {user.totp_enabled ? "2FA on" : "2FA off"}</small>
              </button>
            </div>
            <button className="icon-btn ghost" onClick={onSignOut} title="Sign out" aria-label="Sign out"><Icon name="logout" /></button>
          </div>
        </div>
      </aside>
      <div className="main-col">
        <header className="topbar">
          <button className="icon-btn nav-toggle" aria-label="Open navigation" aria-expanded={navOpen} onClick={() => setNavOpen(true)}><Icon name="menu" /></button>
          <nav className="crumbs" aria-label="Breadcrumb">
            <span className="crumb-org">{org}</span><Icon name="chevron" size={12} className="crumb-org" />
            <div className="host-picker">
              <button className="crumb-host" onClick={() => setHostOpen((v) => !v)} aria-haspopup="listbox" aria-expanded={hostOpen}>
                <Dot tone={toneOf(host.status)} /><span className="mono">{host.id}</span><Icon name="down" size={12} />
              </button>
              {hostOpen && (
                <div className="menu" role="listbox">
                  <button role="option" aria-selected onClick={() => setHostOpen(false)}>
                    <Dot tone={toneOf(host.status)} /><span className="mono">{host.id}</span><small>{host.provider || host.virt}</small><Icon name="check" size={14} />
                  </button>
                </div>
              )}
            </div>
            <Icon name="chevron" size={12} className="crumb-page" /><b className="crumb-page">{SCREEN_TITLE[screen]}</b>
          </nav>
          <div className="top-tools">
            <button className="search-btn" onClick={() => setPaletteOpen(true)} aria-label="Search and run commands" aria-keyshortcuts="Control+K /"><Icon name="search" size={14} /><span>Search or jump to…</span><kbd>{navigator.platform.includes("Mac") ? "⌘K" : "Ctrl K"}</kbd></button>
            <NotificationsMenu items={alerts} onRead={onReadAlerts} onDismiss={onDismissAlert} onOpen={(a) => { if (a.screen === "edge" || a.screen === "apm" || a.screen === "hardware") onNavigate(a.screen, a.tab) }} />
            <ThemeSwitch theme={theme} setTheme={setTheme} />
          </div>
        </header>
        {screen !== "updates" && (
          <div className="toolbar">
            <div className="toolbar-left">
              <h1>{SCREEN_TITLE[screen]}</h1>
              <span className="host-facts mono">{host.fqdn} · {host.ip || "address unavailable"} · {host.virt}</span>
            </div>
            <div className="toolbar-right">
              <div className="live-group">
                <span className={`data-mode ${isLive ? "is-live" : ""}`}>
                  {loading ? <><Spinner size={12} />Loading<span className="dm-detail"> · fetching metrics</span></> : syncing && isLive ? <><Spinner size={12} />Syncing<span className="dm-detail"> · polling agent</span></> : isLive ? <><Dot tone="ok" />Live<span className="dm-detail"> · updated {clock(collectedAt)}</span></> : <><Icon name="clock" size={13} />Historical<span className="dm-detail"> · {liveCapable ? "paused" : "1 h rollups"}</span></>}
                </span>
                <button className="btn btn-sm" onClick={() => setLive(!live)} disabled={!liveCapable} title={liveCapable ? undefined : "Live mode available up to 24h ranges"}>
                  <Icon name={isLive ? "pause" : "play"} size={13} />{isLive ? "Pause" : "Resume"}
                </button>
              </div>
              <button className="icon-btn refresh-btn" onClick={onRefresh} disabled={loading} aria-label="Refresh data" title="Refresh data">
                <Icon name="refresh" size={14} className={loading ? "spin" : ""} />
              </button>
              <Segmented label="Time range" value={range} options={RANGE_OPTIONS} onChange={setRange} />
            </div>
          </div>
        )}
        <main id="main" className="content">{children}</main>
      </div>
      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} commands={commands} />
    </div>
  )
}
