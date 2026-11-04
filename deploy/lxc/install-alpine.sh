#!/bin/sh
#
# Installs PrintNest on an Alpine LXC (OpenRC) from a prebuilt release bundle.
# Nothing is compiled on the container; no git clone is needed.
#
#   sh ./printnest/deploy/lxc/install-alpine.sh   run from an extracted bundle
#   curl -fsSL https://github.com/shut-sh/printnest/releases/download/v0.1.0/install-alpine.sh \
#     | RELEASE_TAG=v0.1.0 sh                      standalone, downloads the bundle
#
# Env: RELEASE_TAG (default: latest), BUNDLE_FILE, BUNDLE_URL + BUNDLE_SHA256,
# REPO (default: shut-sh/printnest), APP_USER, APP_DIR, DATA_DIR, INSTALL_HPLIP=0.
set -eu

APP_USER="${APP_USER:-printnest}"
APP_DIR="${APP_DIR:-/opt/printnest}"
DATA_DIR="${DATA_DIR:-/var/lib/printnest}"
REPO="${REPO:-shut-sh/printnest}"
INSTALL_HPLIP="${INSTALL_HPLIP:-1}"

die() {
  echo "error: $*" >&2
  exit 1
}

configure_cups_retries() {
  conf=/etc/cups/cupsd.conf
  if [ ! -f "$conf" ]; then
    echo "warning: $conf not found; skipping CUPS retry configuration" >&2
    return 0
  fi
  tmp="$(mktemp)"
  awk '
    /^# BEGIN printnest$/ { skip = 1; next }
    /^# END printnest$/   { skip = 0; next }
    skip != 1             { print }
  ' "$conf" > "$tmp"
  cat >> "$tmp" <<'CUPSBLOCK'

# BEGIN printnest
# Bound retries when a printer stops answering IPP (PrintNest)
JobRetryLimit 3
JobRetryInterval 30
# END printnest
CUPSBLOCK
  chmod 0644 "$tmp"
  chown root:root "$tmp" 2>/dev/null || true
  cp -p "$conf" "${conf}.printnest.bak" 2>/dev/null || true
  mv "$tmp" "$conf"
  if cupsd -t >/dev/null 2>&1; then
    echo "    CUPS retry config applied: JobRetryLimit 3, JobRetryInterval 30"
  else
    echo "warning: cupsd rejected the new config, restoring backup" >&2
    if [ -f "${conf}.printnest.bak" ]; then
      mv "${conf}.printnest.bak" "$conf"
    fi
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) die "unsupported arch $(uname -m) (need amd64 or arm64)" ;;
  esac
}

find_extracted_bundle() {
  [ -f "$0" ] || return 0
  candidate="$(cd "$(dirname "$0")/../.." && pwd)" || return 0
  if [ -x "$candidate/printnest" ] && [ -f "$candidate/deploy/lxc/printnest.initd" ] && [ -f "$candidate/frontend/dist/index.html" ]; then
    echo "$candidate"
  fi
}

fetch_bundle() {
  arch="$(detect_arch)"
  if [ -n "${BUNDLE_FILE:-}" ]; then
    [ -f "$BUNDLE_FILE" ] || die "BUNDLE_FILE not found: $BUNDLE_FILE"
    BUNDLE_TAR="$BUNDLE_FILE"
    return 0
  fi
  command -v curl >/dev/null 2>&1 || die "curl is required to download the bundle (apk add curl, or set BUNDLE_FILE)"
  if [ -n "${BUNDLE_URL:-}" ]; then
    BUNDLE_TAR="$WORKDIR/bundle.tar.gz"
    echo "==> Downloading $BUNDLE_URL"
    curl -fsSL --retry 3 -o "$BUNDLE_TAR" "$BUNDLE_URL" || die "download failed: $BUNDLE_URL"
    if [ -n "${BUNDLE_SHA256:-}" ]; then
      printf '%s  bundle.tar.gz\n' "$BUNDLE_SHA256" > "$WORKDIR/bundle.tar.gz.sha256"
      (cd "$WORKDIR" && sha256sum -c bundle.tar.gz.sha256)
    else
      echo "warning: BUNDLE_SHA256 not set, skipping checksum verification" >&2
    fi
    return 0
  fi
  if [ -z "${RELEASE_TAG:-}" ]; then
    RELEASE_TAG="$(curl -fsSL --retry 3 "https://api.github.com/repos/$REPO/releases/latest" \
      | grep -m1 '"tag_name"' | cut -d'"' -f4)" || true
    [ -n "$RELEASE_TAG" ] || die "no release found for $REPO (set RELEASE_TAG or BUNDLE_FILE)"
  fi
  case "$RELEASE_TAG" in
    v*) ;;
    *) RELEASE_TAG="v$RELEASE_TAG" ;;
  esac
  name="printnest-${RELEASE_TAG#v}-linux-$arch.tar.gz"
  base="https://github.com/$REPO/releases/download/$RELEASE_TAG"
  BUNDLE_TAR="$WORKDIR/$name"
  echo "==> Downloading $base/$name"
  curl -fsSL --retry 3 -o "$WORKDIR/$name.sha256" "$base/$name.sha256" \
    || die "download failed: $base/$name.sha256 (is $RELEASE_TAG published?)"
  curl -fsSL --retry 3 -o "$BUNDLE_TAR" "$base/$name" || die "download failed: $base/$name"
  (cd "$WORKDIR" && sha256sum -c "$name.sha256")
}

main() {
  if [ "$(id -u)" -ne 0 ]; then
    die "run as root"
  fi
  if [ ! -f /etc/alpine-release ]; then
    echo "warning: /etc/alpine-release not found; continuing anyway" >&2
  fi

  echo "==> PrintNest install (Alpine, prebuilt bundle, no build)"
  echo "    APP_DIR=$APP_DIR DATA_DIR=$DATA_DIR USER=$APP_USER HPLIP=$INSTALL_HPLIP"

  BUNDLE_ROOT="$(find_extracted_bundle)"
  if [ -n "$BUNDLE_ROOT" ]; then
    echo "==> Using extracted bundle at $BUNDLE_ROOT"
  else
    WORKDIR="$(mktemp -d)"
    trap 'rm -rf "$WORKDIR"' EXIT
    fetch_bundle
    echo "==> Extracting $(basename "$BUNDLE_TAR")"
    tar xzf "$BUNDLE_TAR" -C "$WORKDIR"
    BUNDLE_ROOT="$WORKDIR/printnest"
  fi
  for f in printnest deploy/lxc/config.env.example deploy/lxc/printnest.initd frontend/dist/index.html; do
    [ -e "$BUNDLE_ROOT/$f" ] || die "bundle is missing $f ($BUNDLE_ROOT)"
  done

  if grep -q '^[[:space:]]*#[[:space:]]*https\?://[^[:space:]]*/community[[:space:]]*$' /etc/apk/repositories 2>/dev/null; then
    echo "==> Enabling community repository"
    sed -i 's|^[[:space:]]*#[[:space:]]*\(https\?://[^[:space:]]*/community[[:space:]]*\)$|\1|' /etc/apk/repositories
  fi

  echo "==> Installing runtime packages"
  apk update
  apk add openrc cups cups-client cups-openrc dbus dbus-openrc avahi avahi-openrc \
    libheif-tools imagemagick curl ca-certificates
  if [ "$INSTALL_HPLIP" = "1" ]; then
    apk add hplip
  else
    echo "    Skipping hplip (driverless 'everywhere' only)"
  fi

  configure_cups_retries

  if ! id "$APP_USER" >/dev/null 2>&1; then
    addgroup -S "$APP_USER"
    adduser -S -G "$APP_USER" -h "$DATA_DIR" -s /sbin/nologin "$APP_USER"
  fi
  addgroup -S lpadmin 2>/dev/null || true
  addgroup "$APP_USER" lpadmin 2>/dev/null || true
  for admin in root "${SUDO_USER:-}"; do
    if [ -n "$admin" ]; then
      addgroup "$admin" lpadmin 2>/dev/null || true
    fi
  done

  mkdir -p "$APP_DIR" "$DATA_DIR/uploads"
  if [ "$BUNDLE_ROOT" != "$APP_DIR" ]; then
    cp -r "$BUNDLE_ROOT/." "$APP_DIR/"
  fi
  chown -R root:root "$APP_DIR"
  install -m 0755 "$APP_DIR/printnest" /usr/local/bin/printnest
  chown -R "$APP_USER":"$APP_USER" "$DATA_DIR"

  if [ ! -f "$DATA_DIR/config.env" ]; then
    cp "$APP_DIR/deploy/lxc/config.env.example" "$DATA_DIR/config.env"
    echo "    EDIT $DATA_DIR/config.env and set PRINTNEST_PASSWORD before starting"
  fi
  chown "$APP_USER":"$APP_USER" "$DATA_DIR/config.env"
  chmod 600 "$DATA_DIR/config.env"

  echo "==> Enabling services (OpenRC)"
  cp "$APP_DIR/deploy/lxc/printnest.initd" /etc/init.d/printnest
  chmod 755 /etc/init.d/printnest
  rc-update add dbus default
  rc-update add avahi-daemon default
  rc-update add cupsd default
  rc-update add printnest default
  rc-service dbus start 2>/dev/null || true
  rc-service avahi-daemon start 2>/dev/null || true
  rc-service cupsd start 2>/dev/null || true
  rc-service cupsd restart 2>/dev/null || true
  if rc-service printnest status >/dev/null 2>&1; then
    rc-service printnest restart
  fi

  echo ""
  echo "==> Next steps"
  echo "    1. Set the admin password: vi $DATA_DIR/config.env  (PRINTNEST_PASSWORD, read on first start only)"
  echo "    2. rc-service printnest start && rc-service printnest status"
  echo "    3. Add the printer: lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere"
  echo "       (fallback: hp-setup -i, then retry with the hplip PPD)"
  echo "    4. Point your nginx HTTPS vhost at http://127.0.0.1:8080 (template: $APP_DIR/deploy/lxc/nginx-snippet.conf)"
  echo "    mDNS discovery (network scan) needs avahi+dbus running; direct ipp:// add works without them."
}

main "$@"
