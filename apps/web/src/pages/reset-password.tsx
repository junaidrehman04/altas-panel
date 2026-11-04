import { useEffect, useState, type FormEvent } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { AuthLayout } from "../components/auth-layout"
import { FormError, FormNote, PasswordField } from "../components/fields"
import { Button } from "../components/ui/button"
import { api, ApiError } from "../lib/api"
import { useAuth } from "../lib/auth"

export function ResetPasswordPage() {
  const { clearLocalSession } = useAuth()
  const [params] = useSearchParams()
  const token = params.get("token") ?? ""
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)
  const [pending, setPending] = useState(false)

  useEffect(() => {
    document.title = "Reset password · Altas Panel"
  }, [])

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (password.length < 8) {
      setError("Password must be at least 8 characters.")
      return
    }
    setError(null)
    setPending(true)
    try {
      await api("/api/auth/reset-password", {
        method: "POST",
        body: JSON.stringify({ token, password }),
      })
      clearLocalSession()
      setDone(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not update the password.")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthLayout
      title="Reset password"
      lede="Choose a new password. This link works once, and every existing session for the account is signed out."
      footer={
        <Link to="/login" className="text-accent">
          Back to sign in
        </Link>
      }
    >
      {!token ? (
        <div className="grid gap-4">
          <FormError>This reset link is missing a token.</FormError>
          <Link to="/forgot-password" className="text-sm text-accent">
            Request a new link
          </Link>
        </div>
      ) : done ? (
        <div className="grid gap-4">
          <FormNote>Password updated. Sign in with the new password.</FormNote>
          <Button asChild>
            <Link to="/login">Sign in</Link>
          </Button>
        </div>
      ) : (
        <form className="grid gap-4" onSubmit={onSubmit} noValidate>
          <PasswordField
            id="reset-password"
            label="New password"
            value={password}
            onChange={setPassword}
            autoComplete="new-password"
          />
          <FormError>{error}</FormError>
          <Button type="submit" disabled={pending}>
            {pending ? "Updating password" : "Update password"}
          </Button>
        </form>
      )}
    </AuthLayout>
  )
}
