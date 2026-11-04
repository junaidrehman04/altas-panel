import { useEffect, useState, type FormEvent } from "react"
import { Link, useNavigate } from "react-router-dom"
import { AuthLayout } from "../components/auth-layout"
import { EmailField, FormError, PasswordField } from "../components/fields"
import { Button } from "../components/ui/button"
import { ApiError } from "../lib/api"
import { useAuth } from "../lib/auth"
import { useRegistration } from "../lib/registration"

export function LoginPage() {
  const { login } = useAuth()
  const registration = useRegistration()
  const navigate = useNavigate()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  useEffect(() => {
    document.title = "Sign in · Altas Panel"
  }, [])

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setPending(true)
    try {
      await login(email, password)
      navigate("/dashboard", { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not sign in.")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthLayout
      title="Sign in"
      lede="Use the email and password for this panel. The session stays in this browser until you log out."
      footer={
        registration === "closed" ? (
          <>Public registration is closed. The first account is the admin.</>
        ) : registration === "loading" ? (
          <>Checking registration</>
        ) : (
          <>
            No account yet?{" "}
            <Link to="/register" className="text-accent">
              {registration === "open" ? "Create the admin account" : "Create an account"}
            </Link>
          </>
        )
      }
    >
      <form className="grid gap-4" onSubmit={onSubmit} noValidate>
        <EmailField id="login-email" value={email} onChange={setEmail} autoComplete="email" />
        <PasswordField
          id="login-password"
          label="Password"
          value={password}
          onChange={setPassword}
          autoComplete="current-password"
        />
        <div className="text-right">
          <Link to="/forgot-password" className="font-mono text-[11px] tracking-[0.12em] text-muted uppercase">
            Forgot password
          </Link>
        </div>
        <FormError>{error}</FormError>
        <Button type="submit" disabled={pending}>
          {pending ? "Signing in" : "Sign in"}
        </Button>
      </form>
    </AuthLayout>
  )
}
