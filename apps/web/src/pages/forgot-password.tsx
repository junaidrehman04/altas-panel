import { useEffect, useState, type FormEvent } from "react"
import { Link } from "react-router-dom"
import { AuthLayout } from "../components/auth-layout"
import { EmailField, FormError, FormNote } from "../components/fields"
import { Button } from "../components/ui/button"
import { api, ApiError } from "../lib/api"

export function ForgotPasswordPage() {
  const [email, setEmail] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [sent, setSent] = useState(false)
  const [pending, setPending] = useState(false)

  useEffect(() => {
    document.title = "Forgot password · Altas Panel"
  }, [])

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setPending(true)
    try {
      await api("/api/auth/forgot-password", {
        method: "POST",
        body: JSON.stringify({ email }),
      })
      setSent(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not issue a reset link.")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthLayout
      title="Forgot password"
      lede="If the email is registered, the API writes a one-hour reset link to its log. Mail delivery is not part of this release."
      footer={
        <Link to="/login" className="text-accent">
          Back to sign in
        </Link>
      }
    >
      {sent ? (
        <div className="grid gap-4">
          <FormNote>
            If an account exists for that email, the reset link is in the API process output. Open
            it in this browser, then choose a new password.
          </FormNote>
          <Button type="button" variant="outline" onClick={() => setSent(false)}>
            Use a different email
          </Button>
        </div>
      ) : (
        <form className="grid gap-4" onSubmit={onSubmit} noValidate>
          <EmailField id="forgot-email" value={email} onChange={setEmail} autoComplete="email" />
          <FormError>{error}</FormError>
          <Button type="submit" disabled={pending}>
            {pending ? "Sending link" : "Send reset link"}
          </Button>
        </form>
      )}
    </AuthLayout>
  )
}
