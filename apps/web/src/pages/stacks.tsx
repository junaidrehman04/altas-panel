import { useCallback, useEffect, useRef, useState } from "react"
import { DashboardLayout } from "../components/dashboard-layout"
import { Button } from "../components/ui/button"
import { api, ApiError } from "../lib/api"
import type { ComposeReport, StackCatalog } from "../lib/stacks"

export function StacksPage() {
  const [catalog, setCatalog] = useState<StackCatalog | null>(null)
  const [compose, setCompose] = useState<ComposeReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const [nextCatalog, nextCompose] = await Promise.all([
        api<StackCatalog>("/api/stacks"),
        api<ComposeReport>("/api/stacks/compose"),
      ])
      setCatalog(nextCatalog)
      setCompose(nextCompose)
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load stacks.")
    }
  }, [])

  const alive = useRef(true)

  useEffect(() => {
    document.title = "Stacks · Altas Panel"
    alive.current = true
    async function tick() {
      try {
        const [nextCatalog, nextCompose] = await Promise.all([
          api<StackCatalog>("/api/stacks"),
          api<ComposeReport>("/api/stacks/compose"),
        ])
        if (!alive.current) return
        setCatalog(nextCatalog)
        setCompose(nextCompose)
        setError(null)
      } catch (err) {
        if (!alive.current) return
        setError(err instanceof ApiError ? err.message : "Could not load stacks.")
      }
    }
    void tick()
    return () => {
      alive.current = false
    }
  }, [])

  return (
    <DashboardLayout section="Stacks">
      <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">02 / Stacks</p>
      <h1 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">Registry</h1>
      <p className="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
        Docker Compose, Laravel, and Next.js are known kinds. Creating, starting, and stopping
        projects is a later release. The list below is the registry, not a running fleet.
      </p>
      {error ? (
        <p role="alert" className="mt-6 border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </p>
      ) : null}
      {catalog ? (
        <>
          <p className="mt-6 font-mono text-[11px] text-muted">
            Project jail <span className="text-ink">{catalog.projects_root}</span>
          </p>
          <ul className="mt-4 grid gap-px border border-line bg-line lg:grid-cols-3">
            {catalog.stacks.map((stack) => (
              <li key={stack.kind} className="bg-card px-4 py-4">
                <p className="font-mono text-[10px] tracking-[0.14em] text-muted uppercase">{stack.kind}</p>
                <h2 className="mt-2 text-lg font-semibold tracking-tight">{stack.title}</h2>
                <p className="mt-2 text-sm leading-relaxed text-muted">{stack.summary}</p>
                <p className="mt-3 font-mono text-[10px] tracking-[0.14em] text-signal uppercase">
                  {stack.deployable ? "Deployable" : "Not deployable yet"}
                </p>
              </li>
            ))}
          </ul>
        </>
      ) : null}
      <section className="corner mt-8 border border-line bg-card p-6" aria-labelledby="compose-heading">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="compose-heading" className="text-xl font-semibold tracking-tight">
            Local Compose projects
          </h2>
          <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
            Refresh
          </Button>
        </div>
        {compose?.message ? <p className="mt-3 max-w-2xl text-sm leading-relaxed text-muted">{compose.message}</p> : null}
        {compose && compose.projects.length > 0 ? (
          <ul className="mt-4 grid gap-3">
            {compose.projects.map((project) => (
              <li key={project.name} className="border border-line px-4 py-3">
                <p className="font-mono text-sm text-ink">{project.name}</p>
                <p className="mt-1 font-mono text-[11px] text-muted">
                  {project.status} · {project.running}/{project.containers} running
                  {project.workdir ? ` · ${project.workdir}` : ""}
                </p>
              </li>
            ))}
          </ul>
        ) : null}
        {compose && compose.projects.length === 0 ? (
          <p className="mt-4 text-sm text-muted">No Compose projects are listed.</p>
        ) : null}
      </section>
    </DashboardLayout>
  )
}
