#!/usr/bin/env bash
set -euo pipefail

APP_USER="${APP_USER:-printnest}"
APP_DIR="${APP_DIR:-/opt/printnest}"
DATA_DIR="${DATA_DIR:-/var/lib/printnest}"
REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

echo "==> PrintNest native LXC install (no Docker, no Caddy)"
echo "    APP_DIR=$APP_DIR DATA_DIR=$DATA_DIR USER=$APP_USER"

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root" >&2; exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y cups hplip cups-client libheif-examples imagemagick golang-go nodejs npm nginx

id -u "$APP_USER" >/dev/null 2>&1 || useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$APP_USER"
usermod -aG lpadmin "$APP_USER" || true

mkdir -p "$APP_DIR" "$DATA_DIR/uploads"
cp -r "$REPO_DIR/backend" "$REPO_DIR/frontend" "$APP_DIR/"
chown -R "$APP_USER":"$APP_USER" "$DATA_DIR"
chown -R root:root "$APP_DIR"

echo "==> Building frontend"
(cd "$APP_DIR/frontend" && npm ci --no-audit --no-fund && npm run build)
mkdir -p "$APP_DIR/frontend/dist"
cp "$APP_DIR/frontend/manifest.webmanifest" "$APP_DIR/frontend/icon.svg" "$APP_DIR/frontend/sw.js" "$APP_DIR/frontend/dist/"

echo "==> Building backend"
(cd "$APP_DIR/backend" && go build -o /usr/local/bin/printnest .)
chmod +x /usr/local/bin/printnest

if [ ! -f "$DATA_DIR/config.env" ]; then
  cp "$REPO_DIR/deploy/lxc/config.env.example" "$DATA_DIR/config.env"
  echo "    EDIT $DATA_DIR/config.env and set PRINTNEST_PASSWORD, then: systemctl restart printnest"
fi
chown "$APP_USER":"$APP_USER" "$DATA_DIR/config.env" 2>/dev/null || true
chmod 600 "$DATA_DIR/config.env" 2>/dev/null || true

cp "$REPO_DIR/deploy/lxc/printnest.service" /etc/systemd/system/printnest.service
systemctl daemon-reload
systemctl enable --now cups
systemctl enable printnest || true

echo ""
echo "==> Next steps"
echo "    1. Edit $DATA_DIR/config.env (PRINTNEST_PASSWORD required on first boot)"
echo "    2. systemctl restart printnest && systemctl status printnest"
echo "    3. Add printer: lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere"
echo "       (fallback: hp-setup / hplip PPD if driverless fails)"
echo "    4. Point your existing nginx HTTPS vhost at http://127.0.0.1:8080 (see nginx snippet below)"
