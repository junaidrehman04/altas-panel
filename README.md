# Altas Panel

[![GitHub stars](https://img.shields.io/github/stars/junaidrehman04/altas-panel?style=flat-square)](https://github.com/junaidrehman04/altas-panel/stargazers)
[![MIT license](https://img.shields.io/github/license/junaidrehman04/altas-panel?style=flat-square)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/junaidrehman04/altas-panel?style=flat-square)](go.mod)
[![GitHub issues](https://img.shields.io/github/issues/junaidrehman04/altas-panel?style=flat-square)](https://github.com/junaidrehman04/altas-panel/issues)

Ultra-lightweight open-source control panel for a 1GB VPS and a 64-core dedicated box, from the same plane. Product home: [altaspanel.tech](https://altaspanel.tech).

Altas reads true server state from `/proc`. Cache is not reported as memory full, and the panel itself is built to stay under 20MB resident, about 18MB, so the host keeps its RAM for the workloads on it. The same control plane is the path to multi-server, multi-domain, and multi-email operations, with an AI Server Doctor on top of that telemetry.

**v0.1.0** is the door, not the building. The first account becomes the admin, public registration closes, sessions and password reset work, and the signed-in shell is empty on purpose.

## How Altas works

![How Altas works: install, add a server, manage domains and mail, then monitor with true-state metrics and AI Server Doctor](docs/images/workflow-how-altas-works.jpg)

1. **Install** — `curl -fsSL https://get.altaspanel.tech | sudo bash`
2. **Add server** — Connect VPS or dedicated hosts into one quiet panel
3. **Manage domains & mail** — SSL, multi-domain, multi-mailbox from one place
4. **Monitor** — True-State metrics + AI Server Doctor

`get.altaspanel.tech` is the planned install host. The script is not live yet. Until it is, use the [quick start](#quick-start) below.

## Install

### Planned one-liner

`get.altaspanel.tech` is the install host target. It is not serving the installer yet.

```bash
curl -fsSL https://get.altaspanel.tech | sudo bash
```

A published container image is not part of this release. Until the one-liner is live, install from source.

### From source

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

## Features

| Capability | Status |
| --- | --- |
| Accounts, sessions, password reset, empty signed-in shell | Implemented |
| First admin, then closed public registration | Implemented |
| Live server stacks (Docker, Laravel, Next) | Coming |
| Analytics and monitoring | Coming |
| True-state telemetry from `/proc` | Coming |
| Multi-server | Coming |
| Multi-domain and SSL | Coming |
| Multi-email / mail suite (DKIM, SPF, DMARC) | Coming |
| AI Server Doctor | Coming |

## Screenshots

Panel mockups for the product Altas is building. The app in this repository is still the v0.1.0 auth shell. [altaspanel.tech](https://altaspanel.tech) will carry the public site; a preview build is at [altas-marketing.vercel.app](https://altas-marketing.vercel.app).

### Overview

![Altas overview in light mode, with CPU, RAM, disk, network, and a server list](docs/images/dashboard-light.jpg)

![Altas overview in dark mode, with true uptime, true-state metrics, and a multi-server table](docs/images/dashboard-dark.jpg)

### Server

![Server detail in light mode](docs/images/server-detail-light.jpg)

![Server detail in dark mode](docs/images/server-detail-dark.jpg)

### Domains

![Domains view in light mode](docs/images/domains-light.jpg)

### Mail

![Mail view in light mode](docs/images/mail-light.jpg)

## Quick start

Follow [From source](#from-source). On a fresh database the register screen creates the admin account and signs you in. Sign out from the top bar. After that account exists, the register screen closes and unauthenticated visits to the overview return to sign-in.

### Password reset without mail

v0.1.0 does not send email. With `ALTAS_ENV=development` (the default), a forgot-password request stores a single-use token and prints the full reset URL to the API stdout:

```text
reset url: http://127.0.0.1:5173/reset-password?token=...
```

Open that URL, set a new password, and sign in again. Existing sessions for the account are revoked. Set `ALTAS_ENV=production` when you do not want reset URLs in the log.

## Auth model

Most panels do not leave public registration open. Altas does not either, after the box has an owner.

- The first successful `POST /api/auth/register` creates the panel admin (`is_admin = 1`) and a session.
- If any user already exists, public register responds with `403` and `Public registration is closed. The first account is the panel admin; further accounts will be invite-only.`
- `GET /api/auth/registration` returns `{"open": true}` only while the user table is empty. The web UI hides the create-account form once it is closed.
- Later accounts are invite-only. Admin-created users are not in this release.
- Databases created before this gate gain an `is_admin` column on the next API start. The earliest account is marked admin, and public registration is closed.
- Login, logout, session cookies, and password reset are unchanged. Reset still revokes every session for that account.

## Architecture

True state means reading the kernel, not a second runtime parked beside it. `/proc` separates application RAM from buffer and cache, which is the difference between "the box is full" and "the page cache is doing its job." The resident target for the panel stays under 20MB, about 18MB. Multi-server comes later: many hosts, one plane, still that small. v0.1.0 does not read `/proc` yet.

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
apps/api/internal/auth       Argon2id passwords, sessions, reset tokens, first-admin gate
apps/api/internal/httpapi    Chi routes
apps/api/internal/db         SQLite schema (pure Go driver)
apps/web                     Vite + React + Tailwind shell
docs/images                  Panel mockups used in this README
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

## License

MIT. See [LICENSE](LICENSE).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
