# Contributing

Altas Panel is MIT licensed. v0.3.0 installs with `scripts/install.sh` onto systemd, embeds the monitoring shell in the API binary, and updates that binary from GitHub Releases when an admin enables it. GeoLite2-Country downloads when `MAXMIND_LICENSE_KEY` is set. The first registered account is the admin; after that, public registration stays closed. The design is `docs/architecture/phase-1-stacks-monitoring.md`, `docs/architecture/analytics-metrics.md`, and `docs/architecture/auto-update.md`.

Fixes to auth, metrics, analytics rollups, the jail, and the Compose files are welcome. Do not add container create/start/stop, a file manager UI, or a malware scanner that deletes files unless a release opens that work. Do not add a Prometheus stack or other heavy monitoring dependencies. Do not store raw client addresses.

## Setup

Follow the quick start in the README. Run the Go API and the Vite app in two terminals.

## Checks

From the repository root:

```bash
go test ./...
cd apps/web && npm run build
```

Format Go changes with `gofmt` before you commit.

## Commits

Use a short semantic subject, for example `fix(api): reject expired reset tokens`.

## Database files

Do not commit SQLite databases. The API creates `data/altas.db` on startup and applies the auth schema itself.

## Author

Muhammad Junaid Rehman Siddiqui — [junaidrehman04](https://github.com/junaidrehman04)
