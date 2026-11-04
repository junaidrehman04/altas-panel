import { useEffect, useState } from "react"
import { api } from "./api"

export type RegistrationState = "loading" | "open" | "closed" | "unknown"

export function useRegistration(): RegistrationState {
  const [state, setState] = useState<RegistrationState>("loading")

  useEffect(() => {
    let cancelled = false
    api<{ open: boolean }>("/api/auth/registration")
      .then((data) => {
        if (!cancelled) setState(data.open ? "open" : "closed")
      })
      .catch(() => {
        if (!cancelled) setState("unknown")
      })
    return () => {
      cancelled = true
    }
  }, [])

  return state
}
