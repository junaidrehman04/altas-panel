import { useEffect, useState } from "react"
import { api, ApiError } from "./api"
import type { RangeId } from "./ranges"

type QueryState<T> = {
  path: string
  range: RangeId
  data: T | null
  error: string | null
  loading: boolean
}

export function usePanelQuery<T>(path: string, range: RangeId) {
  const [state, setState] = useState<QueryState<T>>({
    path,
    range,
    data: null,
    error: null,
    loading: true,
  })
  const current =
    state.path === path && state.range === range
      ? state
      : { path, range, data: null, error: null, loading: true }

  useEffect(() => {
    let cancelled = false

    async function load() {
      try {
        const next = await api<T>(`${path}?range=${range}`)
        if (cancelled) return
        setState({ path, range, data: next, error: null, loading: false })
      } catch (err) {
        if (cancelled) return
        setState({
          path,
          range,
          data: null,
          error: err instanceof ApiError ? err.message : "Could not load this view.",
          loading: false,
        })
      }
    }

    void load()
    if (range !== "live") {
      return () => {
        cancelled = true
      }
    }
    const timer = window.setInterval(() => void load(), 5000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [path, range])

  return { data: current.data, error: current.error, loading: current.loading }
}

export function hasNumbers(values: Array<number | null>) {
  return values.some((value) => value != null && !Number.isNaN(value))
}

export type HostSeries = {
  range: string
  resolution: string
  from: string
  to: string
  points: Array<{
    t: string
    cpu_percent: number | null
    mem_used_percent: number
    mem_used_bytes: number
    disk_used_percent: number
    disk_used_bytes: number
    rx_bps: number | null
    tx_bps: number | null
  }>
}

export type AnalyticsTop = {
  name: string
  requests: number
  errors: number
  bytes: number
}

export type AnalyticsPlace = {
  name: string
  country?: string
  requests: number
}

export type AnalyticsSummary = {
  range: string
  resolution: string
  from: string
  to: string
  project: string
  geo_configured: boolean
  totals: {
    requests: number
    visitors: number
    bytes: number
    errors: number
    error_rate: number
    p95_ms: number | null
  }
  series: Array<{
    t: string
    requests: number
    errors: number
    bytes: number
    error_rate: number
    p95_ms: number | null
  }>
  top_hosts: AnalyticsTop[]
  top_apps: AnalyticsTop[]
  top_paths: AnalyticsTop[]
  countries: AnalyticsPlace[]
  cities: AnalyticsPlace[]
  devices: AnalyticsTop[]
}
