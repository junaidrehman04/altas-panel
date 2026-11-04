export type UpdateSettings = {
  auto_update: boolean
  last_check: string | null
  current_version: string
  latest_available: string
  rollback_available: boolean
  last_error: string
  check_interval_seconds: number
  restart_mode: string
  unit: string
  verification: string
  repository: string
  apply_behavior: string
  applied: boolean
  update_available: boolean
}
