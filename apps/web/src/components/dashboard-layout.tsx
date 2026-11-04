import { useEffect, useState, type ReactNode } from "react"
import { NavLink } from "react-router-dom"
import { useAuth } from "../lib/auth"
import { cn } from "../lib/utils"
import { Brand } from "./mark"
import { ThemeToggle } from "./theme-toggle"
import { Button } from "./ui/button"

function useDesktop() {
  const [desktop, setDesktop] = useState(() => window.matchMedia("(min-width: 1024px)").matches)

  useEffect(() => {
    const query = window.matchMedia("(min-width: 1024px)")
    const onChange = () => setDesktop(query.matches)
    query.addEventListener("change", onChange)
    return () => query.removeEventListener("change", onChange)
  }, [])

  return desktop
}

export function DashboardLayout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth()
  const desktop = useDesktop()
  const [open, setOpen] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  useEffect(() => {
    if (!open) return
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [open])

  async function onLogout() {
    setError(null)
    setPending(true)
    try {
      await logout()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not sign out.")
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="min-h-svh bg-base text-ink">
      {open ? (
        <button
          type="button"
          className="fixed inset-0 z-30 bg-ink/40 lg:hidden"
          aria-label="Close menu"
          onClick={() => setOpen(false)}
        />
      ) : null}
      <aside
        id="panel-sidebar"
        inert={!desktop && !open ? true : undefined}
        className={cn(
          "fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-line bg-card transition-transform lg:translate-x-0",
          open ? "translate-x-0" : "-translate-x-full",
        )}
      >
        <div className="flex items-center border-b border-line px-4 py-4">
          <Brand compact />
        </div>
        <div className="px-3 py-4">
          <p className="px-2 font-mono text-[10px] tracking-[0.16em] text-muted uppercase">Panel</p>
          <NavLink
            to="/dashboard"
            onClick={() => setOpen(false)}
            className={({ isActive }) =>
              cn(
                "mt-2 flex items-center border-l-2 px-3 py-2 text-sm",
                isActive
                  ? "border-accent bg-base text-ink"
                  : "border-transparent text-muted hover:text-ink",
              )
            }
          >
            Overview
          </NavLink>
        </div>
        <div className="mt-auto border-t border-line px-4 py-4">
          <p className="truncate font-mono text-xs text-ink">{user?.email}</p>
          <p className="mt-1 font-mono text-[10px] tracking-[0.14em] text-muted uppercase">
            {user?.is_admin ? "Admin · MIT · v0.1.0" : "MIT · v0.1.0"}
          </p>
        </div>
      </aside>
      <div className="lg:pl-64">
        <header className="sticky top-0 z-20 flex h-14 items-center gap-3 border-b border-line bg-base/90 px-4 backdrop-blur">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="lg:hidden"
            aria-expanded={open}
            aria-controls="panel-sidebar"
            onClick={() => setOpen((current) => !current)}
          >
            Menu
          </Button>
          <p className="font-mono text-[11px] tracking-[0.16em] text-muted uppercase">Overview</p>
          <div className="ml-auto flex items-center gap-2">
            <ThemeToggle />
            <Button type="button" variant="outline" size="sm" onClick={onLogout} disabled={pending}>
              {pending ? "Signing out" : "Log out"}
            </Button>
          </div>
        </header>
        {error ? (
          <p role="alert" className="border-b border-danger/40 bg-danger/10 px-4 py-2 text-sm text-danger">
            {error}
          </p>
        ) : null}
        <main className="px-4 py-8 sm:px-8">{children}</main>
      </div>
    </div>
  )
}
