import { useCallback, useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { ApiError, api } from "../lib/api"
import { useAuth } from "../lib/auth"
import type { AlertItem, ApmView, Bootstrap, EdgeView, HardwareView, Range } from "../lib/panel"
import { rememberSignIn } from "./auth-screen"
import { ApmPage } from "./pages/apm"
import { EdgePage } from "./pages/edge"
import { HardwarePage } from "./pages/hardware"
import { Shell, type Screen, type Theme } from "./shell"
import { Icon, Spinner } from "./ui"
import type { UpdateSettings } from "../lib/settings"

const IDLE_MS = 30 * 60 * 1000

export function Workspace({
  theme,
  setTheme,
  screen,
  setScreen,
}: {
  theme: Theme
  setTheme: (theme: Theme) => void
  screen: Screen
  setScreen: (screen: Screen) => void
}) {
  const { user, logout, refresh, clearLocalSession } = useAuth()
  const navigate = useNavigate()
  const [range, setRange] = useState<Range>("24h")
  const [live, setLive] = useState(true)
  const [boot, setBoot] = useState<Bootstrap | null>(null)
  const [edge, setEdge] = useState<EdgeView | null>(null)
  const [apm, setApm] = useState<ApmView | null>(null)
  const [hardware, setHardware] = useState<HardwareView | null>(null)
  const [alerts, setAlerts] = useState<AlertItem[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [tab, setTab] = useState<string | undefined>()
  const [nonce, setNonce] = useState(0)
  const [project, setProject] = useState("")
  const [accountOpen, setAccountOpen] = useState(false)

  const load = useCallback(async (opts?: { soft?: boolean }) => {
    if (!opts?.soft) setLoading(true)
    else setSyncing(true)
    try {
      const [nextBoot, nextAlerts] = await Promise.all([
        api<Bootstrap>("/api/panel/bootstrap"),
        api<{ alerts: AlertItem[] }>("/api/alerts"),
      ])
      setBoot(nextBoot)
      setAlerts(nextAlerts.alerts ?? [])
      if (nextBoot.user.last_sign_in_at) rememberSignIn(nextBoot.user.last_sign_in_at, nextBoot.user.last_sign_in_place)
      const query = `range=${range}${project ? `&project=${encodeURIComponent(project)}` : ""}`
      if (screen === "edge") setEdge(await api<EdgeView>(`/api/panel/edge?${query}`))
      if (screen === "apm") setApm(await api<ApmView>(`/api/panel/apm?${query}`))
      if (screen === "hardware") setHardware(await api<HardwareView>(`/api/panel/hardware?${query}`))
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearLocalSession()
        navigate("/login", { replace: true })
      }
    } finally {
      setLoading(false)
      setSyncing(false)
    }
  }, [clearLocalSession, navigate, project, range, screen])

  useEffect(() => { void load() }, [load])

  useEffect(() => {
    const liveCapable = range === "1h" || range === "6h" || range === "24h"
    if (!live || !liveCapable || screen === "updates") return
    const timer = window.setInterval(() => { void load({ soft: true }) }, 10000)
    return () => window.clearInterval(timer)
  }, [live, load, range, screen])

  useEffect(() => {
    let timer = window.setTimeout(() => { void logout().finally(() => navigate("/login?notice=expired", { replace: true })) }, IDLE_MS)
    const reset = () => { window.clearTimeout(timer); timer = window.setTimeout(() => { void logout().finally(() => navigate("/login?notice=expired", { replace: true })) }, IDLE_MS) }
    const events = ["mousedown", "keydown", "scroll", "touchstart"] as const
    events.forEach((ev) => window.addEventListener(ev, reset, { passive: true }))
    return () => { window.clearTimeout(timer); events.forEach((ev) => window.removeEventListener(ev, reset)) }
  }, [logout, navigate])

  useEffect(() => {
    let prefix = false
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.tagName === "SELECT" || t.isContentEditable || e.metaKey || e.ctrlKey) return
      if (e.key === "g") { prefix = true; window.setTimeout(() => { prefix = false }, 800); return }
      if (!prefix) return
      if (e.key === "e") setScreen("edge")
      if (e.key === "a") setScreen("apm")
      if (e.key === "h") setScreen("hardware")
      prefix = false
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [setScreen])

  const go = (next: Screen) => {
    setScreen(next)
    setTab(undefined)
    setNonce((n) => n + 1)
    window.scrollTo({ top: 0 })
  }

  if (!boot || !user) {
    return <div className="auth"><main className="auth-main"><div className="auth-card"><div className="auth-body"><div className="center-state"><Spinner /><p>Checking session</p></div></div></div></main></div>
  }

  const collected = screen === "edge" ? edge?.collected_at : screen === "apm" ? "" : hardware?.collected_at
  return (
    <>
      <Shell
        screen={screen}
        setScreen={go}
        theme={theme}
        setTheme={setTheme}
        host={boot.host}
        org={boot.org}
        version={boot.version}
        user={{ ...boot.user, totp_enabled: user.totp_enabled ?? boot.user.totp_enabled }}
        agentCpu={boot.agent.cpu_percent}
        range={range}
        setRange={(next) => { setRange(next); setNonce((n) => n + 1) }}
        live={live}
        setLive={setLive}
        loading={loading}
        collectedAt={collected || new Date().toISOString()}
        syncing={syncing}
        onRefresh={() => void load()}
        onSignOut={() => { void logout().finally(() => navigate("/login?notice=signedout", { replace: true })) }}
        onNavigate={(next, nextTab) => { setScreen(next); setTab(nextTab); setNonce((n) => n + 1); window.scrollTo({ top: 0 }) }}
        alerts={alerts}
        onReadAlerts={(ids) => {
          void api("/api/alerts/read", { method: "POST", body: JSON.stringify(ids === "all" ? { all: true } : { ids }) }).then(() => load({ soft: true }))
        }}
        onDismissAlert={(id) => { void api("/api/alerts/dismiss", { method: "POST", body: JSON.stringify({ id }) }).then(() => load({ soft: true })) }}
        onAccount={() => setAccountOpen(true)}
      >
        {screen === "edge" && <EdgePage key={nonce} data={edge} loading={loading} initialTab={tab} />}
        {screen === "apm" && <ApmPage key={nonce} data={apm} loading={loading} initialTab={tab} onSelect={(id) => { setProject(id); setNonce((n) => n + 1) }} />}
        {screen === "hardware" && <HardwarePage key={nonce} data={hardware} loading={loading} initialTab={tab} />}
        {screen === "updates" && <UpdatesPage />}
      </Shell>
      {accountOpen && <EnrollDialog enabled={!!(user.totp_enabled ?? boot.user.totp_enabled)} onClose={() => setAccountOpen(false)} onChanged={() => { void refresh(); setAccountOpen(false) }} />}
    </>
  )
}

function UpdatesPage() {
  const { user } = useAuth()
  const [settings, setSettings] = useState<UpdateSettings | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [pending, setPending] = useState<string | null>(null)

  useEffect(() => {
    if (!user?.is_admin) return
    api<UpdateSettings>("/api/settings").then(setSettings).catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Could not load settings."))
  }, [user?.is_admin])

  async function run(action: string, path: string, body: unknown, done: (next: UpdateSettings) => string) {
    setPending(action)
    setError(null)
    try {
      const next = await api<UpdateSettings>(path, { method: path === "/api/settings" ? "PATCH" : "POST", body: JSON.stringify(body) })
      setSettings(next)
      setNotice(done(next))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Request failed.")
    } finally {
      setPending(null)
    }
  }

  if (!user?.is_admin) return <div className="page"><PanelNote title="Updates" text="Update settings are limited to the panel admin." /></div>
  return (
    <div className="page">
      <div className="toolbar"><div className="toolbar-left"><h1>Updates</h1><span className="host-facts">Auto-update stays off until an admin enables it.</span></div></div>
      {error && <div className="notice notice-error"><Icon name="warning" /><p>{error}</p></div>}
      {notice && <div className="notice notice-ok"><Icon name="check" /><p>{notice}</p></div>}
      {settings && (
        <div className="grid g-12">
          <section className="panel span-12">
            <header className="panel-head"><h3>This install</h3></header>
            <div className="panel-body">
              <div className="kv-grid">
                <div className="kv"><span>Current</span><b className="mono">{settings.current_version || "Unknown"}</b></div>
                <div className="kv"><span>Latest</span><b className="mono">{settings.latest_available || "None yet"}</b></div>
                <div className="kv"><span>Repository</span><b className="mono">{settings.repository}</b></div>
              </div>
              <p className="note">{settings.apply_behavior}</p>
              <div className="row-actions">
                <button className="btn" disabled={pending !== null} onClick={() => void run("toggle", "/api/settings", { auto_update: !settings.auto_update }, (next) => next.auto_update ? "Auto-update is on." : "Auto-update is off.")}>{settings.auto_update ? "Turn auto-update off" : "Turn auto-update on"}</button>
                <button className="btn" disabled={pending !== null} onClick={() => void run("check", "/api/settings/updates/check", {}, (next) => next.update_available ? `${next.latest_available} is available.` : "This install is current.")}>{pending === "check" ? "Checking" : "Check now"}</button>
                <button className="btn" disabled={pending !== null || !settings.update_available} onClick={() => void run("apply", "/api/settings/updates/apply", {}, (next) => next.applied ? `Installed ${next.latest_available}.` : "No newer release was installed.")}>Apply</button>
                <button className="btn" disabled={pending !== null || !settings.rollback_available} onClick={() => void run("rollback", "/api/settings/updates/rollback", {}, () => "Restored the previous binary.")}>Rollback</button>
              </div>
            </div>
          </section>
        </div>
      )}
    </div>
  )
}

function PanelNote({ title, text }: { title: string; text: string }) {
  return <section className="panel"><header className="panel-head"><h3>{title}</h3></header><div className="panel-body"><p>{text}</p></div></section>
}

function EnrollDialog({ enabled, onClose, onChanged }: { enabled: boolean; onClose: () => void; onChanged: () => void }) {
  const [secret, setSecret] = useState("")
  const [qr, setQr] = useState("")
  const [code, setCode] = useState("")
  const [password, setPassword] = useState("")
  const [codes, setCodes] = useState<string[] | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (enabled) return
    api<{ secret: string; qr_png: string }>("/api/me/totp/setup", { method: "POST", body: "{}" })
      .then((data) => { setSecret(data.secret); setQr(data.qr_png) })
      .catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Could not start enrollment."))
  }, [enabled])

  return (
    <div className="dialog-backdrop" onMouseDown={onClose}>
      <div className="dialog" role="dialog" aria-modal="true" aria-labelledby="enroll-title" onMouseDown={(e) => e.stopPropagation()}>
        <div className="dialog-head">
          <span className="dialog-icon primary"><Icon name="shield" size={18} /></span>
          <h2 id="enroll-title">{enabled ? "Authenticator" : "Set up authenticator"}</h2>
          <button className="icon-btn ghost" aria-label="Close dialog" onClick={onClose}><Icon name="x" /></button>
        </div>
        <div className="dialog-body">
          {error && <div className="notice notice-error"><Icon name="warning" /><p>{error}</p></div>}
          {codes ? (
            <>
              <p>Save these recovery codes now. Each code works once and they are not shown again.</p>
              <ul className="consequences">{codes.map((item) => <li key={item}><span className="mono">{item}</span></li>)}</ul>
            </>
          ) : enabled ? (
            <>
              <p>Enter your password and a current authenticator or recovery code to turn 2FA off.</p>
              <label className="field"><span>Password</span><input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" /></label>
              <label className="field"><span>Code</span><input value={code} onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code" /></label>
            </>
          ) : (
            <>
              <p>Scan the code, or enter the secret manually, then confirm with a 6-digit code before 2FA is enabled.</p>
              {qr && <img alt="Authenticator QR code" src={`data:image/png;base64,${qr}`} width={180} height={180} />}
              {secret && <p className="mono">{secret}</p>}
              <label className="field"><span>6-digit code</span><input value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))} inputMode="numeric" autoComplete="one-time-code" /></label>
            </>
          )}
        </div>
        <div className="dialog-foot">
          <button className="btn" onClick={() => (codes ? onChanged() : onClose())}>{codes ? "Done" : "Close"}</button>
          {!codes && (enabled ? (
            <button className="btn btn-danger" disabled={busy || !password || !code} onClick={() => {
              setBusy(true)
              api("/api/me/totp/disable", { method: "POST", body: JSON.stringify({ password, code }) })
                .then(() => onChanged())
                .catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Could not disable 2FA."))
                .finally(() => setBusy(false))
            }}>{busy ? "Saving…" : "Turn off"}</button>
          ) : (
            <button className="btn btn-primary" disabled={busy || code.length < 6} onClick={() => {
              setBusy(true)
              api<{ recovery_codes?: string[] }>("/api/me/totp/confirm", { method: "POST", body: JSON.stringify({ code }) })
                .then((data) => setCodes(data.recovery_codes ?? []))
                .catch((err: unknown) => setError(err instanceof ApiError ? err.message : "That code did not match."))
                .finally(() => setBusy(false))
            }}>{busy ? "Checking…" : "Enable"}</button>
          ))}
        </div>
      </div>
    </div>
  )
}
