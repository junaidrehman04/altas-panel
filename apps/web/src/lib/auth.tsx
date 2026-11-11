import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"
import { api, type User } from "./api"

export type LoginResult = {
  totpRequired: boolean
  challenge?: string
  account?: string
  user?: User
}

type AuthContextValue = {
  user: User | null
  loading: boolean
  login: (email: string, password: string, remember?: boolean) => Promise<LoginResult>
  completeTotp: (challenge: string, code: string, trust: boolean) => Promise<User>
  completeRecovery: (challenge: string, code: string) => Promise<User>
  register: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
  clearLocalSession: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    api<{ user: User }>("/api/me")
      .then((data) => {
        if (!cancelled) setUser(data.user)
      })
      .catch(() => {
        if (!cancelled) setUser(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const refresh = useCallback(async () => {
    const data = await api<{ user: User }>("/api/me")
    setUser(data.user)
  }, [])

  const login = useCallback(async (email: string, password: string, remember = true) => {
    const data = await api<{ user?: User; totp_required?: boolean; challenge?: string; account?: string }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password, remember }),
    })
    if (data.totp_required) {
      return { totpRequired: true, challenge: data.challenge, account: data.account }
    }
    setUser(data.user ?? null)
    return { totpRequired: false, user: data.user }
  }, [])

  const completeTotp = useCallback(async (challenge: string, code: string, trust: boolean) => {
    const data = await api<{ user: User }>("/api/auth/totp", {
      method: "POST",
      body: JSON.stringify({ challenge, code, trust }),
    })
    setUser(data.user)
    return data.user
  }, [])

  const completeRecovery = useCallback(async (challenge: string, code: string) => {
    const data = await api<{ user: User }>("/api/auth/recovery", {
      method: "POST",
      body: JSON.stringify({ challenge, code }),
    })
    setUser(data.user)
    return data.user
  }, [])

  const register = useCallback(async (email: string, password: string) => {
    const data = await api<{ user: User }>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    })
    setUser(data.user)
  }, [])

  const logout = useCallback(async () => {
    await api<void>("/api/auth/logout", { method: "POST" })
    setUser(null)
  }, [])

  const clearLocalSession = useCallback(() => {
    setUser(null)
  }, [])

  const value = useMemo(
    () => ({ user, loading, login, completeTotp, completeRecovery, register, logout, refresh, clearLocalSession }),
    [user, loading, login, completeTotp, completeRecovery, register, logout, refresh, clearLocalSession],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider")
  }
  return ctx
}
