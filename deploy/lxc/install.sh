#!/usr/bin/env bash
#
# PrintNest native Debian/Ubuntu LXC install from a prebuilt release bundle.
# No build toolchain on the target: no Go, no Node, no npm, no git needed.
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
# Runtime-only deps installed here: cups cups-client hplip (unless
# INSTALL_HPLIP=0) libheif-tools imagemagick nginx. System CUPS provides the
# drivers/filters on glibc, so the static Go binary works as-is.
# Other env: APP_USER APP_DIR DATA_DIR (same defaults as before).
set -euo pipefail

APP_USER="${APP_USER:-printnest}"
APP_DIR="${APP_DIR:-/opt/printnest}"
DATA_DIR="${DATA_DIR:-/var/lib/printnest}"
REPO="${REPO:-shut-sh/printnest}"
INSTALL_HPLIP="${INSTALL_HPLIP:-1}"

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) echo "unsupported arch: $(uname -m) (need amd64 or arm64)" >&2; exit 1 ;;
  esac
}
ARCH="$(detect_arch)"

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

echo "==> PrintNest install from prebuilt bundle (Debian/Ubuntu, no build)"
echo "    APP_DIR=$APP_DIR DATA_DIR=$DATA_DIR USER=$APP_USER HPLIP=$INSTALL_HPLIP REPO=$REPO"

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root" >&2; exit 1
fi

TARBALL="$(fetch_bundle)"

export DEBIAN_FRONTEND=noninteractive
apt-get update
# Runtime only: CUPS stack + HEIC tool + nginx. No golang-go/nodejs/npm.
apt-get install -y cups cups-client libheif-examples imagemagick nginx curl ca-certificates
if [ "$INSTALL_HPLIP" = "1" ]; then
  apt-get install -y hplip
else
  echo "    Skipping hplip (driverless 'everywhere' only)"
fi

id -u "$APP_USER" >/dev/null 2>&1 || useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$APP_USER"
usermod -aG lpadmin "$APP_USER" || true

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
  echo "    EDIT $DATA_DIR/config.env and set PRINTNEST_PASSWORD, then: systemctl restart printnest"
fi
chown "$APP_USER":"$APP_USER" "$DATA_DIR/config.env" 2>/dev/null || true
chmod 600 "$DATA_DIR/config.env" 2>/dev/null || true

cp "$APP_DIR/deploy/lxc/printnest.service" /etc/systemd/system/printnest.service
systemctl daemon-reload
systemctl enable --now cups
systemctl enable printnest || true

echo ""
echo "==> Next steps"
echo "    1. Edit $DATA_DIR/config.env (PRINTNEST_PASSWORD required on first boot)"
echo "    2. systemctl restart printnest && systemctl status printnest"
echo "    3. Add printer: lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere"
echo "       (fallback: hp-setup -i  (hplip), then retry with the hplip PPD)"
echo "    4. Point your existing nginx HTTPS vhost at http://127.0.0.1:8080 (see deploy/lxc/nginx-snippet.conf in $APP_DIR)"
