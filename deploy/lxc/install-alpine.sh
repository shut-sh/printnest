#!/bin/sh
# PrintNest native Alpine LXC install from a prebuilt release bundle.
# No build toolchain on the target: no go, no nodejs, no npm, no git needed.
# Debian/Ubuntu equivalent: deploy/lxc/install.sh
# Target: Alpine 3.20+ Proxmox LXC with OpenRC.
#
# Bundle (built by deploy/build-bundle.sh, attached to the GitHub release):
#   printnest-<version>-linux-<arch>.tar.gz, verified against its .sha256
#
# Sources for the bundle (env overrides):
#   BUNDLE_FILE  local .tar.gz path
#   BUNDLE_URL   direct URL to the .tar.gz   (BUNDLE_SHA256 expected alongside)
#   BUNDLE_SHA256  pinned "sha256  filename" line, or URL of the .sha256 file
#   RELEASE_TAG  e.g. v0.1.0; fetched from GitHub releases when neither is set
#   REPO         GitHub owner/repo for the release (default: shut-sh/printnest)
#
# Runtime-only deps installed here: cups cups-client cups-openrc dbus avahi
# libheif-tools imagemagick nginx (+ hplip unless INSTALL_HPLIP=0).
# Other env: APP_USER APP_DIR DATA_DIR (same defaults as before).
set -eu

APP_USER="${APP_USER:-printnest}"
APP_DIR="${APP_DIR:-/opt/printnest}"
DATA_DIR="${DATA_DIR:-/var/lib/printnest}"
REPO="${REPO:-shut-sh/printnest}"
INSTALL_HPLIP="${INSTALL_HPLIP:-1}"

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "unsupported arch: $(uname -m) (need amd64 or arm64)" >&2; exit 1 ;;
esac

TMPDIR="$(mktemp -d)"
cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

fetch_bundle() {
  if [ -n "${BUNDLE_FILE:-}" ]; then
    [ -f "$BUNDLE_FILE" ] || { echo "BUNDLE_FILE not found: $BUNDLE_FILE" >&2; exit 1; }
    echo "$BUNDLE_FILE"
    return
  fi
  if [ -n "${BUNDLE_URL:-}" ]; then
    out="$TMPDIR/bundle.tar.gz"
    echo "==> Downloading $BUNDLE_URL"
    curl -fsSL --retry 3 -o "$out" "$BUNDLE_URL"
    if [ -n "${BUNDLE_SHA256:-}" ]; then
      case "$BUNDLE_SHA256" in
        http*|ftp*)
          curl -fsSL --retry 3 -o "$TMPDIR/bundle.tar.gz.sha256" "$BUNDLE_SHA256" ;;
        *)
          printf '%s\n' "$BUNDLE_SHA256" > "$TMPDIR/bundle.tar.gz.sha256" ;;
      esac
      (cd "$TMPDIR" && sha256sum -c bundle.tar.gz.sha256)
    else
      echo "warning: no BUNDLE_SHA256 given, skipping checksum verify" >&2
    fi
    echo "$out"
    return
  fi
  if [ -z "${RELEASE_TAG:-}" ]; then
    RELEASE_TAG="$(curl -fsSL --retry 3 "https://api.github.com/repos/$REPO/releases/latest" \
      | grep -m1 '"tag_name"' | cut -d'"' -f4)"
    [ -n "$RELEASE_TAG" ] || { echo "could not resolve latest release tag" >&2; exit 1; }
  fi
  VERSION="${RELEASE_TAG#v}"
  TARBALL="printnest-$VERSION-linux-$ARCH.tar.gz"
  URL="https://github.com/$REPO/releases/download/$RELEASE_TAG/$TARBALL"
  echo "==> Downloading $URL"
  curl -fsSL --retry 3 -o "$TMPDIR/$TARBALL" "$URL"
  curl -fsSL --retry 3 -o "$TMPDIR/$TARBALL.sha256" "$URL.sha256"
  (cd "$TMPDIR" && sha256sum -c "$TARBALL.sha256")
  echo "$TMPDIR/$TARBALL"
}

echo "==> PrintNest install from prebuilt bundle (Alpine, no build)"
echo "    APP_DIR=$APP_DIR DATA_DIR=$DATA_DIR USER=$APP_USER HPLIP=$INSTALL_HPLIP REPO=$REPO"

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root" >&2; exit 1
fi
if [ ! -f /etc/alpine-release ]; then
  echo "warning: /etc/alpine-release not found; continuing anyway" >&2
fi

TARBALL="$(fetch_bundle)"

# hplip + libheif-tools live in community; enable it if commented out.
if grep -q '^[[:space:]]*#[[:space:]]*https\?://[^[:space:]]*/community[[:space:]]*$' /etc/apk/repositories 2>/dev/null; then
  echo "==> Enabling community repository"
  sed -i 's|^[[:space:]]*#[[:space:]]*\(https\?://[^[:space:]]*/community[[:space:]]*\)$|\1|' /etc/apk/repositories
fi

echo "==> Installing runtime packages (no go/nodejs/npm)"
# Package map vs Debian install.sh:
#   libheif-examples -> libheif-tools (provides heif-convert)
#   avahi+dbus       -> mDNS discovery for CUPS-Get-Devices scan
apk update
apk add cups cups-client cups-openrc dbus avahi libheif-tools imagemagick nginx curl ca-certificates
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
echo "==> Extracting bundle"
tar xzf "$TARBALL" -C "$TMPDIR"
test -x "$TMPDIR/printnest/printnest" || { echo "bundle missing printnest binary" >&2; exit 1; }
test -f "$TMPDIR/printnest/frontend/dist/index.html" || { echo "bundle missing frontend/dist" >&2; exit 1; }
cp -r "$TMPDIR/printnest/." "$APP_DIR/"
install -m 0755 "$APP_DIR/printnest" /usr/local/bin/printnest
chown -R "$APP_USER":"$APP_USER" "$DATA_DIR"
chown -R root:root "$APP_DIR"

if [ ! -f "$DATA_DIR/config.env" ]; then
  cp "$APP_DIR/deploy/lxc/config.env.example" "$DATA_DIR/config.env"
  echo "    EDIT $DATA_DIR/config.env and set PRINTNEST_PASSWORD, then: rc-service printnest restart"
fi
chown "$APP_USER":"$APP_USER" "$DATA_DIR/config.env" 2>/dev/null || true
chmod 600 "$DATA_DIR/config.env" 2>/dev/null || true

echo "==> Enabling services (OpenRC)"
cp "$APP_DIR/deploy/lxc/printnest.initd" /etc/init.d/printnest
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
echo "    4. Point your existing nginx HTTPS vhost at http://127.0.0.1:8080 (see deploy/lxc/nginx-snippet.conf in $APP_DIR)"
echo "    NOTE: mDNS discovery (CUPS-Get-Devices scan) needs avahi+dbus running;"
echo "          direct ipp:// add works without them."
