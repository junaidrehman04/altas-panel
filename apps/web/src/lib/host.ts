export type HostSnapshot = {
  collected_at: string
  hostname: string
  source: string
  uptime_seconds: number
  note?: string
  cpu: {
    usage_percent: number | null
    cores: number
  }
  memory: {
    total_bytes: number
    available_bytes: number
    used_bytes: number
    free_bytes: number
    cache_bytes: number
    used_percent: number
  }
  load: {
    load1: number
    load5: number
    load15: number
  }
  disks: Array<{
    mount: string
    filesystem: string
    total_bytes: number
    used_bytes: number
    available_bytes: number
    used_percent: number
  }>
  network: Array<{
    name: string
    rx_bytes: number
    tx_bytes: number
    rx_errors: number
    tx_errors: number
    rx_bps: number | null
    tx_bps: number | null
  }>
}
