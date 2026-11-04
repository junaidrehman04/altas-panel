import { useState, type ReactNode } from "react"
import { Input } from "./ui/input"
import { Label } from "./ui/label"

export function EmailField({
  id,
  value,
  onChange,
  autoComplete,
}: {
  id: string
  value: string
  onChange: (value: string) => void
  autoComplete: string
}) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>Email</Label>
      <Input
        id={id}
        name={id}
        type="email"
        inputMode="email"
        autoComplete={autoComplete}
        spellCheck={false}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        required
      />
    </div>
  )
}

export function PasswordField({
  id,
  label,
  value,
  onChange,
  autoComplete,
}: {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  autoComplete: string
}) {
  const [visible, setVisible] = useState(false)
  return (
    <div className="grid gap-2">
      <div className="flex items-center justify-between gap-3">
        <Label htmlFor={id}>{label}</Label>
        <button
          type="button"
          className="font-mono text-[11px] tracking-[0.12em] text-muted uppercase hover:text-ink"
          aria-pressed={visible}
          aria-controls={id}
          onClick={() => setVisible((current) => !current)}
        >
          {visible ? "Hide" : "Show"}
        </button>
      </div>
      <Input
        id={id}
        name={id}
        type={visible ? "text" : "password"}
        autoComplete={autoComplete}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        required
      />
    </div>
  )
}

export function FormError({ children }: { children: ReactNode }) {
  if (!children) return null
  return (
    <p role="alert" className="border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
      {children}
    </p>
  )
}

export function FormNote({ children }: { children: ReactNode }) {
  return (
    <p role="status" className="border border-line bg-base px-3 py-2 text-sm text-ink">
      {children}
    </p>
  )
}
