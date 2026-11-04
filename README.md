# Altas Panel

Ultra-lightweight open-source control panel for a 1GB VPS and a 64-core dedicated box, from the same plane.

**v0.1.0** ships accounts, sessions, password reset, and an empty signed-in shell. Nothing else is pretending to manage a server yet.

## True state, not a parked runtime

Traditional panels leave 500MB–1.5GB of UI runtime, bundled databases, and pollers on the machine they claim to administer. On a 1GB cloud VPS that is the difference between serving traffic and swapping to death.

Altas is built to stay under 18MB resident. It reads the kernel directly from `/proc`, so cache is not reported as “memory full” and the RAM stays available for the apps on the box. A small VPS keeps its headroom. A high-core dedicated server can grow into RAID, thermal, IPMI, and NIC telemetry without a second operating system sitting in the panel.

That reader is the product. This release is the door in front of it: register, sign in, sign out, reset a password, and land on a quiet overview.

## Quick start

You need Go 1.22 or newer and Node.js 22 or newer.

API:

```bash
cd apps/api
go run ./cmd/server
```

The API listens on [http://127.0.0.1:8080](http://127.0.0.1:8080).

Web, in a second terminal:

```bash
cd apps/web
npm install
npm run dev
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). The dev server proxies `/api` to the Go process.

Create an account, and you are on the overview. Sign out from the top bar. Unauthenticated visits to the overview return to the sign-in screen.

### Password reset without mail

v0.1.0 does not send email. With `ALTAS_ENV=development` (the default), a forgot-password request stores a single-use token and prints the full reset URL to the API stdout:

```text
reset url: http://127.0.0.1:5173/reset-password?token=...
```

Open that URL, set a new password, and sign in again. Existing sessions for the account are revoked. Set `ALTAS_ENV=production` when you do not want reset URLs in the log.

## Checks

From the repository root:

```bash
go test ./...
cd apps/web && npm run build
```

`go test ./...` also works from `apps/api`.

## Layout

```text
apps/api/cmd/server          API process
apps/api/internal/auth       Argon2id passwords, sessions, reset tokens
apps/api/internal/httpapi    Chi routes
apps/api/internal/db         SQLite schema (pure Go driver)
apps/web                     Vite + React + Tailwind shell
```

The Go module lives at the repository root so one `go test ./...` covers the API. `cd apps/api && go run ./cmd/server` still runs the server.

## Configuration

The API reads its environment directly. See `.env.example`.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ALTAS_ADDR` | `:8080` | Listen address |
| `ALTAS_DB` | `data/altas.db` | SQLite file, relative to the API working directory |
| `ALTAS_ENV` | `development` | `production` stops printing reset URLs |
| `ALTAS_WEB_ORIGIN` | `http://127.0.0.1:5173` | Origin used in the logged reset link |
| `ALTAS_SESSION_TTL` | `168h` | Session lifetime |
| `ALTAS_RESET_TTL` | `1h` | Reset token lifetime |

## How sign-in works

Passwords are stored as Argon2id (19 MiB, 2 iterations, 1 lane). A session is an opaque id in the `altas_session` cookie: `HttpOnly`, `SameSite=Lax`, and `Secure` when the request is HTTPS. The database stores a SHA-256 of that id, and logout sets `revoked_at` so the cookie cannot be reused. Reset tokens are stored the same way, work once, and a successful reset revokes every session for the account.

## Author

Muhammad Junaid Rehman Siddiqui — [junaidrehman04](https://github.com/junaidrehman04)

MIT License. See [LICENSE](LICENSE). Contributions: [CONTRIBUTING.md](CONTRIBUTING.md).
