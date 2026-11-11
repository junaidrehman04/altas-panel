#!/usr/bin/env bash
# Altas Panel installer.
#
# Official command, once a GitHub Release exists and get.altaspanel.tech
# proxies this file:
#   curl -fsSL https://get.altaspanel.tech | sudo bash
#
# Stable path in this repository: scripts/install.sh
# Raw file: https://raw.githubusercontent.com/junaidrehman04/altas-panel/main/scripts/install.sh
#
# The script downloads a release tarball from GitHub, checks SHA256SUMS, and
# installs a systemd service. It does not pipe a second script into a shell.
# Re-running upgrades the binary and keeps the data directory and env file.
#
#   ALTAS_VERSION=v0.3.0
#   ALTAS_DOWNLOAD_BASE   default GitHub release download URL for that version
#   ALTAS_RELEASE_API      default GitHub latest-release API
#   sudo bash scripts/install.sh --uninstall
#   sudo bash scripts/install.sh --uninstall --purge

set -euo pipefail

OWNER="junaidrehman04"
REPO="altas-panel"
SERVICE_USER="altas"
UNIT_NAME="altas-panel"
LIB_DIR="/usr/local/lib/altas"
BIN_PATH="${LIB_DIR}/altas-panel"
DATA_DIR="/var/lib/altas"
ENV_DIR="/etc/altas"
ENV_FILE="${ENV_DIR}/altas.env"
UNIT_PATH="/etc/systemd/system/${UNIT_NAME}.service"
SUDOERS_PATH="/etc/sudoers.d/altas-panel"
POLKIT_PATH="/etc/polkit-1/rules.d/50-altas-panel.rules"

ACTION="install"
PURGE=0
VERSION="${ALTAS_VERSION:-}"

usage() {
  cat <<'EOF'
Usage: install.sh [--version vX.Y.Z] [--uninstall] [--purge]

  Installs or upgrades Altas Panel from GitHub Releases.
  --uninstall removes the service and binary. Data in /var/lib/altas is kept.
  --purge     with --uninstall, also removes /var/lib/altas, /etc/altas, and the altas user.

Environment:
  ALTAS_VERSION         release tag, for example v0.3.0. Empty uses the latest release.
  ALTAS_DOWNLOAD_BASE   release download prefix. Default is the GitHub release for ALTAS_VERSION.
  ALTAS_RELEASE_API     latest-release URL. Default is the GitHub API for this repository.
EOF
}

die() {
  echo "altas-install: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --uninstall)
      ACTION="uninstall"
      shift
      ;;
    --purge)
      PURGE=1
      shift
      ;;
    --version)
      [ $# -ge 2 ] || die "--version needs a tag"
      VERSION=$2
      shift 2
      ;;
    --version=*)
      VERSION=${1#--version=}
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

if [ "$(id -u)" -ne 0 ]; then
  die "run as root, for example: curl -fsSL https://get.altaspanel.tech | sudo bash"
fi

assert_url() {
  case "$1" in
    https://github.com/*|https://api.github.com/*|https://objects.githubusercontent.com/*|https://release-assets.githubusercontent.com/*|https://github-releases.githubusercontent.com/*)
      ;;
    http://127.0.0.1:*|http://127.0.0.1/*|http://localhost:*|http://localhost/*|https://127.0.0.1:*|https://127.0.0.1/*|https://localhost:*|https://localhost/*)
      ;;
    *)
      die "refusing download host: $1"
      ;;
  esac
}

fetch() {
  url=$1
  dest=$2
  assert_url "$url"
  case "$url" in
    https://*)
      curl --proto '=https' --tlsv1.2 -fsSL --retry 3 --retry-delay 2 --max-time 180 -o "$dest" "$url"
      ;;
    http://127.0.0.1*|http://localhost*)
      curl --proto '=http' -fsSL --retry 3 --retry-delay 1 --max-time 60 -o "$dest" "$url"
      ;;
    *)
      die "refusing URL: $url"
      ;;
  esac
}

valid_version() {
  printf '%s' "$1" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.]+)?$'
}

install_deps() {
  if need_cmd curl && need_cmd tar && need_cmd sha256sum && need_cmd install && { need_cmd useradd || need_cmd adduser; }; then
    return 0
  fi
  if need_cmd apt-get; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y ca-certificates curl tar coreutils passwd
  elif need_cmd dnf; then
    dnf install -y ca-certificates curl tar coreutils shadow-utils
  elif need_cmd yum; then
    yum install -y ca-certificates curl tar coreutils shadow-utils
  elif need_cmd apk; then
    apk add --no-cache ca-certificates curl tar coreutils shadow
  elif need_cmd pacman; then
    pacman -Sy --noconfirm --needed ca-certificates curl tar coreutils shadow
  else
    die "install curl, tar, and sha256sum, then re-run"
  fi
}

hash_file() {
  if need_cmd sha256sum; then
    sha256sum "$1" | awk '{print tolower($1)}'
    return 0
  fi
  if need_cmd openssl; then
    openssl dgst -sha256 "$1" | awk '{print tolower($NF)}'
    return 0
  fi
  die "sha256sum is required"
}

arch_name() {
  case "$(uname -m)" in
    x86_64|amd64) printf '%s' amd64 ;;
    aarch64|arm64) printf '%s' arm64 ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

ensure_user() {
  if getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
    return 0
  fi
  nologin=/usr/sbin/nologin
  if [ ! -x "$nologin" ]; then
    nologin=/sbin/nologin
  fi
  if need_cmd useradd; then
    useradd --system --user-group --home-dir "$DATA_DIR" --create-home --shell "$nologin" "$SERVICE_USER"
    return 0
  fi
  if need_cmd adduser; then
    adduser -S -D -h "$DATA_DIR" -s "$nologin" "$SERVICE_USER"
    return 0
  fi
  die "could not create the altas user"
}

ensure_key() {
  key=$1
  value=$2
  if [ ! -f "$ENV_FILE" ]; then
    die "env file is missing"
  fi
  if grep -q "^${key}=" "$ENV_FILE"; then
    return 0
  fi
  printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
}

write_env() {
  install -d -m 0755 "$ENV_DIR"
  if [ -f "$ENV_FILE" ]; then
    ensure_key ALTAS_ADDR ":8080"
    ensure_key ALTAS_DB "${DATA_DIR}/altas.db"
    ensure_key ALTAS_ENV production
    ensure_key ALTAS_WEB_ORIGIN "http://127.0.0.1:8080"
    ensure_key ALTAS_HOST_PROC /proc
    ensure_key ALTAS_HOST_ROOT /
    ensure_key ALTAS_UPDATE_BINARY "$BIN_PATH"
    ensure_key ALTAS_UPDATE_UNIT "$UNIT_NAME"
    ensure_key ALTAS_UPDATE_RESTART systemd
    ensure_key ALTAS_UPDATE_INTERVAL 12h
    ensure_key MAXMIND_LICENSE_KEY ""
    ensure_key ALTAS_GEOIP_DB "${DATA_DIR}/GeoLite2-Country.mmdb"
    chown root:"$SERVICE_USER" "$ENV_FILE" || true
    chmod 0640 "$ENV_FILE"
    return 0
  fi
  umask 077
  cat > "$ENV_FILE" <<EOF
ALTAS_ADDR=:8080
ALTAS_DB=${DATA_DIR}/altas.db
ALTAS_ENV=production
ALTAS_WEB_ORIGIN=http://127.0.0.1:8080
ALTAS_HOST_PROC=/proc
ALTAS_HOST_ROOT=/
ALTAS_UPDATE_BINARY=${BIN_PATH}
ALTAS_UPDATE_UNIT=${UNIT_NAME}
ALTAS_UPDATE_RESTART=systemd
ALTAS_UPDATE_INTERVAL=12h
MAXMIND_LICENSE_KEY=
ALTAS_GEOIP_DB=${DATA_DIR}/GeoLite2-Country.mmdb
EOF
  chown root:"$SERVICE_USER" "$ENV_FILE"
  chmod 0640 "$ENV_FILE"
}

write_unit() {
  cat > "$UNIT_PATH" <<EOF
[Unit]
Description=Altas Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${DATA_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${BIN_PATH}
Restart=on-failure
RestartSec=2
UMask=0027
NoNewPrivileges=false
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=${DATA_DIR} ${LIB_DIR}
LockPersonality=true

[Install]
WantedBy=multi-user.target
EOF
  chmod 0644 "$UNIT_PATH"
}

write_sudoers() {
  systemctl_bin=$(command -v systemctl)
  case "$systemctl_bin" in
    /*) ;;
    *) die "systemctl is not an absolute path" ;;
  esac
  tmp=$(mktemp)
  cat > "$tmp" <<EOF
# Altas Panel may restart only its own unit.
Defaults:${SERVICE_USER} !requiretty
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} restart ${UNIT_NAME}, ${systemctl_bin} restart ${UNIT_NAME}.service, ${systemctl_bin} is-active ${UNIT_NAME}, ${systemctl_bin} is-active ${UNIT_NAME}.service, ${systemctl_bin} is-active --quiet ${UNIT_NAME}, ${systemctl_bin} is-active --quiet ${UNIT_NAME}.service
EOF
  if need_cmd visudo; then
    visudo -cf "$tmp" >/dev/null
  fi
  install -d -m 0750 /etc/sudoers.d
  install -m 0440 "$tmp" "$SUDOERS_PATH"
  rm -f "$tmp"
}

write_polkit() {
  if [ ! -d /etc/polkit-1/rules.d ] && [ ! -d /usr/share/polkit-1 ]; then
    return 0
  fi
  install -d -m 0755 /etc/polkit-1/rules.d
  cat > "$POLKIT_PATH" <<'EOF'
polkit.addRule(function(action, subject) {
  if (subject.user !== "altas") {
    return;
  }
  if (action.id !== "org.freedesktop.systemd1.manage-units") {
    return;
  }
  var unit = action.lookup("unit");
  var verb = action.lookup("verb");
  if ((unit === "altas-panel.service" || unit === "altas-panel") &&
      (verb === "restart" || verb === "start" || verb === "stop")) {
    return polkit.Result.YES;
  }
});
EOF
  chmod 0644 "$POLKIT_PATH"
}

resolve_version() {
  if [ -n "$VERSION" ]; then
    valid_version "$VERSION" || die "refusing version: $VERSION"
    return 0
  fi
  api="${ALTAS_RELEASE_API:-https://api.github.com/repos/${OWNER}/${REPO}/releases/latest}"
  body=$(mktemp)
  fetch "$api" "$body"
  VERSION=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$body" | head -n 1)
  rm -f "$body"
  if [ -z "$VERSION" ]; then
    die "no GitHub Release is published yet. The one-liner works after the first v* release."
  fi
  valid_version "$VERSION" || die "refusing version from the release API: $VERSION"
}

download_verified() {
  arch=$(arch_name)
  asset="altas-panel-linux-${arch}.tar.gz"
  base="${ALTAS_DOWNLOAD_BASE:-https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}}"
  base=${base%/}
  work=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf $(printf '%q' "$work")" RETURN
  fetch "${base}/SHA256SUMS" "${work}/SHA256SUMS"
  fetch "${base}/${asset}" "${work}/${asset}"
  # mawk does not support {64}, so the length check stays in awk and grep
  # validates the hex. Both are available on the distros this script supports.
  digest=$(awk -v name="$asset" '
    {
      n = $2
      sub(/^\*/, "", n)
      if (n == name && length($1) == 64) { print tolower($1); exit }
    }
  ' "${work}/SHA256SUMS")
  [ -n "$digest" ] || die "SHA256SUMS has no entry for ${asset}"
  printf '%s\n' "$digest" | grep -Eq '^[0-9a-f]{64}$' || die "SHA256SUMS entry for ${asset} is not a SHA256"
  got=$(hash_file "${work}/${asset}")
  if [ "$got" != "$digest" ]; then
    die "SHA256 mismatch for ${asset}"
  fi
  if ! tar -tzf "${work}/${asset}" > "${work}/list"; then
    die "release archive is not a tarball"
  fi
  count=0
  while IFS= read -r entry; do
    [ -n "$entry" ] || continue
    count=$((count + 1))
    case "$entry" in
      altas-panel|./altas-panel) ;;
      *) die "unexpected archive entry: ${entry}" ;;
    esac
  done < "${work}/list"
  [ "$count" -eq 1 ] || die "release archive must contain only altas-panel"
  mkdir -p "${work}/extract"
  tar -xzf "${work}/${asset}" -C "${work}/extract"
  src="${work}/extract/altas-panel"
  if [ -L "$src" ] || [ ! -f "$src" ]; then
    die "archive binary is not a regular file"
  fi
  magic=$(od -An -t x1 -N 4 "$src" | tr -d ' \n')
  if [ "$magic" != "7f454c46" ]; then
    die "archive does not contain an ELF binary"
  fi
  install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0755 "$LIB_DIR"
  if [ -f "$BIN_PATH" ]; then
    cp -a "$BIN_PATH" "${BIN_PATH}.bak"
  fi
  install -m 0755 "$src" "$BIN_PATH"
  chown root:"$SERVICE_USER" "$BIN_PATH" || true
  rm -rf "$work"
  trap - RETURN
}

start_service() {
  systemctl daemon-reload
  systemctl enable "$UNIT_NAME" >/dev/null
  systemctl restart "$UNIT_NAME"
  if ! systemctl is-active --quiet "$UNIT_NAME"; then
    die "altas-panel did not stay active. See: journalctl -u ${UNIT_NAME} -n 80 --no-pager"
  fi
}

print_next() {
  port=8080
  if [ -f "$ENV_FILE" ]; then
    configured=$(sed -n 's/^ALTAS_ADDR=//p' "$ENV_FILE" | head -n 1 || true)
    case "$configured" in
      *:*) port=${configured##*:} ;;
    esac
  fi
  ip=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
  if [ -z "${ip:-}" ]; then
    ip=127.0.0.1
  fi
  echo
  echo "Altas Panel ${VERSION} is running."
  echo "Open http://${ip}:${port}"
  echo "The first account you register becomes the panel admin."
  echo "Auto-update stays off until that admin enables it from the command palette."
}

uninstall() {
  if need_cmd systemctl; then
    systemctl disable --now "$UNIT_NAME" >/dev/null 2>&1 || true
  fi
  rm -f "$UNIT_PATH" "$SUDOERS_PATH" "$POLKIT_PATH"
  rm -f "$BIN_PATH" "${BIN_PATH}.bak" "${BIN_PATH}.next" "${BIN_PATH}.restart"
  if need_cmd systemctl; then
    systemctl daemon-reload || true
  fi
  if [ "$PURGE" -eq 1 ]; then
    rm -rf "$DATA_DIR" "$ENV_DIR"
    if need_cmd userdel && getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
      userdel "$SERVICE_USER" || true
    fi
    if need_cmd groupdel && getent group "$SERVICE_USER" >/dev/null 2>&1; then
      groupdel "$SERVICE_USER" || true
    fi
    echo "Altas Panel was removed, including ${DATA_DIR}."
    return 0
  fi
  echo "Altas Panel was removed. Data remains in ${DATA_DIR}. Re-run with --purge to delete it."
}

if [ "$ACTION" = "uninstall" ]; then
  uninstall
  exit 0
fi

[ "$(uname -s)" = "Linux" ] || die "Altas Panel installs on Linux"
need_cmd systemctl || die "systemd is required"
install_deps
ensure_user
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$DATA_DIR"
resolve_version
write_env
download_verified
write_unit
write_sudoers
write_polkit
start_service
print_next
