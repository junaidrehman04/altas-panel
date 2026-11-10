import type { Label, Range } from "../design/data"

export type { Label, Range }

export type PanelUser = {
  id: string
  email: string
  is_admin: boolean
  created_at: string
  totp_enabled: boolean
  last_sign_in_at: string
  last_sign_in_place: string
  initials: string
  label: string
}

export type HostCard = {
  id: string
  fqdn: string
  kind: string
  virt: string
  hypervisor: boolean
  provider: string
  ip: string
  os: string
  kernel: string
  cpu: string
  cores: number
  threads: number
  ram_bytes: number
  uptime: string
  load1: number
  load5: number
  load15: number
  reboot_pending: boolean
  status: "ok" | "warning" | "critical" | string
}

export type Bootstrap = {
  version: string
  env: string
  org: string
  user: PanelUser
  host: HostCard
  agent: { connected: boolean; cpu_percent: number | null }
}

export type Gap = { available: boolean; reason: string }

export type NetView = {
  available: boolean
  iface: string
  ingress_mbps: number | null
  egress_mbps: number | null
  peak_ingress_mbps: number | null
  peak_egress_mbps: number | null
  ingress_delta: number | null
  egress_delta: number | null
  labels: Label[]
  ingress: (number | null)[]
  egress: (number | null)[]
}

export type CountryRow = {
  code: string
  name: string
  requests: number
  lat: number | null
  lon: number | null
}

export type PathRow = { path: string; requests: number; bytes: number }

export type TrafficView = {
  available: boolean
  source: string
  requests: number
  req_per_s: number | null
  visitors: number
  bytes: number
  errors: number
  error_rate: number
  p95_ms: number | null
  geo_configured: boolean
  note: string
  labels: Label[]
  req_per_s_series: (number | null)[]
  paths: PathRow[]
  countries: CountryRow[]
}

export type PanelEvent = { at: string; sev: string; title: string; detail: string; loc: string }

export type EdgeView = {
  range: string
  collected_at: string
  network: NetView
  traffic: TrafficView
  classes: Gap
  protocols: Gap
  forecast: Gap
  clients: Gap
  events: PanelEvent[]
}

export type MemoryView = {
  total_bytes: number
  rss_bytes: number
  buffers_bytes: number
  cache_bytes: number
  free_bytes: number
  available_bytes: number
  available_percent: number
  min_available_percent: number | null
  swap_used_bytes: number
  swap_total_bytes: number
  psi: { available: boolean; memory_some: number; memory_full: number; io_some: number }
  labels: Label[]
  rss: (number | null)[]
  buffers: (number | null)[]
  cache: (number | null)[]
  available: (number | null)[]
}

export type FSView = {
  mount: string
  device: string
  fs: string
  used_bytes: number
  total_bytes: number
  used_percent: number
  inode_percent: number | null
}

export type ListenRow = {
  proto: string
  addr: string
  process: string
  pid: number
  user: string
  exposure: string
}

export type HardwareView = {
  range: string
  collected_at: string
  identity: HostCard
  memory: MemoryView
  heat: { labels: Label[]; cores: string[]; values: number[][] }
  iowait: { current: number | null; peak: number | null; labels: Label[]; values: (number | null)[] }
  steal: { applicable: boolean; reason: string; current: number | null; labels: Label[]; values: (number | null)[] }
  filesystems: FSView[]
  raid: Gap & { arrays: { name: string; level: string; state: string; members: string }[] }
  nvme: Gap & { devices: { name: string; model: string; temp_c: number | null }[] }
  ipmi: Gap
  listeners: { note: string; ports: ListenRow[] }
  deleted: { note: string; files: { pid: number; process: string; path: string; bytes: number }[] }
  kernel: Gap & { events: { lane: string; tone: string; label: string; at: string }[]; labels: Label[] }
  ssh: Gap & { failed: number; sources: { ip: string; fails: number; users: string }[] }
}

export type ProjectRow = {
  id: string
  domain: string
  stack: string
  color: string
  source: string
  cpu: number | null
  ram_bytes: number | null
  rps: number | null
  p95_ms: number | null
  err5xx: number | null
  http: boolean
  status: string
}

export type ApmView = {
  range: string
  host: { cores: number; threads: number; ram_bytes: number; cpu_percent: number | null; mem_used_bytes: number }
  projects: ProjectRow[]
  selected: string
  http: Gap & {
    labels: Label[]
    rps: (number | null)[]
    p95: (number | null)[]
    error_rate: (number | null)[]
    ok: number
    errors: number
    paths: PathRow[]
  }
  queues: Gap
  sql: Gap
  cache: Gap
  cron: Gap & { jobs: { command: string; schedule: string; source: string }[] }
  mail: Gap & { sent: number; deferred: number; bounced: number }
}

export type AlertItem = {
  id: string
  sev: "critical" | "warning" | "info"
  icon: "memory" | "disk" | "cpu" | "refresh" | "network" | "activity" | "shield" | "bell"
  title: string
  detail: string
  host: string
  time: string
  screen: "edge" | "apm" | "hardware"
  tab: string
  read: boolean
}

export function deltaText(value: number | null | undefined) {
  if (value == null || Number.isNaN(value)) return undefined
  const sign = value > 0 ? "+" : ""
  return `${sign}${value.toFixed(1)}%`
}
