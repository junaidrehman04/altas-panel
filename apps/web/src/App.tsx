import { useEffect, useState } from "react"
import { Navigate, Route, Routes, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { AuthScreen, type AuthNotice } from "./design/auth-screen"
import { type Screen, type Theme } from "./design/shell"
import { Workspace } from "./design/workspace"
import { useAuth } from "./lib/auth"

function Boot() {
  return (
    <div className="auth">
      <main className="auth-main"><p className="auth-meta">Checking session</p></main>
    </div>
  )
}

function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem("altas-theme")
    if (saved === "light" || saved === "dark") return saved
    return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark"
  })
  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem("altas-theme", theme)
  }, [theme])
  return [theme, setTheme] as const
}

function Guest() {
  const { user, loading } = useAuth()
  const [theme, setTheme] = useTheme()
  const [params] = useSearchParams()
  const notice = (params.get("notice") === "expired" || params.get("notice") === "signedout" ? params.get("notice") : null) as AuthNotice
  const location = useLocation()
  if (loading) return <Boot />
  if (user) return <Navigate to="/edge" replace />
  return <AuthScreen key={location.pathname} theme={theme} setTheme={setTheme} notice={notice} />
}

function SignedIn() {
  const { user, loading } = useAuth()
  const [theme, setTheme] = useTheme()
  const location = useLocation()
  const navigate = useNavigate()
  const screen: Screen = location.pathname.startsWith("/apm") ? "apm" : location.pathname.startsWith("/hardware") ? "hardware" : location.pathname.startsWith("/updates") ? "updates" : "edge"
  if (loading) return <Boot />
  if (!user) return <Navigate to="/login" replace />
  return (
    <Workspace
      theme={theme}
      setTheme={setTheme}
      screen={screen}
      setScreen={(next) => navigate(next === "edge" ? "/edge" : `/${next}`)}
    />
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Guest />} />
      <Route path="/register" element={<Guest />} />
      <Route path="/forgot-password" element={<Guest />} />
      <Route path="/reset-password" element={<Guest />} />
      <Route path="/edge" element={<SignedIn />} />
      <Route path="/apm" element={<SignedIn />} />
      <Route path="/hardware" element={<SignedIn />} />
      <Route path="/updates" element={<SignedIn />} />
      <Route path="/" element={<Navigate to="/edge" replace />} />
      <Route path="*" element={<Navigate to="/edge" replace />} />
    </Routes>
  )
}
