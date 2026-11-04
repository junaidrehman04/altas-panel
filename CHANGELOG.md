# Changelog

## 0.2.0-dev

Unreleased foundation for stacks and true-state monitoring.

### Added

- Host CPU, memory, disk, and network summaries from `/proc`, with a non-Linux fallback that does not invent those numbers
- `GET /api/metrics/host` for a signed-in session, shown on the overview
- One-shot reader at `apps/api/cmd/agent`
- Path jail and command allowlist, with tests for traversal and shell injection
- Stack registry for `docker-compose`, `laravel`, and `nextjs`
- Optional read-only Docker Compose inventory (`ALTAS_DOCKER_PROBE`, off by default)
- Report-only malware hook. Auto-delete is refused. ClamAV is not executed
- Docker Compose files for a production-style panel deploy and a dev stack

### Not in this version

- Creating, starting, or stopping containers from templates
- File manager UI
- Geo traffic analytics
- Malware scanning beyond the hook
- `get.altaspanel.tech` installer
- Auto-update (settings toggle, GitHub Releases check, checksum verify, safe restart, rollback). Planned next, not implemented
