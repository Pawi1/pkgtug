#!/bin/sh
# Bootstrap pkgtug — downloads a temporary binary, then uses it to install
# itself properly (tracked in state, auto-updatable).
#
# Usage:
#   curl -fsSL https://pawi1.github.io/pkgtug/install.sh | sh
#   wget -qO- https://pawi1.github.io/pkgtug/install.sh | sh
set -e

SERVER="https://tug.rutkow.eu"

# ── Platform detection ────────────────────────────────────────────────────────

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)        ARCH="x64"   ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l)        ARCH="arm"   ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux)  PLATFORM="linux-$ARCH"  ;;
  darwin) PLATFORM="darwin-$ARCH" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

URL="$SERVER/tug/repo/pkgtug/binaries/latest/$PLATFORM/pkgtug"

# ── Privilege helper ──────────────────────────────────────────────────────────

# pkgtug stores config in /etc/pkgtug/ and state in /var/lib/pkgtug/ by
# default — both require root. Use a privilege escalation tool when not root.
SUDO=""
if [ "$(id -u)" != "0" ]; then
  if   command -v sudo  >/dev/null 2>&1; then SUDO="sudo"
  elif command -v doas  >/dev/null 2>&1; then SUDO="doas"
  elif command -v run0  >/dev/null 2>&1; then SUDO="run0"
  else
    echo "Error: run as root or install sudo / doas / run0" >&2
    exit 1
  fi
fi

# ── Download bootstrap binary ─────────────────────────────────────────────────

echo "Downloading pkgtug bootstrap ($PLATFORM)..."

TMP=$(mktemp /tmp/pkgtug-bootstrap-XXXXXX)
trap 'rm -f "$TMP"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$TMP"
elif command -v wget >/dev/null 2>&1; then
  wget -q "$URL" -O "$TMP"
else
  echo "Error: curl or wget is required" >&2
  exit 1
fi

chmod +x "$TMP"

# ── Install pkgtug via pkgtug ─────────────────────────────────────────────────

echo "Adding remote '$SERVER'..."
$SUDO "$TMP" remote add main "$SERVER"

echo ""
echo "Installing pkgtug (you will be prompted for install options)..."
echo ""

# Redirect stdin from /dev/tty so interactive prompts work even when this
# script is piped through curl | sh.
$SUDO "$TMP" install main:pkgtug/pkgtug < /dev/tty
