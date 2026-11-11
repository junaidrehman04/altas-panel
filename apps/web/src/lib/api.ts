export type User = {
  id: string
  email: string
  is_admin: boolean
  created_at: string
  totp_enabled?: boolean
  last_sign_in_at?: string
  last_sign_in_place?: string
  initials?: string
  label?: string
}

export class ApiError extends Error {
  status: number
  attemptsRemaining?: number
  locked?: boolean

  constructor(status: number, message: string, extra?: { attemptsRemaining?: number; locked?: boolean }) {
    super(message)
    this.name = "ApiError"
    this.status = status
    this.attemptsRemaining = extra?.attemptsRemaining
    this.locked = extra?.locked
  }
}

type ErrorBody = { error?: string; attempts_remaining?: number; locked?: boolean }

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body != null && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }

  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      headers,
      credentials: "same-origin",
    })
  } catch {
    throw new ApiError(0, "Could not reach the API on port 8080.")
  }

  if (response.status === 204) {
    return undefined as T
  }

  const text = await response.text()
  let data: ErrorBody | null = null
  if (text) {
    try {
      data = JSON.parse(text) as ErrorBody
    } catch {
      data = null
    }
  }

  if (!response.ok) {
    const message =
      data?.error ??
      (response.status >= 500 ? "Could not reach the API on port 8080." : "Request failed.")
    throw new ApiError(response.status, message, {
      attemptsRemaining: data?.attempts_remaining,
      locked: data?.locked,
    })
  }

  return data as T
}
