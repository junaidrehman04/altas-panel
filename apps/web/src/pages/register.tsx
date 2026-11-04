import { useEffect, useState, type FormEvent } from "react"
import { Link, useNavigate } from "react-router-dom"
import { AuthLayout } from "../components/auth-layout"
import { EmailField, FormError, PasswordField } from "../components/fields"
import { Button } from "../components/ui/button"
import { ApiError } from "../lib/api"
import { useAuth } from "../lib/auth"

export function RegisterPage() {
  const { register } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  useEffect(() => {
    document.title = "Create account · Altas Panel"
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
      title="Create account"
      lede="Email and a password of at least 8 characters. You are signed in as soon as the account exists."
      footer={
        <>
          Already registered?{" "}
          <Link to="/login" className="text-accent">
            Sign in
          </Link>
        </>
      }
    >
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
          {pending ? "Creating account" : "Create account"}
        </Button>
      </form>
    </AuthLayout>
  )
}
