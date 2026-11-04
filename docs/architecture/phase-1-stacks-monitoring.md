# Phase 1 — stacks and monitoring

This document locks the shape of Altas before the panel grows past authentication. v0.2.0-dev implements the foundation described under [This release](#this-release). Items under [Later releases](#later-releases) are named so they are not redesigned by accident.

Altas is a control panel for one VPS or one dedicated machine, with a later path to many machines. It reads the kernel instead of parking a second monitoring stack beside it. The resident targets are a panel API around 15–40MB idle and a monitoring agent under 20MB RSS. Those are design limits, not a claim that every build has been measured on every host.

## 1. Control plane

The control plane is the existing Go API.

- Process: `apps/api/cmd/server`
- Router: Chi
- Metadata: SQLite through `modernc.org/sqlite` (pure Go, no CGO)
- Auth: register, login, logout, password reset, httpOnly session cookie. Unchanged by this phase
- First account is the admin. Public registration closes after that account exists

New authenticated routes in this release:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/metrics/host` | CPU, memory, disk, and network summary for the host the API can see |
| `GET` | `/api/stacks` | Registered stack kinds and the project jail root |
| `GET` | `/api/stacks/compose` | Read-only Compose inventory, or a clear disabled / unavailable message |

`/api/health` stays public. Metrics and stacks require a session. Query strings on those routes are ignored. The API does not shell out to collect them.

Operator settings, not request fields:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ALTAS_HOST_PROC` | `/proc` | Proc tree to read. In Compose this is `/host/proc` |
| `ALTAS_HOST_ROOT` | `/` | Prefix for mount points. In Compose this is `/host/root` |
| `ALTAS_PROJECTS_ROOT` | `/var/lib/altas/projects` | Future jail for stack files. Not created or written yet |
| `ALTAS_DOCKER_PROBE` | off | `1`, `true`, `yes`, or `on` enables the read-only inventory |
| `ALTAS_DOCKER_SOCKET` | empty | Optional socket path. Empty tries dockerd, then the rootless user socket |

## 2. Host agent

The host reader is `apps/api/internal/metrics`. The same package backs the API and the one-shot binary `apps/api/cmd/agent`.

On Linux it reads:

- `/proc/stat` for aggregate CPU. Usage is the delta between two samples. The first API response leaves `cpu.usage_percent` null. The agent binary samples twice, 200ms apart, so a single run has a percent
- `/proc/meminfo`. Used memory is `MemTotal - MemAvailable`. `Buffers + Cached` is reported separately. Cache is not presented as a full machine
- `/proc/loadavg` and `/proc/uptime`
- `/proc/net/dev`. Loopback is omitted when another interface exists. Rates use the previous sample
- `/proc/mounts` plus `statfs`. Virtual filesystems such as `tmpfs`, `proc`, and `cgroup` are skipped. `ext4`, `xfs`, `btrfs`, `zfs`, and `overlay` are kept, as are mounts whose device starts with `/dev/`

Non-Linux builds return `source: "fallback"`, the CPU count, and a note. They do not pretend to have read `/proc`. Parser tests run on every OS. The Linux collector is behind a build tag.

There is no Prometheus, node exporter, or other agents process. A background sampler is not started; the API keeps the last sample in memory.

The binary is intentionally small:

```bash
CGO_ENABLED=0 go build -o altas-agent ./apps/api/cmd/agent
./altas-agent
```

A long-running agent, remote hosts, and push/pull between machines are later work. v0.2.0-dev only sees the machine the process is running on. Inside Compose, the read-only `/proc` and `/` mounts make that machine the host rather than the container's own cgroup view.

## 3. Stacks

A stack is a project template that will eventually live in a jailed directory:

```text
/var/lib/altas/projects/<id>/
```

`<id>` is a single segment: lowercase letters, digits, and hyphens, 1–63 characters. The path is resolved through the jail helper. `..`, absolute paths, and symlinks that leave the root are rejected.

The registry is a fixed list in `apps/api/internal/stacks`. Adding a kind means adding one entry. The kinds in this phase:

| Kind | Deployable now |
| --- | --- |
| `docker-compose` | no |
| `laravel` | no |
| `nextjs` | no |

`GET /api/stacks` returns that list and `projects_root`. Nothing is copied onto disk.

### Docker socket probe

`GET /api/stacks/compose` can list projects that are already on the local Docker daemon. It is off by default.

When enabled, the probe:

- Dials a Unix socket. Candidates are the configured path, `/var/run/docker.sock`, `/run/docker.sock`, and `/run/user/<uid>/docker.sock`
- Rejects relative paths, `..`, and non-sockets
- Sends one `GET /containers/json?all=1`. It does not follow redirects and does not accept a URL from the client
- Groups containers by the label `com.docker.compose.project`
- Returns names, a working-directory label when it has no control characters, running counts, and a status of `running`, `partial`, or `exited`

The response never includes the socket path. A missing daemon returns HTTP 200 with `available: false` and a message that Docker is required. The probe does not start, stop, create, or delete containers, and it does not invoke the `docker` CLI.

The socket is root-equivalent on a normal dockerd. Leave the probe off unless the operator mounts a rootless socket or a group-owned socket and accepts that risk. The API process in the production image is uid 65532, so a root-only socket will simply report that Docker is unavailable.

## 4. File manager

Not built yet. The jail it must use is `apps/api/internal/security`.

`OpenJail` refuses the filesystem root and a root that is not an existing directory. `Resolve` then:

- Rejects empty names, NUL, newlines, and `%` so a later percent-decode cannot introduce `..`
- Rejects any `..` segment, even when cleaning it would land back inside
- Cleans the path and requires the result to stay under the resolved root, including the sibling-prefix case (`/data/jail` versus `/data/jail-evil`)
- Walks each existing component. A symlink is evaluated and must still sit under the root
- Allows a missing tail so a future create can name a file that is not there yet

`Open` re-checks the Linux file descriptor through `/proc/self/fd` after the open.

Callers decode paths once, then call `Resolve`. They do not decode again. They do not join user input onto the root with `filepath.Join` on their own.

## 5. Command allowlist

`security.Allowlist` is the only supported way to turn a string into a process for stack actions. `Command` uses `os/exec` with an absolute `LookPath` result. It never calls `sh -c`.

Rules:

- `argv[0]` is a basename on an explicit allowlist. Paths are rejected
- Shells and interpreters are rejected even if someone puts them on the list (`sh`, `bash`, `busybox`, `env`, `sudo`, `python3`, `perl`, `node`, and the others in the denylist)
- Every argument is a single token of at most 256 bytes: letters, digits, and `._+-:@,/%=`
- `; | & $ < >` backticks, quotes, whitespace, globs, and `..` segments fail
- At most 32 arguments

Docker `--format '{{.ID}}'` is intentionally rejected. The Compose probe talks to the socket with HTTP and does not need a format string. A later deploy path that needs richer arguments has to extend the token rules in code review, not pass a shell string.

## 6. Analytics

Shipped after the v0.2.0-dev foundation. The design and the routes are [analytics-metrics.md](analytics-metrics.md).

The rules from this phase still hold:

- Collectors write rollups into the same SQLite file. There is no Prometheus or ClickHouse deployment
- GeoIP comes from an optional local database. The client address is truncated to IPv4 `/24` or IPv6 `/48` before lookup or hashing
- The raw client address is not stored
- Retention is fixed. Older buckets are deleted
- Per-project filters use the project id, not a free-form query

No GeoIP database file is shipped with the panel. The access-log tail is off unless the operator sets a path.

## 7. Malware

`apps/api/internal/malware` is a report-only hook.

- `Scanner` returns findings. It is not given a delete callback
- `New` returns an error if `AutoDelete` is set. There is no flag that turns deletion on
- `Noop` reports nothing
- `ClamAV` can see whether `clamscan` or `clamdscan` is on `PATH`. `Scan` returns "not enabled" and does not execute either binary
- Hash and process anomaly scanners are the same interface (`hash`, `process`, `signature`) and are not implemented

A later integration may shell out to ClamAV only through the command allowlist, and only to print findings. The default remains report-only. Quarantine or delete requires an explicit operator action in a future release, not a scanner default.

## 8. Memory and deploy

Targets:

- Panel API idle: about 15–40MB RSS. SQLite and the session code are the bulk of that. Metrics add a few structs, not a second database
- Monitoring agent: under 20MB RSS. It is the metrics package plus `encoding/json`
- Dependencies added for this phase: none. No Docker client SDK, no Prometheus client, no ClamAV library

Deploy of the panel itself:

- `docker-compose.yml` builds slim images and listens on `127.0.0.1:8088` (web) and `127.0.0.1:8080` (API)
- `docker-compose.dev.yml` runs the Go and Vite dev servers. It is not the production privilege model
- API image: multi-stage, `CGO_ENABLED=0`, Alpine runtime, uid 65532 after the entrypoint chowns `/data`
- Web image: Node build, then nginx. `/api` is proxied to the API container. The SPA falls back to `index.html`
- Host `/proc` and `/` are mounted read-only so disk and CPU numbers describe the machine. Remove those mounts if you only want the container's own view
- The Docker socket is not mounted

`get.altaspanel.tech` is still the planned installer host. The script is not published and the endpoint is not live. Compose and `go run` are the install paths that work today.

```bash
curl -fsSL https://get.altaspanel.tech | sudo bash
```

Auto-update is implemented in [auto-update.md](auto-update.md). It stays off until an admin enables it, and it applies to an already installed binary. The one-liner above is still the install path that is not live yet.

The API container starts as root only long enough to `chown` the data volume, then `exec`s the server as `altas`. Reset URLs are not logged when `ALTAS_ENV=production`.

## This release

- Compose files and Dockerfiles for the panel
- True-state host metrics, API, dashboard section, and the agent binary
- Path jail, command allowlist, and tests that reject traversal and injection
- Stack registry and the optional read-only Compose probe
- Malware hook that cannot auto-delete
- Auto-update for an installed binary, off by default. See [auto-update.md](auto-update.md)
- README status table updated to match the above

## Later releases

- Create, start, and stop containers from the Compose, Laravel, and Next.js templates
- File manager CRUD UI on top of the jail
- A push API for traffic from outside the panel. The access-log tail is the file hook, and it stays off by default
- Running ClamAV or hash/process checks, still without auto-delete
- Multi-server, domains, mail, and the AI Server Doctor
- Marketing site updates, which live in a separate repository
- The `get.altaspanel.tech` installer

## Security defaults

- Session cookie stays `HttpOnly`, `SameSite=Lax`, and `Secure` on HTTPS
- Metrics and stacks are authenticated
- Docker probe is off
- No shell metacharacter is accepted by the allowlist
- No user-supplied filesystem path is read, except through the jail helper, which no route calls yet
- No secrets belong in the repository. `.env` is gitignored. `.env.example` has empty or local defaults only
- Auto-update is off by default and accepts only the pinned GitHub Releases API. See [auto-update.md](auto-update.md)
