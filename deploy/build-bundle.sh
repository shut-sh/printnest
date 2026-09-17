#!/usr/bin/env bash
#
# Build a self-contained PrintNest release bundle for Linux.
#
# One static binary (CGO_ENABLED=0, pure Go, no cgo) runs on both
# Debian/glibc and Alpine/musl, so a single bundle per arch covers both.
# The Debian/Ubuntu (deploy/lxc/install.sh) and Alpine
# (deploy/lxc/install-alpine.sh) installers copy from this bundle
# instead of building on the target, so no Go/Node toolchain is needed
# on the LXC.
#
# Output: dist/printnest-<version>-linux-<arch>.tar.gz (+ .sha256)
# Layout inside the tarball (top-level dir "printnest/"):
#   printnest                  backend binary (static)
#   frontend/dist/             built Svelte SPA + PWA files
#   deploy/lxc/                install.sh, install-alpine.sh,
#                              printnest.service, printnest.initd,
#                              config.env.example, nginx-snippet.conf
#   README.md, LICENSE, VERSION
#
# Usage:
#   bash deploy/build-bundle.sh [--version X.Y.Z] [--arch amd64|arm64|all] [--out DIR]
# Version defaults to the current git tag (without leading v),
# else frontend/package.json.
set -euo pipefail

VERSION=""
ARCHES="all"
OUT_DIR=""

usage() {
  cat <<EOF
Usage: $(basename "$0") [--version X.Y.Z] [--arch amd64|arm64|all] [--out DIR]

  --version  bundle version (default: git tag, else frontend/package.json)
  --arch     target arch, or "all" for amd64+arm64 (default: all)
  --out      output directory (default: <repo>/dist, git-ignored)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:?--version needs a value}"; shift 2 ;;
    --arch) ARCHES="${2:?--arch needs a value}"; shift 2 ;;
    --out) OUT_DIR="${2:?--out needs a value}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage >&2; exit 1 ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-$REPO_ROOT/dist}"

if [ -z "$VERSION" ]; then
  VERSION="$(git -C "$REPO_ROOT" describe --tags --exact-match 2>/dev/null || true)"
  VERSION="${VERSION#v}"
fi
if [ -z "$VERSION" ]; then
  VERSION="$(node -p "require('$REPO_ROOT/frontend/package.json').version")"
fi
if [ -z "$VERSION" ]; then
  echo "could not determine version (pass --version)" >&2; exit 1
fi

case "$ARCHES" in
  all) ARCH_LIST="amd64 arm64" ;;
  amd64|arm64) ARCH_LIST="$ARCHES" ;;
  *) echo "--arch must be amd64, arm64, or all" >&2; exit 1 ;;
esac

echo "==> PrintNest bundle version=$VERSION arch=[$ARCH_LIST] out=$OUT_DIR"
mkdir -p "$OUT_DIR"

echo "==> Building frontend (arch-independent, once)"
(cd "$REPO_ROOT/frontend" && npm ci --no-audit --no-fund && npm run build)
cp "$REPO_ROOT/frontend/manifest.webmanifest" "$REPO_ROOT/frontend/icon.svg" \
   "$REPO_ROOT/frontend/sw.js" "$REPO_ROOT/frontend/dist/"
test -f "$REPO_ROOT/frontend/dist/index.html" || { echo "frontend dist missing index.html" >&2; exit 1; }

for ARCH in $ARCH_LIST; do
  echo "==> Building backend (linux/$ARCH, static, no cgo)"
  STAGE="$OUT_DIR/stage-printnest"
  rm -rf "$STAGE"
  mkdir -p "$STAGE/printnest/frontend" "$STAGE/printnest/deploy/lxc"
  (cd "$REPO_ROOT/backend" && \
    CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
    go build -trimpath -ldflags "-s -w" -o "$STAGE/printnest/printnest" .)
  BIN="$STAGE/printnest/printnest"
  if command -v file >/dev/null 2>&1; then
    file "$BIN"
    file "$BIN" | grep -q "statically linked" \
      || echo "warning: $ARCH binary may not be fully static" >&2
  fi
  cp -r "$REPO_ROOT/frontend/dist" "$STAGE/printnest/frontend/dist"
  cp "$REPO_ROOT/deploy/lxc/install.sh" "$REPO_ROOT/deploy/lxc/install-alpine.sh" \
     "$REPO_ROOT/deploy/lxc/printnest.service" "$REPO_ROOT/deploy/lxc/printnest.initd" \
     "$REPO_ROOT/deploy/lxc/config.env.example" "$REPO_ROOT/deploy/lxc/nginx-snippet.conf" \
     "$STAGE/printnest/deploy/lxc/"
  chmod +x "$STAGE/printnest/deploy/lxc/install.sh" \
           "$STAGE/printnest/deploy/lxc/install-alpine.sh" \
           "$STAGE/printnest/deploy/lxc/printnest.initd"
  cp "$REPO_ROOT/README.md" "$REPO_ROOT/LICENSE" "$STAGE/printnest/"
  echo "$VERSION" > "$STAGE/printnest/VERSION"
  TARBALL="$OUT_DIR/printnest-$VERSION-linux-$ARCH.tar.gz"
  (cd "$STAGE" && tar czf "$TARBALL" printnest)
  (cd "$OUT_DIR" && sha256sum "$(basename "$TARBALL")" > "$(basename "$TARBALL").sha256")
  rm -rf "$STAGE"
  echo "    wrote $TARBALL"
done

echo "==> Done. Publish dist/*.tar.gz + dist/*.sha256 to the GitHub release."
