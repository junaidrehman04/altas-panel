import { useCallback, useEffect, useRef, useState } from "react"
import { DashboardLayout } from "../components/dashboard-layout"
import { Button } from "../components/ui/button"
import { useAuth } from "../lib/auth"
import { api, ApiError } from "../lib/api"
import type { UpdateSettings } from "../lib/settings"

function formatWhen(iso: string | null) {
  if (!iso) return "Not checked yet"
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date)
}

function formatInterval(seconds: number) {
  if (seconds > 0 && seconds % 3600 === 0) {
    const hours = seconds / 3600
    return hours === 1 ? "every hour" : `every ${hours} hours`
  }
  if (seconds > 0) return `every ${seconds} seconds`
  return "on the saved interval"
}

function verificationLabel(mode: string) {
  if (mode === "sha256+ed25519") return "SHA256 checksum and Ed25519 signature"
  return "SHA256 checksum from the release"
}

export function SettingsPage() {
  const { user } = useAuth()
  const [settings, setSettings] = useState<UpdateSettings | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [pending, setPending] = useState<string | null>(null)

  const load = useCallback(async () => {
    const next = await api<UpdateSettings>("/api/settings")
    setSettings(next)
    setError(null)
  }, [])

  const alive = useRef(true)

  useEffect(() => {
    document.title = "Settings · Altas Panel"
    if (!user?.is_admin) return
    alive.current = true
    api<UpdateSettings>("/api/settings")
      .then((next) => {
        if (!alive.current) return
        setSettings(next)
        setError(null)
      })
      .catch((err: unknown) => {
        if (!alive.current) return
        setError(err instanceof ApiError ? err.message : "Could not load settings.")
      })
    return () => {
      alive.current = false
    }
  }, [user?.is_admin])

  async function run(action: string, task: () => Promise<UpdateSettings>, done: (next: UpdateSettings) => string) {
    setPending(action)
    setError(null)
    setNotice(null)
    try {
      const next = await task()
      setSettings(next)
      setNotice(done(next))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Request failed.")
      try {
        await load()
      } catch {
        // The action error is already shown.
      }
    } finally {
      setPending(null)
    }
  }

  if (!user?.is_admin) {
    return (
      <DashboardLayout section="Settings">
        <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">03 / Settings</p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">Updates</h1>
        <p className="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
          Update settings are limited to the panel admin.
        </p>
      </DashboardLayout>
    )
  }

  return (
    <DashboardLayout section="Settings">
      <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">03 / Settings</p>
      <h1 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">Updates</h1>
      <p className="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
        Auto-update is off until you turn it on. Checks use the GitHub Releases API for{" "}
        <span className="font-mono text-ink">{settings?.repository ?? "junaidrehman04/altas-panel"}</span>.
        A download is installed only after its checksum matches the release.
      </p>

      {error ? (
        <p role="alert" className="mt-6 border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </p>
      ) : null}
      {notice ? (
        <p role="status" className="mt-6 border border-line bg-card px-4 py-3 text-sm text-ink">
          {notice}
        </p>
      ) : null}

      {!settings && !error ? (
        <p className="mt-8 font-mono text-[11px] tracking-[0.16em] text-muted uppercase">Loading settings</p>
      ) : null}

      {settings ? (
        <div className="mt-8 grid max-w-3xl gap-4">
          <section className="corner border border-line bg-card p-6" aria-labelledby="release-heading">
            <h2 id="release-heading" className="text-xl font-semibold tracking-tight">
              This install
            </h2>
            <dl className="mt-4 grid gap-px border border-line bg-line sm:grid-cols-3">
              <Stat label="Current version" value={settings.current_version || "Unknown"} />
              <Stat label="Latest release" value={settings.latest_available || "None yet"} />
              <Stat label="Last check" value={formatWhen(settings.last_check)} />
            </dl>
            <p className="mt-4 text-sm leading-relaxed text-muted">
              Verification is {verificationLabel(settings.verification)}. Scheduled checks run{" "}
              {formatInterval(settings.check_interval_seconds)}. Check now is limited to once a minute.
            </p>
            {settings.update_available ? (
              <p className="mt-3 font-mono text-[10px] tracking-[0.14em] text-signal uppercase">
                Update available
              </p>
            ) : null}
            {settings.last_error ? (
              <p className="mt-3 text-sm text-danger">{settings.last_error}</p>
            ) : null}
          </section>

          <section className="border border-line bg-card p-6" aria-labelledby="toggle-heading">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <h2 id="toggle-heading" className="text-xl font-semibold tracking-tight">
                  Auto-update
                </h2>
                <p className="mt-2 max-w-xl text-sm leading-relaxed text-muted">{settings.apply_behavior}</p>
              </div>
              <Button
                type="button"
                variant={settings.auto_update ? "default" : "outline"}
                aria-pressed={settings.auto_update}
                disabled={pending !== null}
                onClick={() =>
                  void run(
                    "toggle",
                    () =>
                      api<UpdateSettings>("/api/settings", {
                        method: "PATCH",
                        body: JSON.stringify({ auto_update: !settings.auto_update }),
                      }),
                    (next) => (next.auto_update ? "Auto-update is on." : "Auto-update is off."),
                  )
                }
              >
                {pending === "toggle" ? "Saving" : settings.auto_update ? "On" : "Off"}
              </Button>
            </div>
          </section>

          <section className="border border-line bg-card p-6" aria-labelledby="actions-heading">
            <h2 id="actions-heading" className="text-xl font-semibold tracking-tight">
              Actions
            </h2>
            <p className="mt-2 max-w-xl text-sm leading-relaxed text-muted">
              {settings.restart_mode === "systemd"
                ? `A successful install restarts the ${settings.unit} service.`
                : "This process will not restart itself. Restart it after an install."}{" "}
              Rollback restores the previous binary kept beside the install.
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              <Button
                type="button"
                disabled={pending !== null}
                onClick={() =>
                  void run(
                    "check",
                    () => api<UpdateSettings>("/api/settings/updates/check", { method: "POST", body: "{}" }),
                    (next) => {
                      if (next.applied) return `Installed ${next.latest_available}.`
                      if (next.update_available) return `${next.latest_available} is available. The binary was not replaced.`
                      return "This install is current."
                    },
                  )
                }
              >
                {pending === "check" ? "Checking" : "Check now"}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={pending !== null || !settings.update_available}
                onClick={() =>
                  void run(
                    "apply",
                    () => api<UpdateSettings>("/api/settings/updates/apply", { method: "POST", body: "{}" }),
                    (next) => (next.applied ? `Installed ${next.latest_available}.` : "No newer release was installed."),
                  )
                }
              >
                {pending === "apply" ? "Installing" : "Apply"}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={pending !== null || !settings.rollback_available}
                onClick={() =>
                  void run(
                    "rollback",
                    () => api<UpdateSettings>("/api/settings/updates/rollback", { method: "POST", body: "{}" }),
                    () => "Restored the previous binary.",
                  )
                }
              >
                {pending === "rollback" ? "Restoring" : "Rollback"}
              </Button>
            </div>
            {settings.rollback_available ? null : (
              <p className="mt-3 text-sm text-muted">Rollback stays unavailable until an update has saved a previous binary.</p>
            )}
          </section>
        </div>
      ) : null}
    </DashboardLayout>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-card px-4 py-4">
      <dt className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{label}</dt>
      <dd className="mt-2 font-mono text-sm break-all text-ink">{value}</dd>
    </div>
  )
}
