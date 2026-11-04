import type { ReactNode } from "react"
import { Link } from "react-router-dom"
import { Brand } from "./mark"
import { ThemeToggle } from "./theme-toggle"

export function AuthLayout({
  title,
  lede,
  children,
  footer,
}: {
  title: string
  lede: string
  children: ReactNode
  footer: ReactNode
}) {
  return (
    <div className="relative min-h-svh bg-base text-ink">
      <div className="hud-grid pointer-events-none fixed inset-0" aria-hidden="true" />
      <header className="relative flex items-center justify-between px-4 py-4 sm:px-8">
        <Link to="/login" className="text-ink" aria-label="Altas Panel">
          <Brand />
        </Link>
        <ThemeToggle />
      </header>
      <main className="relative mx-auto grid w-full max-w-6xl items-start gap-8 px-4 pb-16 sm:px-8 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)] lg:pt-6">
        <section className="corner hidden border border-line bg-card/80 p-8 lg:block">
          <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">01 / Session</p>
          <p className="mt-4 max-w-md text-4xl font-semibold tracking-tight text-ink">
            One control plane.
          </p>
          <p className="mt-4 max-w-md text-[15px] leading-relaxed text-muted">
            Altas is the panel that reads true server state from{" "}
            <span className="font-mono text-[13px] text-ink">/proc</span>, so a small VPS keeps its
            memory for the workloads on it. v0.1.0 is the door into that plane: the first account
            becomes the admin, then public registration closes.
          </p>
          <dl className="mt-8 grid grid-cols-3 divide-x divide-line border border-line">
            <Fact label="Source" value="/proc" />
            <Fact label="Target" value="<18MB" />
            <Fact label="Release" value="v0.1.0" />
          </dl>
        </section>
        <section className="corner border border-line bg-card p-6 sm:p-8">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          <p className="mt-2 text-sm leading-relaxed text-muted">{lede}</p>
          <div className="mt-6">{children}</div>
          <div className="mt-6 border-t border-line pt-4 text-sm text-muted">{footer}</div>
        </section>
      </main>
    </div>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="px-3 py-3">
      <dt className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{label}</dt>
      <dd className="mt-1 font-mono text-sm text-ink">{value}</dd>
    </div>
  )
}
