# PrintNest - Simple, private, self-hosted printing.

Open-source print portal for home networks. Phone/PC uploads a file over
HTTPS, the Go backend normalizes it to PDF and prints via CUPS/IPP to a
network printer (first target: HP DeskJet 2700, A4).

Flow: Choose / Share file -> Preview -> Configure -> Print.

## Quick start (native LXC, no Docker)

Debian/Ubuntu LXC (Proxmox):

```sh
sudo bash deploy/lxc/install.sh
sudoedit /var/lib/printnest/config.env   # set PRINTNEST_PASSWORD
sudo systemctl restart printnest
```

Alpine LXC (Proxmox, OpenRC instead of systemd):

```sh
sh deploy/lxc/install-alpine.sh          # INSTALL_HPLIP=0 for driverless-only, smaller
vi /var/lib/printnest/config.env         # set PRINTNEST_PASSWORD
rc-service printnest restart
```

mDNS discovery (network scan) on Alpine needs `avahi`+`dbus` running;
direct `ipp://` add works without them.

Add the DeskJet (driverless first, hplip fallback):

```sh
lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere
# if that fails: hp-setup -i  (hplip), then retry with the hplip PPD
lpstat -p -d
```

Point your existing nginx HTTPS vhost at `http://127.0.0.1:8080`
(see `deploy/lxc/nginx-snippet.conf`). PWA install + Android Share Target
require HTTPS, which your nginx provides; the app itself serves plain HTTP.

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
cd backend && go test ./... && go vet ./...
cd ../frontend && npm ci && npm run build
PRINTNEST_PASSWORD=devpass DATA_DIR=/tmp/pn-data FRONTEND_DIR=$PWD/frontend/dist \
  go run . 
```

API: `POST /api/login`, `GET /api/printers`, `POST /api/printers/discover`,
`POST /api/printers/add`, `GET /api/printers/{name}/caps`,
`POST /api/jobs` (multipart `file`), `POST /api/jobs/{id}/print`,
`GET /api/jobs/{id}`, `POST /api/jobs/{id}/cancel`, `GET/PUT /api/settings`.

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
