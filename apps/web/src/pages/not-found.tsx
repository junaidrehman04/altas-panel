import { useEffect } from "react"
import { Link } from "react-router-dom"
import { Brand } from "../components/mark"
import { ThemeToggle } from "../components/theme-toggle"
import { useAuth } from "../lib/auth"

export function NotFoundPage() {
  const { user, loading } = useAuth()

  useEffect(() => {
    document.title = "Not found · Altas Panel"
  }, [])

  return (
    <div className="relative min-h-svh bg-base text-ink">
      <div className="hud-grid pointer-events-none fixed inset-0" aria-hidden="true" />
      <header className="relative flex items-center justify-between px-4 py-4 sm:px-8">
        <Brand />
        <ThemeToggle />
      </header>
      <main className="relative mx-auto max-w-lg px-4 py-16">
        <p className="font-mono text-[11px] tracking-[0.18em] text-signal uppercase">404</p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight">That page is not on this panel.</h1>
        <p className="mt-3 text-sm text-muted">The address does not match a screen in this panel.</p>
        <Link to={loading ? "/login" : user ? "/dashboard" : "/login"} className="mt-6 inline-block text-sm text-accent">
          {user ? "Back to overview" : "Back to sign in"}
        </Link>
      </main>
    </div>
  )
}
