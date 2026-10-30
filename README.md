# PrintNest - Simple, private, self-hosted printing.

Open-source print portal for home networks. Phone/PC uploads a file over
HTTPS, the Go backend normalizes it to PDF and prints via CUPS/IPP to a
network printer (first target: HP DeskJet 2700, A4).

Flow: Choose / Share file -> Preview -> Configure -> Print.

## Install (native LXC, no Docker)

Works in a Debian/Ubuntu or Alpine LXC on Proxmox, amd64 or arm64. Nothing is
compiled on the container and no git clone is needed. Each release contains a
static binary, the built web UI, the installers, and the config templates.

Run as root inside the container. Both options below need curl first:

```sh
apt-get install -y curl ca-certificates      # Alpine: apk add curl ca-certificates
```

Option A, one command (downloads the release for this architecture and verifies it):

```sh
# Debian/Ubuntu
curl -fsSL https://github.com/shut-sh/printnest/releases/download/v0.1.1/install.sh | RELEASE_TAG=v0.1.1 bash

# Alpine
curl -fsSL https://github.com/shut-sh/printnest/releases/download/v0.1.1/install-alpine.sh | RELEASE_TAG=v0.1.1 sh
```

Option B, download and verify the tarball yourself, then install from it:

```sh
VERSION=0.1.1
ARCH=$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
BASE=https://github.com/shut-sh/printnest/releases/download/v$VERSION
curl -fsSLO "$BASE/printnest-$VERSION-linux-$ARCH.tar.gz"
curl -fsSLO "$BASE/printnest-$VERSION-linux-$ARCH.tar.gz.sha256"
sha256sum -c "printnest-$VERSION-linux-$ARCH.tar.gz.sha256"
tar xzf "printnest-$VERSION-linux-$ARCH.tar.gz"
./printnest/deploy/lxc/install.sh            # Alpine: sh ./printnest/deploy/lxc/install-alpine.sh
```

Both options install the same things: `/usr/local/bin/printnest`, the web UI and
templates under `/opt/printnest`, the config at `/var/lib/printnest/config.env`, and a
`printnest` service (systemd, or OpenRC on Alpine). The service is enabled but
stays stopped until you set the admin password:

```sh
vi /var/lib/printnest/config.env     # PRINTNEST_PASSWORD is read on first start only
systemctl start printnest            # Alpine: rc-service printnest start
```

Logs: `journalctl -u printnest` on Debian/Ubuntu, `/var/log/printnest.log` on Alpine.

Add the DeskJet (driverless first, hplip fallback):

```sh
lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere
# if that fails: hp-setup -i  (hplip), then retry with the hplip PPD
lpstat -p -d
```

Point your existing nginx HTTPS vhost at `http://127.0.0.1:8080`, using
`/opt/printnest/deploy/lxc/nginx-snippet.conf` as the template. PWA install and the
Android Share Target require HTTPS, which your nginx provides; the app itself serves plain HTTP.

mDNS discovery (network scan) on Alpine needs `avahi` and `dbus` running; a direct
`ipp://` add works without them.

To upgrade, run the installer again with the new release. Config and uploads are kept,
and a running service is restarted.

Installer options (environment variables, all optional):

| Variable | Effect |
|---|---|
| `RELEASE_TAG` | Release to download when running standalone (default: latest) |
| `BUNDLE_FILE` | Install from a local tarball instead of downloading |
| `BUNDLE_URL`, `BUNDLE_SHA256` | Download from any URL and verify against the given sha256 hex digest |
| `INSTALL_HPLIP=0` | Skip hplip (driverless `everywhere` only, smaller) |
| `APP_DIR`, `DATA_DIR`, `APP_USER` | Install locations and service user |

When the installer runs from an extracted bundle (Option B), it does not download the
bundle again. Package installation still fetches from the distro repositories.

## Using it

1. Open the site, sign in with the admin password.
2. Upload / share / drag-drop a PDF, JPG, PNG, WEBP, or HEIC (max 50 MB).
3. Preview, pick printer, A4, portrait/landscape, color/BW, actual/fit/fill,
   copies, page range (e.g. `1-2`).
4. Print, watch job status, cancel if needed.
5. Uploaded file bytes auto-delete after `file_retention_minutes`
   (default 30, Admin UI); job rows are kept.

Admin pages: `/admin/printers` (scan network via CUPS-Get-Devices,
add with driverless `everywhere`), `/admin/settings` (retention).

## Dev

```sh
cd frontend && npm ci && npm run build && cd ..
cd backend && go test ./... && go vet ./...
PRINTNEST_PASSWORD=devpass DATA_DIR=/tmp/pn-data FRONTEND_DIR=$(pwd)/../frontend/dist go run .
```

API: `POST /api/login`, `GET /api/printers`, `POST /api/printers/discover`,
`POST /api/printers/add`, `GET /api/printers/{name}/caps`,
`POST /api/jobs` (multipart `file`), `POST /api/jobs/{id}/print`,
`GET /api/jobs/{id}`, `POST /api/jobs/{id}/cancel`, `GET/PUT /api/settings`.

## Release (manual)

The GitHub Actions release workflow is disabled, so releases are cut by hand.
`deploy/build-bundle.sh` writes one static bundle per architecture plus the two
installers into `dist/`:

```sh
git tag -a v0.1.1 -m "PrintNest v0.1.1" && git push origin v0.1.1
bash deploy/build-bundle.sh --version 0.1.1
gh release create v0.1.1 --title "PrintNest v0.1.1" --generate-notes dist/*
```

## E2E prove-out (DeskJet 2700)

Discover, add `everywhere`, then physically print: A4 color, mono,
scaling none/fit/fill, copies=2, page-ranges=1-2. Cancel a queued job.
Confirm file bytes vanish after retention while rows persist, and that
`printer-state-reasons` surface on failure. Missing `heif-convert` yields
HTTP 415; >50 MB yields 413; bad attrs yield 400 with the supported list.

## Non-goals (V1)

No Docker image, no Caddy default, no multi-user, no file history,
no server thumbnails, no iOS Share Extension, no duplex/staple UI,
no auto/auto-fit scaling, no raw image passthrough (images always go
to CUPS as PDF).

MIT - see LICENSE.
