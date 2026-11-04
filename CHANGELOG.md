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
- Admin settings for auto-update, off by default: GitHub Releases check, SHA256 verification, atomic binary replace, systemd restart, and rollback. Ed25519 verification runs when `ALTAS_UPDATE_PUBKEY` is set. A project public key is not pinned yet

### Not in this version

- Creating, starting, or stopping containers from templates
- File manager UI
- Geo traffic analytics
- Malware scanning beyond the hook
- `get.altaspanel.tech` installer
- A pinned Ed25519 project key for release signatures
