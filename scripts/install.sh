#!/usr/bin/env bash
set -euo pipefail

REPO="mtsandeep/l8bin-monitor"
BINARY="litebin-monitor"
DEST="/usr/local/bin/${BINARY}"
SERVICE="/etc/systemd/system/${BINARY}.service"

echo "==> ${BINARY} install/update script"

# Detect architecture
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
if [[ -z "$ARCH" ]]; then
    echo "Error: unsupported architecture $(uname -m)"
    exit 1
fi

ASSET="${BINARY}-linux-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

# Stop service if running
if systemctl is-active --quiet "${BINARY}" 2>/dev/null; then
    echo "-- Stopping running service..."
    systemctl stop "${BINARY}"
fi

# Download binary
echo "-- Downloading ${ASSET}..."
curl -fsSL "${URL}" -o "/tmp/${BINARY}"
mv "/tmp/${BINARY}" "${DEST}"
chmod +x "${DEST}"

# Download and install service file
echo "-- Installing systemd service..."
curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/${BINARY}.service" \
    -o "/tmp/${BINARY}.service"
mv "/tmp/${BINARY}.service" "${SERVICE}"
systemctl daemon-reload
systemctl enable "${BINARY}"
systemctl start "${BINARY}"

echo "==> Done! $( "${DEST}" -v )"
