# Auto-update

Auto-update installs a newer official build of the panel binary. It is off until an admin turns it on. The official installer is `scripts/install.sh`. `get.altaspanel.tech` proxies that file. The one-liner works after a `v*` GitHub Release exists.

The code lives in `apps/api/internal/update`. Settings are rows in the SQLite `settings` table. Admins open Updates from the command palette (`/updates`).

## What an update is allowed to do

The updater downloads bytes and writes them to the panel binary. It does not run a shell script from the release, and it does not unpack an archive.

It talks only to the GitHub Releases API for one owner and repository. The default is `junaidrehman04/altas-panel`. `ALTAS_UPDATE_OWNER` and `ALTAS_UPDATE_REPO` can retarget that pair, and each value has to be a single repository name. The API origin is `https://api.github.com`. `ALTAS_UPDATE_API_BASE` is ignored in production. When `ALTAS_ENV=development`, a loopback base is accepted so a test can publish a fake release. Any other host is ignored.

Asset URLs inside the release JSON are ignored. The client builds `/repos/{owner}/{repo}/releases/latest` and `/repos/{owner}/{repo}/releases/assets/{id}` itself. A redirect is followed only to `api.github.com`, `github.com`, or the GitHub release CDNs (`objects.githubusercontent.com`, `release-assets.githubusercontent.com`, `github-releases.githubusercontent.com`).

The asset name is `altas-panel-<os>-<arch>` (`altas-panel-windows-<arch>.exe` on Windows). The release must also contain `SHA256SUMS`, `SHA256SUMS.txt`, or `checksums.txt` with a coreutils line for that file. The SHA256 has to match before the file is written. A script, including a file that starts with `#!`, is refused even when the digest matches. The accepted headers are ELF, Mach-O, and MZ.

Ed25519 is implemented and runs when `ALTAS_UPDATE_PUBKEY` is a hex-encoded 32-byte public key. The release must then include `altas-panel-<os>-<arch>.ed25519`, the hex or base64 signature of the raw binary. A project public key is not pinned in this repository yet, so the default gate is the checksum file from the same release. Pinning that key is the next step. Minisign framing is not accepted in this version.

## Apply and rollback

Scheduled checks run only while auto-update is on. The interval is `ALTAS_UPDATE_INTERVAL`, default 12 hours, clamped to 6–24 hours. The loop wakes about once a minute and does not call GitHub until that interval has elapsed. A Check now performed while auto-update is off records the version and does not consume the schedule, so enabling the toggle installs on the next wake.

Check now is an admin action. It is limited to once a minute. When auto-update is on, a verified newer release is installed immediately. When it is off, Check now only records the version. Apply installs that release without requiring the toggle. Apply has the same one-minute floor.

The current build version is `apps/api/internal/version.Version` (`v0.3.0` unless the build sets `-X github.com/junaidrehman04/altas/apps/api/internal/version.Version=...`). `GET /api/version` returns it. The same value is on `GET /api/health`.

Install writes `binary.next`, copies the current file to `binary.bak`, then renames the new file onto the current path. The previous bytes stay in `.bak`. Rollback copies `.bak` back and restarts. Rollback does not delete `.bak`.

`ALTAS_UPDATE_BINARY` is the file that gets replaced. When it is empty, the updater uses its own executable path. `ALTAS_UPDATE_RESTART=systemd` (the default) restarts `ALTAS_UPDATE_UNIT` (default `altas-panel`). The restart is started outside the service, because a synchronous `systemctl restart` from inside the unit waits for this process to exit. The helper runs `systemctl restart` with an absolute `systemctl` path and a single unit argument. The unit must be `altas-panel`. If the unit does not stay active, the helper copies `.bak` back and restarts again. `ALTAS_UPDATE_RESTART=none` replaces the file and does not restart it. If the in-process restart call fails before the helper starts, the previous binary is restored as well.

A host install that should update itself looks like this:

```ini
[Service]
User=altas
ExecStart=/usr/local/lib/altas/altas-panel
Environment=ALTAS_UPDATE_BINARY=/usr/local/lib/altas/altas-panel
Environment=ALTAS_UPDATE_UNIT=altas-panel
Environment=ALTAS_UPDATE_RESTART=systemd
Restart=on-failure
```

`scripts/install.sh` writes this unit. The binary directory is owned by `altas`, so the process can replace the file. A polkit rule and a sudoers drop-in allow that user to restart only `altas-panel`. Release assets are the raw binaries `altas-panel-linux-amd64` and `altas-panel-linux-arm64` (what auto-update installs) plus `altas-panel-linux-<arch>.tar.gz` (what the installer unpacks). `SHA256SUMS` lists both.

Docker Compose is a different path. `docker compose restart api` restarts the existing image. It does not change that image. Leave auto-update off for a Compose install. Replacing the binary inside the container is not a durable update.

## Admin API

All of these require a session for an admin. Anyone else gets 403. A missing session gets 401.

| Method | Path | Effect |
| --- | --- | --- |
| `GET` | `/api/settings` | Toggle, last check, current version, latest release, whether `.bak` exists |
| `PATCH` | `/api/settings` | `{"auto_update": true}` or `false` |
| `POST` | `/api/settings/updates/check` | Check now. Installs when the toggle is on and the release verifies |
| `POST` | `/api/settings/updates/apply` | Verify and install a newer release, even when the toggle is off |
| `POST` | `/api/settings/updates/rollback` | Restore `.bak` and restart |

A checksum or signature failure responds with 409 and does not replace the binary. If the repository has no published release, Check now reports that instead of installing anything.
