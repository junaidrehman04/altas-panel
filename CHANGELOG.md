# Changelog

## 0.3.0-dev

Unreleased host history and traffic analytics.

### Added

- SQLite rollups for CPU, memory, disk, and network, sampled about every 15 seconds
- `GET /api/metrics/host/series` for live, 24h, 7d, 30d, and month windows
- Metrics page with a range selector and charts
- Analytics rollups for requests, visitors, bandwidth, error rate, p95, tops, devices, and optional geo
- `GET /api/analytics/summary` for the same windows, session required
- Panel API middleware as the traffic source. No demo rows are inserted at startup
- Optional Combined access-log tail (`ALTAS_ACCESS_LOG`), off by default, starting at the end of the file
- Optional MaxMind GeoIP database (`ALTAS_GEOIP_DB`). A missing file leaves country and city empty
- Daily visitor hashes. Raw client addresses are not stored. IPv4 is truncated to `/24` and IPv6 to `/48` before lookup or hashing

### Not in this version

- A public ingest URL for outside traffic
- Importing an existing access log from the start of the file
- Creating, starting, or stopping containers
- File manager UI
- Malware scanning beyond the report-only hook

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
- Geo traffic from outside the panel, beyond the optional access-log tail
- Malware scanning beyond the hook
- `get.altaspanel.tech` installer
- A pinned Ed25519 project key for release signatures
