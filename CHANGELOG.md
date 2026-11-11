# Changelog

## 0.2.0

The signed-in panel now uses the approved monitoring layout, fed by this host. Earlier unreleased notes for host history and traffic rollups are part of this version. There is no 0.3.0.

### Added

- Light and dark shell for Edge Traffic, Projects & APM, and Hardware: theme switch, one-host picker, 1h/6h/24h/7d/30d range, live pause, command palette, and G E / G A / G H
- Notifications from real thresholds (memory, disk, inodes, load, steal on a guest, public sensitive ports, reboot flag, panel 5xx rate, sign-in lockouts)
- TOTP enrollment with a QR code and manual secret, hashed one-time recovery codes, a six-digit sign-in step, optional 30-day browser trust, and a 15-minute lockout after five failures
- “Keep me signed in for 14 days” sets a 14-day session. Leaving it unchecked keeps a browser session cookie and a 12-hour server expiry
- Last successful sign-in time, and city/country when `ALTAS_GEOIP_DB` resolves the truncated address
- Panel pages read `/proc`, `/sys`, statfs, listening sockets, and SQLite rollups. Missing IPMI, RAID, GeoIP, access logs, queues, and SQL render the same components in an empty or unavailable state
- SQLite rollups for CPU, memory composition, disk, network, per-core utilisation, requests, visitors, bandwidth, error rate, and p95
- Optional Combined access-log tail (`ALTAS_ACCESS_LOG`) and optional MaxMind database (`ALTAS_GEOIP_DB`)
- Read-only container list when a Docker socket is present

### Changed

- The running version is `v0.2.0`
- Demo identities from the design prototype are not used. The shell shows this hostname, the signed-in account, `ALTAS_ENV`, and `ALTAS_ORG` when set

### Not in this version

- WebAuthn / security keys (the tab is visible and does not sign anyone in)
- A second server in the host picker
- Storing raw client addresses, WAF verdicts, or traffic-class labels
- IPMI command execution, SMART wear beyond what sysfs exposes, or firewall changes from the UI
- Queue, SQL, and object-cache metrics
- Creating, starting, or stopping containers
- File manager
- Malware scanning beyond the report-only hook
- `get.altaspanel.tech` installer

## 0.2.0-dev

Superseded by 0.2.0. The notes below were the unreleased foundation.

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
