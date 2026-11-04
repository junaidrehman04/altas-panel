export type User = {
  id: string
  email: string
  created_at: string
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

type ErrorBody = { error?: string }

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
    throw new ApiError(response.status, message)
  }

  return data as T
}
