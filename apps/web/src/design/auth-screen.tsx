import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import { ApiError, api } from "../lib/api"
import { useAuth } from "../lib/auth"
import { useRegistration } from "../lib/registration"
import { ThemeSwitch, type Theme } from "./shell"
import { Dot, Icon, Logo, Segmented, Spinner, Wordmark } from "./ui"

export type AuthNotice = "expired" | "signedout" | null

type Step = "credentials" | "totp" | "recovery" | "locked" | "success" | "register" | "forgot" | "reset"
type Method = "password" | "key"

type PublicInfo = { version: string; hostname: string; env: string }

const LAST_KEY = "altas-last-sign-in"

function readLast() {
  try {
    const raw = localStorage.getItem(LAST_KEY)
    if (!raw) return null
    return JSON.parse(raw) as { at: string; place: string }
  } catch {
    return null
  }
}

export function rememberSignIn(at: string, place: string) {
  if (!at) return
  localStorage.setItem(LAST_KEY, JSON.stringify({ at, place }))
}

function formatSignIn(at: string, place: string) {
  const date = new Date(at)
  if (Number.isNaN(date.getTime())) return ""
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]
  const text = `${months[date.getMonth()]} ${date.getDate()}, ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`
  return place ? `${text} from ${place}` : text
}

export function AuthScreen({ theme, setTheme, notice }: { theme: Theme; setTheme: (t: Theme) => void; notice: AuthNotice }) {
  const { login, completeTotp, completeRecovery, register } = useAuth()
  const registration = useRegistration()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const resetToken = params.get("token") ?? ""
  const path = window.location.pathname
  const [step, setStep] = useState<Step>(() => {
    if (path.startsWith("/register")) return "register"
    if (path.startsWith("/forgot")) return "forgot"
    if (path.startsWith("/reset")) return "reset"
    return "credentials"
  })
  const [method, setMethod] = useState<Method>("password")
  const [user, setUser] = useState("")
  const [pass, setPass] = useState("")
  const [show, setShow] = useState(false)
  const [remember, setRemember] = useState(true)
  const [caps, setCaps] = useState(false)
  const [errors, setErrors] = useState<{ user?: string; pass?: string; form?: string }>({})
  const [attempts, setAttempts] = useState<number | null>(null)
  const [noticeOpen, setNoticeOpen] = useState(notice !== null)
  const [busy, setBusy] = useState(false)
  const [keyNote, setKeyNote] = useState("")
  const [code, setCode] = useState(["", "", "", "", "", ""])
  const [codeError, setCodeError] = useState("")
  const [trust, setTrust] = useState(false)
  const [recovery, setRecovery] = useState("")
  const [challenge, setChallenge] = useState("")
  const [secs, setSecs] = useState(() => 30 - (Math.floor(Date.now() / 1000) % 30))
  const [info, setInfo] = useState<PublicInfo>({ version: "", hostname: "", env: "" })
  const [forgotSent, setForgotSent] = useState(false)
  const [resetPass, setResetPass] = useState("")
  const digits = useRef<(HTMLInputElement | null)[]>([])
  const passRef = useRef<HTMLInputElement>(null)
  const last = readLast()

  useEffect(() => {
    api<PublicInfo>("/api/version").then(setInfo).catch(() => undefined)
  }, [])

  useEffect(() => {
    if (step !== "totp") return
    const t = setInterval(() => setSecs(30 - (Math.floor(Date.now() / 1000) % 30)), 1000)
    return () => clearInterval(t)
  }, [step])

  const finish = () => {
    setStep("success")
    window.setTimeout(() => navigate("/edge", { replace: true }), 700)
  }

  const submitCredentials = async (e: FormEvent) => {
    e.preventDefault()
    const next: typeof errors = {}
    if (!user.trim()) next.user = "Enter your username or email address."
    if (!pass) next.pass = "Enter your password."
    setErrors(next)
    if (next.user || next.pass) return
    setBusy(true)
    setKeyNote("")
    try {
      const result = await login(user.trim(), pass, remember)
      if (result.totpRequired && result.challenge) {
        setChallenge(result.challenge)
        setNoticeOpen(false)
        setStep("totp")
        window.setTimeout(() => digits.current[0]?.focus(), 30)
        return
      }
      if (result.user?.last_sign_in_at) rememberSignIn(result.user.last_sign_in_at, result.user.last_sign_in_place ?? "")
      setAttempts(null)
      finish()
    } catch (err) {
      const apiErr = err instanceof ApiError ? err : null
      const remaining = apiErr?.attemptsRemaining
      if (apiErr?.locked) {
        setStep("locked")
        return
      }
      if (remaining != null) setAttempts(remaining)
      setErrors({ form: remaining != null && remaining < 5 ? `Incorrect username or password. ${remaining} attempt${remaining === 1 ? "" : "s"} left before a 15-minute lockout.` : apiErr?.message ?? "Could not sign in." })
      setPass("")
      passRef.current?.focus()
    } finally {
      setBusy(false)
    }
  }

  const verify = async (value: string) => {
    setBusy(true)
    try {
      const signed = await completeTotp(challenge, value, trust)
      if (signed.last_sign_in_at) rememberSignIn(signed.last_sign_in_at, signed.last_sign_in_place ?? "")
      finish()
    } catch (err) {
      const apiErr = err instanceof ApiError ? err : null
      if (apiErr?.locked) {
        setStep("locked")
        return
      }
      setCodeError(apiErr?.message ?? "That code is invalid or has expired. Wait for a new code and try again.")
      setCode(["", "", "", "", "", ""])
      digits.current[0]?.focus()
    } finally {
      setBusy(false)
    }
  }

  const setDigit = (i: number, v: string) => {
    const clean = v.replace(/\D/g, "")
    if (clean.length > 1) {
      const arr = clean.slice(0, 6).split("")
      const next = code.map((_, k) => arr[k] ?? "")
      setCode(next)
      digits.current[Math.min(5, arr.length)]?.focus()
      if (arr.length === 6) void verify(arr.join(""))
      return
    }
    const next = [...code]
    next[i] = clean
    setCode(next)
    setCodeError("")
    if (clean && i < 5) digits.current[i + 1]?.focus()
    if (next.every((d) => d)) void verify(next.join(""))
  }
  const onDigitKey = (i: number, e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Backspace" && !code[i] && i > 0) digits.current[i - 1]?.focus()
    if (e.key === "ArrowLeft" && i > 0) digits.current[i - 1]?.focus()
    if (e.key === "ArrowRight" && i < 5) digits.current[i + 1]?.focus()
  }

  const submitRegister = async (e: FormEvent) => {
    e.preventDefault()
    const next: typeof errors = {}
    if (!user.trim()) next.user = "Enter an email address."
    if (pass.length < 8) next.pass = "Password must be at least 8 characters."
    setErrors(next)
    if (next.user || next.pass) return
    setBusy(true)
    try {
      await register(user.trim(), pass)
      finish()
    } catch (err) {
      setErrors({ form: err instanceof ApiError ? err.message : "Could not create the account." })
    } finally {
      setBusy(false)
    }
  }

  const submitForgot = async (e: FormEvent) => {
    e.preventDefault()
    if (!user.trim()) {
      setErrors({ user: "Enter your email address." })
      return
    }
    setBusy(true)
    try {
      await api("/api/auth/forgot-password", { method: "POST", body: JSON.stringify({ email: user.trim() }) })
      setForgotSent(true)
      setErrors({})
    } catch (err) {
      setErrors({ form: err instanceof ApiError ? err.message : "Could not issue a reset link." })
    } finally {
      setBusy(false)
    }
  }

  const submitReset = async (e: FormEvent) => {
    e.preventDefault()
    if (resetPass.length < 8) {
      setErrors({ pass: "Password must be at least 8 characters." })
      return
    }
    setBusy(true)
    try {
      await api("/api/auth/reset-password", { method: "POST", body: JSON.stringify({ token: resetToken, password: resetPass }) })
      setStep("credentials")
      setErrors({ form: "" })
      setNoticeOpen(true)
      navigate("/login", { replace: true })
    } catch (err) {
      setErrors({ form: err instanceof ApiError ? err.message : "Reset link is invalid or expired." })
    } finally {
      setBusy(false)
    }
  }

  const stepIndex = step === "credentials" || step === "register" || step === "forgot" || step === "reset" ? 1 : 2
  const version = info.version || "v0.3.0"
  const envLabel = info.env === "production" ? "Production" : info.env === "development" ? "Development" : info.env || "Development"

  return (
    <div className="auth">
      <div className="auth-bg" aria-hidden="true" />
      <header className="auth-top">
        <Wordmark />
        <div className="auth-top-right">
          <a className="status-chip" href="#status" onClick={(e) => e.preventDefault()}><Dot tone="ok" />All systems operational</a>
          <ThemeSwitch theme={theme} setTheme={setTheme} />
        </div>
      </header>

      <main className="auth-main">
        {noticeOpen && notice && step === "credentials" && (
          <div className={`notice ${notice === "signedout" ? "notice-ok" : "notice-info"}`} role="status">
            <Icon name={notice === "signedout" ? "check" : "clock"} />
            <p>{notice === "signedout" ? "You have been signed out securely. All active panel sessions on this browser were closed." : "Your session ended. Sign in again to continue."}</p>
            <button className="icon-btn ghost" aria-label="Dismiss notice" onClick={() => setNoticeOpen(false)}><Icon name="x" size={14} /></button>
          </div>
        )}

        <div className="auth-card">
          <div className="auth-server">
            <span className="server-icon"><Icon name="server" size={16} /></span>
            <div>
              <b className="mono">{info.hostname || "This host"}</b>
              <small><Dot tone="ok" />Panel online · ALTAS {version.replace(/^v/, "")}</small>
            </div>
            <span className="env-badge">{envLabel}</span>
          </div>

          {(step === "credentials" || step === "totp" || step === "recovery") && (
            <div className="auth-steps" aria-label={`Step ${stepIndex} of 2`}>
              <span className="on" /><span className={stepIndex === 2 ? "on" : ""} />
              <small>Step {stepIndex} of 2 · {stepIndex === 1 ? "Identify" : "Verify"}</small>
            </div>
          )}

          <div className="auth-body">
            {step === "credentials" && (
              <>
                <h1>Welcome back</h1>
                <p className="auth-sub">Sign in to manage servers, projects and security.</p>
                <div className="method-switch">
                  <Segmented label="Sign-in method" value={method} onChange={(m) => { setMethod(m); setErrors({}); setKeyNote("") }} options={[{ value: "password", label: <><Icon name="lock" size={13} />Password</> }, { value: "key", label: <><Icon name="key" size={13} />Security key</> }]} />
                </div>
                {errors.form && <div className="notice notice-error" role="alert"><Icon name="warning" /><p>{errors.form}</p></div>}
                {keyNote && <div className="notice notice-info" role="status"><Icon name="info" /><p>{keyNote}</p></div>}

                <form onSubmit={method === "password" ? (e) => void submitCredentials(e) : (e) => { e.preventDefault(); if (!user.trim()) { setErrors({ user: "Enter your username first so we can find your registered keys." }); return } setKeyNote("Security keys (WebAuthn) are coming in a later version.") }} noValidate>
                  <label className={`field ${errors.user ? "invalid" : ""}`}>
                    <span>Username or email</span>
                    <div className="input-wrap has-icon">
                      <Icon name="user" size={15} className="input-icon" />
                      <input value={user} onChange={(e) => setUser(e.target.value)} autoComplete="username webauthn" autoFocus aria-invalid={!!errors.user} aria-describedby={errors.user ? "err-user" : undefined} />
                    </div>
                    {errors.user && <em id="err-user"><Icon name="warning" size={12} />{errors.user}</em>}
                  </label>

                  {method === "password" ? (
                    <>
                      <label className={`field ${errors.pass ? "invalid" : ""}`}>
                        <span className="field-row">Password<Link to="/forgot-password" onClick={() => setStep("forgot")}>Forgot password?</Link></span>
                        <div className="input-wrap has-icon">
                          <Icon name="lock" size={15} className="input-icon" />
                          <input ref={passRef} type={show ? "text" : "password"} value={pass} onChange={(e) => setPass(e.target.value)} autoComplete="current-password" placeholder="Enter your password"
                            onKeyUp={(e) => setCaps(e.getModifierState("CapsLock"))} aria-invalid={!!errors.pass} aria-describedby="pass-help" />
                          <button type="button" className="input-action" onClick={() => setShow((v) => !v)} aria-label={show ? "Hide password" : "Show password"} aria-pressed={show}>
                            <Icon name={show ? "eyeOff" : "eye"} size={15} />
                          </button>
                        </div>
                        {errors.pass ? <em id="pass-help"><Icon name="warning" size={12} />{errors.pass}</em> : caps ? <small id="pass-help" className="warn-text"><Icon name="warning" size={12} />Caps Lock is on</small> : null}
                      </label>
                      <label className="check">
                        <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
                        <span>Keep me signed in for 14 days</span>
                      </label>
                      <button className="btn btn-primary btn-block" type="submit" disabled={busy}>{busy ? <><Spinner />Checking credentials…</> : <>Continue<Icon name="arrow" size={15} /></>}</button>
                    </>
                  ) : (
                    <div className="key-box">
                      <span className="key-visual"><Icon name="key" size={22} /></span>
                      <p>Use a FIDO2 / WebAuthn key or your device passkey. No password or code needed.</p>
                      <button className="btn btn-primary btn-block" type="submit">Sign in with security key</button>
                    </div>
                  )}
                </form>
                {attempts != null && attempts < 5 && attempts > 0 && <p className="attempts"><Icon name="shield" size={12} />{attempts} of 5 attempts remaining</p>}
                {registration === "open" && <p className="auth-foot-link">First time on this host? <button className="link" onClick={() => { setStep("register"); setErrors({}) }}>Create the admin account</button></p>}
              </>
            )}

            {step === "register" && (
              <form onSubmit={(e) => void submitRegister(e)} noValidate>
                <button type="button" className="back-link" onClick={() => { setStep("credentials"); setErrors({}) }}><Icon name="chevron" size={12} className="flip" />Back</button>
                <h1>Create the admin account</h1>
                <p className="auth-sub">The first account becomes the panel administrator. Public registration closes after that.</p>
                {registration === "closed" && <div className="notice notice-error" role="alert"><Icon name="warning" /><p>Public registration is closed. The first account is the panel admin; further accounts will be invite-only.</p></div>}
                {errors.form && <div className="notice notice-error" role="alert"><Icon name="warning" /><p>{errors.form}</p></div>}
                <label className={`field ${errors.user ? "invalid" : ""}`}>
                  <span>Email</span>
                  <div className="input-wrap has-icon"><Icon name="user" size={15} className="input-icon" /><input value={user} onChange={(e) => setUser(e.target.value)} autoComplete="username" autoFocus /></div>
                  {errors.user && <em><Icon name="warning" size={12} />{errors.user}</em>}
                </label>
                <label className={`field ${errors.pass ? "invalid" : ""}`}>
                  <span>Password</span>
                  <div className="input-wrap has-icon"><Icon name="lock" size={15} className="input-icon" /><input type={show ? "text" : "password"} value={pass} onChange={(e) => setPass(e.target.value)} autoComplete="new-password" placeholder="At least 8 characters" /></div>
                  {errors.pass && <em><Icon name="warning" size={12} />{errors.pass}</em>}
                </label>
                <button className="btn btn-primary btn-block" type="submit" disabled={busy || registration === "closed"}>{busy ? <><Spinner />Creating account…</> : "Create account"}</button>
              </form>
            )}

            {step === "forgot" && (
              <form onSubmit={(e) => void submitForgot(e)}>
                <button type="button" className="back-link" onClick={() => { setStep("credentials"); navigate("/login") }}><Icon name="chevron" size={12} className="flip" />Back</button>
                <h1>Forgot password</h1>
                <p className="auth-sub">If the email is registered, a one-hour reset link is written to the API log. Mail delivery is not part of this release.</p>
                {forgotSent ? <div className="notice notice-ok" role="status"><Icon name="check" /><p>If an account exists for that email, the reset link is in the API process output.</p></div> : null}
                {errors.form && <div className="notice notice-error" role="alert"><Icon name="warning" /><p>{errors.form}</p></div>}
                <label className={`field ${errors.user ? "invalid" : ""}`}>
                  <span>Email</span>
                  <div className="input-wrap has-icon"><Icon name="user" size={15} className="input-icon" /><input value={user} onChange={(e) => setUser(e.target.value)} autoFocus /></div>
                  {errors.user && <em><Icon name="warning" size={12} />{errors.user}</em>}
                </label>
                <button className="btn btn-primary btn-block" type="submit" disabled={busy}>{busy ? <><Spinner />Sending…</> : "Send reset link"}</button>
              </form>
            )}

            {step === "reset" && (
              <form onSubmit={(e) => void submitReset(e)}>
                <h1>Choose a new password</h1>
                <p className="auth-sub">This signs out every existing session for the account.</p>
                {errors.form && <div className="notice notice-error" role="alert"><Icon name="warning" /><p>{errors.form}</p></div>}
                <label className={`field ${errors.pass ? "invalid" : ""}`}>
                  <span>New password</span>
                  <div className="input-wrap has-icon"><Icon name="lock" size={15} className="input-icon" /><input type="password" value={resetPass} onChange={(e) => setResetPass(e.target.value)} autoFocus autoComplete="new-password" /></div>
                  {errors.pass && <em><Icon name="warning" size={12} />{errors.pass}</em>}
                </label>
                <button className="btn btn-primary btn-block" type="submit" disabled={busy || !resetToken}>{busy ? <><Spinner />Saving…</> : "Update password"}</button>
              </form>
            )}

            {step === "totp" && (
              <div>
                <button className="back-link" onClick={() => { setStep("credentials"); setCodeError("") }}><Icon name="chevron" size={12} className="flip" />Back</button>
                <div className="verify-head">
                  <span className="verify-icon"><Icon name="shield" size={20} /></span>
                  <div>
                    <h1>Verify it's you</h1>
                    <p className="auth-sub">Enter the 6-digit code from your authenticator app for <b className="mono">{user}</b>.</p>
                  </div>
                </div>
                {codeError && <div className="notice notice-error" role="alert"><Icon name="warning" /><p>{codeError}</p></div>}
                <fieldset className="otp" aria-label="Authentication code">
                  {code.map((d, i) => (
                    <input key={i} ref={(el) => { digits.current[i] = el }} value={d} inputMode="numeric" autoComplete={i === 0 ? "one-time-code" : "off"} maxLength={6}
                      aria-label={`Digit ${i + 1}`} onChange={(e) => setDigit(i, e.target.value)} onKeyDown={(e) => onDigitKey(i, e)} className={`${codeError ? "invalid" : ""} ${d ? "filled" : ""}`} disabled={busy} />
                  ))}
                </fieldset>
                <div className="totp-timer">
                  <svg width="18" height="18" viewBox="0 0 20 20" aria-hidden="true"><circle cx="10" cy="10" r="8" className="ring-bg" /><circle cx="10" cy="10" r="8" className={`ring ${secs <= 5 ? "low" : ""}`} strokeDasharray={`${(secs / 30) * 50.3} 50.3`} transform="rotate(-90 10 10)" /></svg>
                  <span>Code refreshes in <b className="mono">{secs}s</b></span>
                </div>
                <label className="check"><input type="checkbox" checked={trust} onChange={(e) => setTrust(e.target.checked)} /><span>Don't ask again on this browser for 30 days</span></label>
                <button className="btn btn-primary btn-block" disabled={busy || code.some((c) => !c)} onClick={() => void verify(code.join(""))}>{busy ? <><Spinner />Verifying…</> : "Verify and sign in"}</button>
                <p className="auth-foot-link">Can't access your authenticator? <button className="link" onClick={() => { setStep("recovery"); setCodeError("") }}>Use a recovery code</button></p>
              </div>
            )}

            {step === "recovery" && (
              <form onSubmit={(e) => { e.preventDefault(); void (async () => {
                setBusy(true)
                try {
                  const signed = await completeRecovery(challenge, recovery.trim())
                  if (signed.last_sign_in_at) rememberSignIn(signed.last_sign_in_at, signed.last_sign_in_place ?? "")
                  finish()
                } catch (err) {
                  setCodeError(err instanceof ApiError ? err.message : "Recovery code was not accepted.")
                } finally {
                  setBusy(false)
                }
              })() }}>
                <button type="button" className="back-link" onClick={() => { setStep("totp"); setCodeError("") }}><Icon name="chevron" size={12} className="flip" />Back</button>
                <h1>Use a recovery code</h1>
                <p className="auth-sub">Each code works once. You can turn the authenticator off afterwards from account security.</p>
                <label className={`field ${codeError ? "invalid" : ""}`}>
                  <span>Recovery code</span>
                  <div className="input-wrap has-icon">
                    <Icon name="key" size={15} className="input-icon" />
                    <input className="mono" placeholder="XXXX-XXXX" value={recovery} onChange={(e) => { setRecovery(e.target.value.toUpperCase()); setCodeError("") }} autoFocus aria-invalid={!!codeError} />
                  </div>
                  {codeError && <em><Icon name="warning" size={12} />{codeError}</em>}
                </label>
                <button className="btn btn-primary btn-block" type="submit" disabled={busy}>{busy ? <><Spinner />Checking…</> : "Sign in"}</button>
              </form>
            )}

            {step === "locked" && (
              <div className="center-state">
                <span className="state-icon danger"><Icon name="lock" size={22} /></span>
                <h1>Account temporarily locked</h1>
                <p className="auth-sub">Too many failed attempts. Try again in 15 minutes.</p>
                <button className="btn btn-block" onClick={() => { setAttempts(null); setErrors({}); setPass(""); setStep("credentials") }}>Back to sign in</button>
              </div>
            )}

            {step === "success" && (
              <div className="center-state" role="status">
                <span className="state-icon ok"><Icon name="check" size={22} /></span>
                <h1>Welcome back</h1>
                <p className="auth-sub">Loading your server…</p>
                <div className="progress-line"><i /></div>
              </div>
            )}
          </div>

          <div className="auth-card-foot">
            <Icon name="shield" size={13} />
            <span>Protected by 2FA, TLS 1.3 and brute-force lockout</span>
          </div>
        </div>

        <p className="auth-meta">{last ? `Last successful sign-in: ${formatSignIn(last.at, last.place)}` : "No previous sign-in on this browser"}</p>
      </main>

      <footer className="auth-footer">
        <span className="auth-footer-brand"><Logo size={16} />ALTAS Panel {version} · MIT</span>
        <nav aria-label="Footer">
          <a href="https://github.com/junaidrehman04/altas-panel" target="_blank" rel="noreferrer">Documentation</a>
          <a href="#status" onClick={(e) => e.preventDefault()}>Status</a>
          <a href="https://github.com/junaidrehman04/altas-panel/blob/main/LICENSE" target="_blank" rel="noreferrer">Privacy</a>
          <label className="lang"><Icon name="globe" size={13} /><select aria-label="Language" defaultValue="en"><option value="en">English</option><option value="de">Deutsch</option><option value="fr">Français</option><option value="es">Español</option></select></label>
        </nav>
      </footer>
    </div>
  )
}
