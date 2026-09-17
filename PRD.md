# PrintNest PRD + Token-Optimized Implementation Plan

Source idea: `/projects/printnest/idea.md` (123 lines, frozen input - do NOT re-interpret).
Target: `HP DeskJet 2700 Wi-Fi`, `A4`, native Proxmox LXC, no Docker in V1.

## 1. Locked decisions (do not revisit without asking user)

| # | Decision | Lock |
|---|---|---|
| 1 | Backend | Go, `hiventive/go-ipp` high-level CUPSClient+IPPClient, pure stdlib, no cgo |
| 2 | Frontend | Vite + Svelte SPA static `dist`, served by Go, no SSR, no SvelteKit |
| 3 | CUPS talk | IPP primary to `localhost:631` (same LXC cupsd), `exec lp/lpstat/cancel` fallback behind `PrinterBackend` interface. No other path |
| 4 | Auth | Login required, single admin password. `PRINTNEST_PASSWORD` env on first boot -> bcrypt hash in SQLite `settings`. Session: 32B random, HTTP-only cookie, SQLite `sessions` with expiry. No registration, no roles |
| 5 | Storage | SQLite via `modernc.org/sqlite` (pure Go, no gcc). Row persists, file bytes deleted. `file_retention_minutes` configurable in Admin UI, default 30. TTL sweeper deletes files, keeps row |
| 6 | Images | Accept `pdf,jpg/jpeg,png,heic/heif,webp` max 50MB. Client previews via pdf.js / objectURL. Server always normalizes images to PDF: `HEIC->JPG via heif-convert`, then `JPG/PNG/WEBP->PDF` in Go. CUPS always gets `application/pdf` for images, `application/pdf` passthrough for PDFs |
| 7 | Scaling map | `Actual=none, Fit=fit, Fill=fill`, default `fit`. IPP `print-scaling` |
| 8 | Full IPP attr map | `copies->copies int`, `page-ranges->page-ranges 1setOf range`, `color/BW->print-color-mode=color/monochrome`, `orientation->orientation-requested=3 portrait /4 landscape`, `media->media=iso_a4_210x297mm default`, `scaling->print-scaling`. Validate against `xxx-supported` from Get-Printer-Attributes |
| 9 | Deploy | Native Debian/Ubuntu LXC only. No Docker in V1. `install.sh` + systemd `printnest.service` (Go) uses system `cups.service`. App listens `127.0.0.1:8080`, TLS at user's existing nginx. Caddy optional doc only |
| 10 | Discovery | Auto discovery UI required. Backend lists queues + scans network, frontend Admin page to scan/add |
| 11 | License | MIT, 1 LICENSE file, no telemetry, no cloud |
| 12 | HTTPS/PWA | PWA install + Android Share Target need HTTPS (provided by user's nginx). App only serves HTTP + manifest + SW + `share_target` |

## 2. Does IPP support scan + scaling/color? Yes - precise scope

Verified against CUPS/PWG/RFC8011 docs:

- Discovery: `CUPS-Get-Printers (0x4002)` lists queues. `CUPS-Get-Devices (0x400B)` scans local + network devices (DNS-SD `_ipp._tcp`/`_ipps._tcp`, socket, lpd). Equivalent to `ippfind` + `lpinfo -v`. Use this, not mDNS code in Go.
- Capabilities: `Get-Printer-Attributes (0x000B)` with `requested-attributes: media-supported, media-default, print-color-mode-supported, print-scaling-supported, orientation-requested-supported, copies-supported, page-ranges-supported, document-format-supported, printer-state, printer-state-reasons`.
- Print: `Print-Job (0x0002)` with Document + Job Template attrs from table above.
- Status: `Get-Jobs (0x000A) / Get-Job-Attributes (0x0009)`, cancel via `Cancel-Job (0x0008)`.
- `print-scaling` standard values: `none, fit, fill, auto, auto-fit` (PAPPL/CUPS). V1 only sends `none/fit/fill`.
- `print-color-mode` standard: `color, monochrome` (+auto etc, not in V1).
- DeskJet 2700: add via `lpadmin -p deskjet2700 -E -v ipp://<printer-ip>/ipp/print -m everywhere` if driverless works, else `-m drv:///hplip/...` or hplip PPD. PRD does not assume; Admin UI surfaces `printer-state-reasons` on failure.

## 3. Architecture (native LXC)

```
Phone/PC --HTTPS--> nginx (user existing) --HTTP 8080--> Go (serves Svelte dist + /api)
Go --IPP localhost:631--> cupsd (same LXC) --IPP/socket--> DeskJet 2700 Wi-Fi
Go --exec fallback--> /usr/bin/lp (CUPS_CLIENT only, same host)
Go --exec--> heif-convert (libheif-examples) for HEIC only
Files: /var/lib/printnest/uploads/{jobId}/original + print.pdf, deleted per retention
DB: /var/lib/printnest/printnest.db (sqlite: jobs, sessions, settings, printers_cache)
```

## 4. Repo layout (agents: respect ownership)

```
backend/go.mod
backend/main.go                         # wire config+db+api+static, no logic
backend/internal/config/config.go       # env: PORT, DATA_DIR, CUPS_HOST, PASSWORD, RETENTION
backend/internal/auth/auth.go           # login/logout/middleware, bcrypt+sessions
backend/internal/db/db.go + schema.sql  # sqlite open+migrate, settings/sessions/jobs
backend/internal/print/backend.go       # interface PrinterBackend only
backend/internal/print/ipp.go           # hiventive/go-ipp impl
backend/internal/print/exec.go          # lp fallback impl
backend/internal/print/caps.go          # attrs map + xxx-supported validation
backend/internal/print/discovery.go     # GetPrinters + GetDevices
backend/internal/convert/convert.go     # heic->jpg->pdf wrap, A4/orientation
backend/internal/jobs/jobs.go           # create, IPP submit, poll, cancel
backend/internal/api/handlers.go        # REST only, thin
backend/internal/cleanup/cleanup.go     # retention sweeper
frontend/package.json vite.config.ts index.html manifest.webmanifest sw.js
frontend/src/App.svelte frontend/src/lib/api.ts frontend/src/lib/preview.ts
frontend/src/pages/Login.svelte Upload.svelte JobStatus.svelte AdminPrinters.svelte AdminSettings.svelte
deploy/lxc/install.sh deploy/lxc/printnest.service deploy/lxc/config.env.example
LICENSE (MIT)
```

## 5. API contract (thin, no extra endpoints)

```
POST /api/login {password} -> Set-Cookie session
POST /api/logout
GET  /api/printers                    # from CUPS-Get-Printers (cached 30s)
POST /api/printers/discover           # CUPS-Get-Devices, returns [{uri,make_model,info}]
POST /api/printers/add {uri,name,model} # lpadmin everywhere via IPP CUPS-Add-Modify-Printer or exec
GET  /api/printers/{name}/caps        # Get-Printer-Attributes filtered to supported/default
POST /api/jobs (multipart file)       # validate type/size -> job row status=uploaded
POST /api/jobs/{id}/print {printer,copies,pageRanges,color,orientation,media,scaling}
GET  /api/jobs/{id} GET /api/jobs?limit=50
POST /api/jobs/{id}/cancel
GET  /api/settings GET/PUT /api/settings {file_retention_minutes}
```

DB `jobs(id, printer, orig_name, mime, copies, page_ranges, color_mode, orientation, media, scaling, status, cups_job_id, created_at, printed_at, file_deleted_at)`. No file bytes in DB.

## 6. Token-optimized agent plan (every agent reads ONLY listed files)

Global rule: copy `AGENTS.md` instruction into PRD: `Read ONLY Allowed-reads. Never glob/read rest. If blocked, ask, do not guess.`

| Order | Task | Allowed-reads | Writes | Must NOT read | Done when |
|---|---|---|---|---|---|
| 0 | Scaffold repo + go.mod + LICENSE + AGENTS.md rule | `idea.md` | `backend/go.mod, LICENSE, AGENTS.md, deploy/lxc/*` | frontend/*, internal/* | `go vet ./...` passes, MIT present |
| 1 | Config + DB schema | `backend/go.mod` | `config.go, db.go, schema.sql` | frontend/*, print/*, convert/* | migrate creates 4 tables, retention default 30 |
| 2 | Auth single-password | `config.go, db.go` | `auth.go, api login/logout` | print/*, convert/*, frontend/* | login sets HTTP-only cookie, wrong pw 401 |
| 3 | Print interface + IPP + exec fallback + caps map | `config.go` | `print/backend.go, ipp.go, exec.go, caps.go` | auth/*, db/*, frontend/*, convert/* | `GetPrinters, GetDevices, PrintJob` work against local cupsd; caps validates, rejects unsupported with 400 listing `xxx-supported` |
| 4 | Discovery API | `print/discovery.go, print/caps.go` | `api printers handlers` | frontend/*, convert/* | `POST /discover` returns DeskJet URI on test LAN |
| 5 | Convert HEIC->PDF wrap | none (new) | `convert/convert.go` | auth, print, frontend, db | HEIC+JPG/PNG/WEBP -> A4 PDF honoring orientation; missing `heif-convert` returns 415 with reason |
| 6 | Jobs submit+poll+cancel+cleanup | `print/backend.go, print/caps.go, db.go, convert.go` | `jobs.go, cleanup.go, api jobs handlers` | frontend/* | PDF A4 color/BW, copies, page-ranges, scaling print on DeskJet; file deleted after completion, row kept; retention env + API configurable |
| 7 | Svelte SPA + PWA + Share Target | `api contract section 5 only` | `frontend/*` | `backend/internal/*` | Vite build `dist`, manifest+SW installable over HTTPS, Android Share Target -> Upload, iOS file picker works, client preview PDF+images, settings map exactly section 1.8 |
| 8 | Native LXC install | `backend/main.go, frontend/dist` | `deploy/lxc/install.sh, printnest.service, config.env.example, README` | backend/internal/* | Fresh Debian LXC: `install.sh` installs cups,hplip,cups-client,libheif-examples,imagemagick, Go binary, systemd, nginx snippet; no Docker required |
| 9 | E2E prove `Backend->CUPS->DeskJet2700` | `jobs.go, caps.go` | test checklist only | frontend/* | A4, color, mono, none/fit/fill, copies=2, page-ranges=1-2 all physically verified, errors surfaced from `printer-state-reasons` |

Parallelizable after 0-2: 3,5,7 (different dirs). 4 needs 3. 6 needs 3+5. 8 needs 6+7. 9 last.

## 7. Data flow + failure modes

Flow: Login -> Upload (validate) -> Preview (client) -> Settings (validated vs caps) -> Convert if image -> IPP Print-Job -> Poll job-state -> Mark done -> Delete file now or after `file_retention_minutes` (failed/pending kept until TTL) -> Row retained.

Failures: CUPS down -> 502 + `printer-state`; unsupported attr -> 400 with supported list; HEIC tool missing -> 415; >50MB -> 413; bad page-range -> 400; cancel only when pending/processing else 409; session expiry -> 401. Exec fallback only on IPP transport error, logged as `backend=exec-fallback`.

## 8. Validation + rollout

- `go vet ./...`, `go test ./...` (caps map + convert + retention unit tests).
- `npm run build` produces `dist`, `vite preview` smoke.
- Manual E2E on LXC with DeskJet 2700 Wi-Fi: discover, add everywhere, print matrix above, cancel, retention sweep, reboot persistence (rows remain, files gone).
- Rollout: single LXC, `install.sh`, no migration (fresh SQLite). Docker explicitly out of V1.

## 9. Explicit non-goals (V1)

No Docker image, no Caddy default, no multi-user, no job file history, no server thumbnails, no iOS Share Extension, no duplex/staple UI, no `auto/auto-fit` scaling, no raw image passthrough.
