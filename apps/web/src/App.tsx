import { Navigate, Outlet, Route, Routes } from "react-router-dom"
import { useAuth } from "./lib/auth"
import { DashboardPage } from "./pages/dashboard"
import { ForgotPasswordPage } from "./pages/forgot-password"
import { LoginPage } from "./pages/login"
import { NotFoundPage } from "./pages/not-found"
import { RegisterPage } from "./pages/register"
import { ResetPasswordPage } from "./pages/reset-password"
import { StacksPage } from "./pages/stacks"

function BootScreen() {
  return (
    <div className="grid min-h-svh place-items-center bg-base text-ink">
      <p className="font-mono text-[11px] tracking-[0.18em] text-muted uppercase">Checking session</p>
    </div>
  )
}

function RequireAuth() {
  const { user, loading } = useAuth()
  if (loading) return <BootScreen />
  if (!user) return <Navigate to="/login" replace />
  return <Outlet />
}

function GuestOnly() {
  const { user, loading } = useAuth()
  if (loading) return <BootScreen />
  if (user) return <Navigate to="/dashboard" replace />
  return <Outlet />
}

export default function App() {
  return (
    <Routes>
      <Route element={<GuestOnly />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
      </Route>
      <Route path="/reset-password" element={<ResetPasswordPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/stacks" element={<StacksPage />} />
      </Route>
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
