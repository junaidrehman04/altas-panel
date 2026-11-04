import { useEffect } from "react"
import { DashboardLayout } from "../components/dashboard-layout"
import { HostMetrics } from "../components/host-metrics"
import { useAuth } from "../lib/auth"

function formatJoined(iso: string) {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date)
}

export function DashboardPage() {
  const { user } = useAuth()

  useEffect(() => {
    document.title = "Overview · Altas Panel"
  }, [])

  if (!user) return null

  return (
    <DashboardLayout section="Overview">
      <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">01 / Control plane</p>
      <h1 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">This host.</h1>
      <p className="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
        Signed in as <span className="break-all text-ink">{user.email}</span>.
        {user.is_admin
          ? " This account is the panel admin, so public registration is closed."
          : null}{" "}
        The numbers below are read from this machine. Other servers, mail, and deploy actions are
        later releases.
      </p>
      <dl className="mt-8 grid gap-px border border-line bg-line sm:grid-cols-3">
        <Stat label="Session" value="Live" live />
        <Stat label="Account" value={user.email} />
        <Stat label="Joined" value={formatJoined(user.created_at)} />
      </dl>
      <HostMetrics />
    </DashboardLayout>
  )
}

function Stat({ label, value, live = false }: { label: string; value: string; live?: boolean }) {
  return (
    <div className="bg-card px-4 py-4">
      <dt className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{label}</dt>
      <dd className="mt-2 flex items-center gap-2 font-mono text-sm break-all text-ink">
        {live ? <span className="inline-block h-1.5 w-1.5 bg-live" aria-hidden="true" /> : null}
        {value}
      </dd>
    </div>
  )
}
