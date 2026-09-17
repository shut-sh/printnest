# PrintNest

 **Open-source, self-hosted print portal for home networks.**

 PrintNest is a lightweight PWA that makes it easy to print documents from phones and computers to network printers without cloud services, vendor accounts, or printer-specific apps.

 ## Goal

 Make printing as simple as:

 Choose / Share file → Preview → Configure → Print

 Initial target: **HP DeskJet 2700 series** connected over Wi-Fi.

 ## Features

 - Mobile-first PWA
- Drag & drop files
- Browse/select files
- PDF preview
- Support PDF, JPG and PNG
- Print settings:
  - Printer
  - Paper size (A4 initially)
  - Portrait / Landscape
  - Color / Black & White
  - Actual size
  - Fit to page
  - Fill page
  - Copies
  - Page range
- Print job status and errors
- Automatically delete temporary documents after printing

 ## Mobile Sharing

 ### Android

 Use **Web Share Target**:

 Any app → Share → PrintNest → Preview → Print

 ### iPhone

 Support normal file selection through the PWA.

 Do not depend on iOS Share Target support. Keep the architecture open for a future native iOS Share Extension.

 ## Architecture

 The browser should never communicate directly with the printer.

```
Phone / PC
    ↓ HTTPS
PrintNest PWA
    ↓ API
PrintNest Backend
    ↓ IPP
CUPS
    ↓
Network Printer
```

 Use **CUPS/IPP** for printing. The backend should remain printer-agnostic and discover printer capabilities through CUPS where possible.

 ## Deployment

 Designed for home servers such as **Proxmox**:

```
Proxmox
  ↓
Debian/Ubuntu LXC or VM
  ↓
Docker Compose
  ├── PrintNest
  └── CUPS
```

 Ideally install with:

```
docker compose up -d
```

 ## Open Source

 PrintNest must be fully open source and self-hostable.

 Requirements:

 - No mandatory cloud service
- No vendor account
- No telemetry by default
- No dependency on HP Smart
- Documents remain under the user's control
- Provide clear installation and configuration documentation
- Use a suitable permissive open-source license

 ## V1 Priority

 First prove:

```
PrintNest Backend → CUPS → HP DeskJet 2700
```

 with:

 - A4
- Color/B&W
- Scaling
- Copies
- Page ranges

 Then build the PWA UI around the working print backend.

 Keep V1 **small, reliable, privacy-focused and easy to deploy**.

 ## Project Tagline

 > **PrintNest — Simple, private, self-hosted printing.**
