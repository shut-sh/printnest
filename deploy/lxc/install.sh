#!/usr/bin/env bash
#
# Installs PrintNest on a Debian/Ubuntu LXC from a prebuilt release bundle.
# Nothing is compiled on the container; no git clone is needed.
#
#   ./printnest/deploy/lxc/install.sh        run from an extracted bundle
#   curl -fsSL https://github.com/shut-sh/printnest/releases/download/v0.1.0/install.sh \
#     | RELEASE_TAG=v0.1.0 bash              standalone, downloads the bundle
#
# Env: RELEASE_TAG (default: latest), BUNDLE_FILE, BUNDLE_URL + BUNDLE_SHA256,
# REPO (default: shut-sh/printnest), APP_USER, APP_DIR, DATA_DIR, INSTALL_HPLIP=0.
set -euo pipefail

APP_USER="${APP_USER:-printnest}"
APP_DIR="${APP_DIR:-/opt/printnest}"
DATA_DIR="${DATA_DIR:-/var/lib/printnest}"
REPO="${REPO:-shut-sh/printnest}"
INSTALL_HPLIP="${INSTALL_HPLIP:-1}"

die() {
  echo "error: $*" >&2
  exit 1
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
  if [ -x "$candidate/printnest" ] && [ -f "$candidate/deploy/lxc/printnest.service" ] && [ -f "$candidate/frontend/dist/index.html" ]; then
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
  command -v curl >/dev/null 2>&1 || die "curl is required to download the bundle (apt-get install -y curl, or set BUNDLE_FILE)"
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

  echo "==> PrintNest install (Debian/Ubuntu, prebuilt bundle, no build)"
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
  for f in printnest deploy/lxc/config.env.example deploy/lxc/printnest.service frontend/dist/index.html; do
    [ -e "$BUNDLE_ROOT/$f" ] || die "bundle is missing $f ($BUNDLE_ROOT)"
  done

  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y cups cups-client libheif-examples imagemagick nginx curl ca-certificates
  if [ "$INSTALL_HPLIP" = "1" ]; then
    apt-get install -y hplip
  else
    echo "    Skipping hplip (driverless 'everywhere' only)"
  fi

  id -u "$APP_USER" >/dev/null 2>&1 || useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$APP_USER"
  usermod -aG lpadmin "$APP_USER" || true

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

  cp "$APP_DIR/deploy/lxc/printnest.service" /etc/systemd/system/printnest.service
  systemctl daemon-reload
  systemctl enable --now cups
  systemctl enable printnest
  systemctl try-restart printnest

  echo ""
  echo "==> Next steps"
  echo "    1. Set the admin password: vi $DATA_DIR/config.env  (PRINTNEST_PASSWORD, read on first start only)"
  echo "    2. systemctl start printnest && systemctl status printnest"
  echo "    3. Add the printer: lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere"
  echo "       (fallback: hp-setup -i, then retry with the hplip PPD)"
  echo "    4. Point your nginx HTTPS vhost at http://127.0.0.1:8080 (template: $APP_DIR/deploy/lxc/nginx-snippet.conf)"
}

main "$@"
