import { useEffect } from "react"
import { DashboardLayout } from "../components/dashboard-layout"
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
    <DashboardLayout>
      <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">01 / Control plane</p>
      <h1 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">Signed in.</h1>
      <p className="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
        This shell is the v0.1.0 control plane for{" "}
        <span className="break-all text-ink">{user.email}</span>.
        {user.is_admin
          ? " This account is the panel admin, so public registration is closed."
          : null}{" "}
        Servers, mail, and true-state telemetry are later releases. The account and the session are
        here.
      </p>
      <dl className="mt-8 grid gap-px border border-line bg-line sm:grid-cols-3">
        <Stat label="Session" value="Live" live />
        <Stat label="Account" value={user.email} />
        <Stat label="Joined" value={formatJoined(user.created_at)} />
      </dl>
      <section className="corner mt-8 border border-line bg-card p-6">
        <p className="font-mono text-[11px] tracking-[0.16em] text-muted uppercase">Nothing queued</p>
        <h2 className="mt-3 text-xl font-semibold tracking-tight">No hosts are attached.</h2>
        <p className="mt-2 max-w-xl text-sm leading-relaxed text-muted">
          When the control plane grows, machines will show up in this room. v0.1.0 stops at the
          signed-in shell so the panel does not pretend to manage a server it cannot see.
        </p>
      </section>
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
