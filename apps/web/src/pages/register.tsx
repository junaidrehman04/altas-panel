import { useEffect, useState, type FormEvent } from "react"
import { Link, useNavigate } from "react-router-dom"
import { AuthLayout } from "../components/auth-layout"
import { EmailField, FormError, PasswordField } from "../components/fields"
import { Button } from "../components/ui/button"
import { ApiError } from "../lib/api"
import { useAuth } from "../lib/auth"
import { useRegistration } from "../lib/registration"

export function RegisterPage() {
  const { register } = useAuth()
  const registration = useRegistration()
  const navigate = useNavigate()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const closed = registration === "closed"

  useEffect(() => {
    document.title = closed ? "Registration closed · Altas Panel" : "Create account · Altas Panel"
  }, [closed])

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (password.length < 8) {
      setError("Password must be at least 8 characters.")
      return
    }
    setError(null)
    setPending(true)
    try {
      await register(email, password)
      navigate("/dashboard", { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create the account.")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthLayout
      title={closed ? "Registration closed" : "Create the admin account"}
      lede={
        closed
          ? "The first account on this panel is the admin. Public registration is closed, and later accounts will be invite-only."
          : "The first account on this panel becomes the admin. After that, public registration closes. Use an email and a password of at least 8 characters. You are signed in as soon as the account exists."
      }
      footer={
        <Link to="/login" className="text-accent">
          {closed ? "Sign in" : "Already registered? Sign in"}
        </Link>
      }
    >
      {registration === "loading" ? (
        <p role="status" className="font-mono text-[11px] tracking-[0.16em] text-muted uppercase">
          Checking registration
        </p>
      ) : closed ? (
        <p className="text-sm leading-relaxed text-muted">
          Sign in with the admin account. Password reset still works from the sign-in screen.
        </p>
      ) : (
        <form className="grid gap-4" onSubmit={onSubmit} noValidate>
          <EmailField id="register-email" value={email} onChange={setEmail} autoComplete="email" />
          <PasswordField
            id="register-password"
            label="Password"
            value={password}
            onChange={setPassword}
            autoComplete="new-password"
          />
          <FormError>{error}</FormError>
          <Button type="submit" disabled={pending}>
            {pending ? "Creating account" : "Create admin account"}
          </Button>
        </form>
      )}
    </AuthLayout>
  )
}
