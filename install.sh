#!/bin/sh
set -eu

REPOSITORY="0jc1/tsync"
SCOPE="user"
CONFIG_PATH="${TSYNC_CONFIG:-}"
UNINSTALL=0

usage() {
  cat <<'EOF'
Usage: install.sh [--scope user|system] [--config PATH] [--uninstall]

Installs tsync and configures it to start automatically. A valid config file
must exist before installation.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --scope)
      SCOPE="${2:-}"
      shift 2
      ;;
    --config)
      CONFIG_PATH="${2:-}"
      shift 2
      ;;
    --uninstall)
      UNINSTALL=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ "$SCOPE" != "user" ] && [ "$SCOPE" != "system" ]; then
  echo "--scope must be user or system" >&2
  exit 1
fi
if [ "$SCOPE" = "system" ] && [ "$(id -u)" -ne 0 ]; then
  echo "System installation must be run as root." >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin|linux) ;;
  *)
    echo "Unsupported operating system: $OS" >&2
    exit 1
    ;;
esac

if [ "$SCOPE" = "system" ]; then
  BINARY_PATH="/usr/local/bin/tsync"
  : "${CONFIG_PATH:=/etc/tsync/config.json}"
else
  BINARY_PATH="$HOME/.local/bin/tsync"
  : "${CONFIG_PATH:=$HOME/.config/tsync/config.json}"
fi

uninstall_service() {
  if [ "$OS" = "darwin" ]; then
    if [ "$SCOPE" = "system" ]; then
      SERVICE_PATH="/Library/LaunchDaemons/com.tsync.agent.plist"
      launchctl bootout system "$SERVICE_PATH" 2>/dev/null || true
    else
      SERVICE_PATH="$HOME/Library/LaunchAgents/com.tsync.agent.plist"
      launchctl bootout "gui/$(id -u)" "$SERVICE_PATH" 2>/dev/null || true
    fi
    rm -f "$SERVICE_PATH"
  elif [ "$SCOPE" = "system" ]; then
    systemctl disable --now tsync.service 2>/dev/null || true
    rm -f /etc/systemd/system/tsync.service
    systemctl daemon-reload
  else
    systemctl --user disable --now tsync.service 2>/dev/null || true
    rm -f "$HOME/.config/systemd/user/tsync.service"
    systemctl --user daemon-reload
  fi
}

if [ "$UNINSTALL" -eq 1 ]; then
  uninstall_service
  rm -f "$BINARY_PATH"
  echo "tsync uninstalled; configuration was preserved at $CONFIG_PATH"
  exit 0
fi

if [ ! -f "$CONFIG_PATH" ]; then
  echo "Create the configuration before installing: $CONFIG_PATH" >&2
  echo "See https://github.com/$REPOSITORY#configuration" >&2
  exit 1
fi

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT INT TERM

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

ARCHIVE="tsync_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPOSITORY/releases/latest/download"
curl -fsSL "$BASE_URL/$ARCHIVE" -o "$TMP_ROOT/$ARCHIVE"
curl -fsSL "$BASE_URL/checksums.txt" -o "$TMP_ROOT/checksums.txt"
awk -v file="$ARCHIVE" '$2 == file || $2 == "*" file { print }' \
  "$TMP_ROOT/checksums.txt" > "$TMP_ROOT/selected-checksum.txt"
if [ ! -s "$TMP_ROOT/selected-checksum.txt" ]; then
  echo "No checksum found for $ARCHIVE" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$TMP_ROOT" && sha256sum -c selected-checksum.txt)
else
  (cd "$TMP_ROOT" && shasum -a 256 -c selected-checksum.txt)
fi
tar -xzf "$TMP_ROOT/$ARCHIVE" -C "$TMP_ROOT"

"$TMP_ROOT/tsync" --config "$CONFIG_PATH" validate
mkdir -p "$(dirname "$BINARY_PATH")"
install -m 0755 "$TMP_ROOT/tsync" "$BINARY_PATH"

render_template() {
  TEMPLATE="$1"
  OUTPUT="$2"
  LOG_DIR="$3"
  sed \
    -e "s|@BINARY@|$BINARY_PATH|g" \
    -e "s|@CONFIG@|$CONFIG_PATH|g" \
    -e "s|@LOG_DIR@|$LOG_DIR|g" \
    "$TEMPLATE" > "$OUTPUT"
}

if [ "$OS" = "darwin" ]; then
  if [ "$SCOPE" = "system" ]; then
    SERVICE_PATH="/Library/LaunchDaemons/com.tsync.agent.plist"
    LOG_DIR="/var/log"
    mkdir -p "$LOG_DIR"
    render_template "$TMP_ROOT/packaging/com.tsync.agent.plist.in" "$SERVICE_PATH" "$LOG_DIR"
    chown root:wheel "$SERVICE_PATH"
    launchctl bootout system "$SERVICE_PATH" 2>/dev/null || true
    launchctl bootstrap system "$SERVICE_PATH"
  else
    SERVICE_PATH="$HOME/Library/LaunchAgents/com.tsync.agent.plist"
    LOG_DIR="$HOME/Library/Logs"
    mkdir -p "$(dirname "$SERVICE_PATH")" "$LOG_DIR"
    render_template "$TMP_ROOT/packaging/com.tsync.agent.plist.in" "$SERVICE_PATH" "$LOG_DIR"
    launchctl bootout "gui/$(id -u)" "$SERVICE_PATH" 2>/dev/null || true
    launchctl bootstrap "gui/$(id -u)" "$SERVICE_PATH"
  fi
elif [ "$SCOPE" = "system" ]; then
  SERVICE_PATH="/etc/systemd/system/tsync.service"
  render_template "$TMP_ROOT/packaging/tsync-system.service.in" "$SERVICE_PATH" ""
  systemctl daemon-reload
  systemctl enable --now tsync.service
else
  SERVICE_PATH="$HOME/.config/systemd/user/tsync.service"
  mkdir -p "$(dirname "$SERVICE_PATH")"
  render_template "$TMP_ROOT/packaging/tsync-user.service.in" "$SERVICE_PATH" ""
  systemctl --user daemon-reload
  systemctl --user enable --now tsync.service
fi

echo "tsync installed and started"
echo "binary: $BINARY_PATH"
echo "config: $CONFIG_PATH"
