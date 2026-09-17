#!/bin/sh
# PrintNest native Alpine LXC install (no Docker, no systemd).
# Debian/Ubuntu equivalent: deploy/lxc/install.sh
# Target: Alpine 3.20+ Proxmox LXC with OpenRC.
#
# Env overrides:
#   APP_USER APP_DIR DATA_DIR INSTALL_HPLIP
# Examples:
#   sh deploy/lxc/install-alpine.sh
#   INSTALL_HPLIP=0 sh deploy/lxc/install-alpine.sh   # driverless 'everywhere' only, smaller
set -eu

APP_USER="${APP_USER:-printnest}"
APP_DIR="${APP_DIR:-/opt/printnest}"
DATA_DIR="${DATA_DIR:-/var/lib/printnest}"
INSTALL_HPLIP="${INSTALL_HPLIP:-1}"
REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

echo "==> PrintNest native Alpine LXC install (no Docker, no systemd)"
echo "    APP_DIR=$APP_DIR DATA_DIR=$DATA_DIR USER=$APP_USER HPLIP=$INSTALL_HPLIP"

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root" >&2; exit 1
fi
if [ ! -f /etc/alpine-release ]; then
  echo "warning: /etc/alpine-release not found; continuing anyway" >&2
fi

# hplip + libheif-tools + go live in community; enable it if commented out.
if grep -q '^[[:space:]]*#[[:space:]]*https\?://[^[:space:]]*/community[[:space:]]*$' /etc/apk/repositories 2>/dev/null; then
  echo "==> Enabling community repository"
  sed -i 's|^[[:space:]]*#[[:space:]]*\(https\?://[^[:space:]]*/community[[:space:]]*\)$|\1|' /etc/apk/repositories
fi

echo "==> Installing packages"
apk update
# Package map vs Debian install.sh:
#   cups/client      -> cups + cups-client + cups-openrc (/etc/init.d/cupsd)
#   libheif-examples -> libheif-tools (provides heif-convert)
#   golang-go        -> go (pure-Go sqlite: no gcc needed)
#   avahi+dbus       -> mDNS discovery for CUPS-Get-Devices scan
apk add cups cups-client cups-openrc dbus avahi libheif-tools imagemagick go nodejs npm nginx
apk add avahi-openrc dbus-openrc nginx-openrc 2>/dev/null || true
if [ "$INSTALL_HPLIP" = "1" ]; then
  apk add hplip
else
  echo "    Skipping hplip (driverless 'everywhere' only)"
fi

if ! id "$APP_USER" >/dev/null 2>&1; then
  adduser -S -h "$DATA_DIR" -s /sbin/nologin "$APP_USER"
fi
addgroup -S lpadmin 2>/dev/null || true
addgroup "$APP_USER" lpadmin 2>/dev/null || true

mkdir -p "$APP_DIR" "$DATA_DIR/uploads"
cp -r "$REPO_DIR/backend" "$REPO_DIR/frontend" "$APP_DIR/"
chown -R "$APP_USER":"$APP_USER" "$DATA_DIR"
chown -R root:root "$APP_DIR"

echo "==> Building frontend"
(cd "$APP_DIR/frontend" && npm ci --no-audit --no-fund && npm run build)
mkdir -p "$APP_DIR/frontend/dist"
cp "$APP_DIR/frontend/manifest.webmanifest" "$APP_DIR/frontend/icon.svg" "$APP_DIR/frontend/sw.js" "$APP_DIR/frontend/dist/"

echo "==> Building backend (pure Go, no gcc needed)"
(cd "$APP_DIR/backend" && go build -o /usr/local/bin/printnest .)
chmod +x /usr/local/bin/printnest

if [ ! -f "$DATA_DIR/config.env" ]; then
  cp "$REPO_DIR/deploy/lxc/config.env.example" "$DATA_DIR/config.env"
  echo "    EDIT $DATA_DIR/config.env and set PRINTNEST_PASSWORD, then: rc-service printnest restart"
fi
chown "$APP_USER":"$APP_USER" "$DATA_DIR/config.env" 2>/dev/null || true
chmod 600 "$DATA_DIR/config.env" 2>/dev/null || true

echo "==> Enabling services (OpenRC)"
cp "$REPO_DIR/deploy/lxc/printnest.initd" /etc/init.d/printnest
chmod +x /etc/init.d/printnest
for svc in dbus cupsd printnest; do
  if [ -f "/etc/init.d/$svc" ]; then
    rc-update add "$svc" default || true
  fi
done
if [ -f /etc/init.d/avahi-daemon ]; then
  rc-update add avahi-daemon default || true
fi

# Best effort: no-ops when OpenRC is not PID 1 (e.g. during image build).
rc-service dbus start 2>/dev/null || true
rc-service avahi-daemon start 2>/dev/null || true
rc-service cupsd start 2>/dev/null || true

echo ""
echo "==> Next steps"
echo "    1. Edit $DATA_DIR/config.env (PRINTNEST_PASSWORD required on first boot)"
echo "    2. rc-service printnest restart && rc-service printnest status"
echo "    3. Add printer: lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere"
echo "       (fallback: apk add hplip; hp-setup -i, then retry with the hplip PPD)"
echo "    4. Point your existing nginx HTTPS vhost at http://127.0.0.1:8080 (see deploy/lxc/nginx-snippet.conf)"
echo "    NOTE: mDNS discovery (CUPS-Get-Devices scan) needs avahi+dbus running;"
echo "          direct ipp:// add works without them."
