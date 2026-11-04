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

type AuthContextValue = {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
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

  const login = useCallback(async (email: string, password: string) => {
    const data = await api<{ user: User }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    })
    setUser(data.user)
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
    () => ({ user, loading, login, register, logout, clearLocalSession }),
    [user, loading, login, register, logout, clearLocalSession],
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
