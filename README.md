# LevelMix

[![CI](https://github.com/simonlewi/levelmix/actions/workflows/ci.yml/badge.svg)](https://github.com/simonlewi/levelmix/actions/workflows/ci.yml)

**No more volume jumps.**

LevelMix is a web-based SaaS that automatically corrects inconsistent audio levels
in DJ mixes, podcasts, audiobooks, and long-form audio content. Upload your file,
choose a preset, download a normalized result — no audio engineering background
required.

Live at [levelmix.io](https://levelmix.io)

---

## The Problem

One track too quiet. The next one too loud. Inconsistent audio loses listeners —
in DJ sets, podcast feeds, and long-form content alike.

Manual loudness correction is slow, technical, and easy to get wrong. LevelMix
automates it, consistently, every time.

**Who it's for:**
- DJs and live set performers with uneven track loudness
- Podcasters with volume inconsistency between hosts or recording sessions
- Audiobook producers needing broadcast-standard consistency
- Content agencies processing high volumes of audio
- Video editors balancing audio across clips

---

## Features

- **Automatic loudness correction** — Upload, process, download. No LUFS
  knowledge needed.
- **Preset-based normalization** — DJ Mix (-5 LUFS), Streaming (-14 LUFS),
  Podcast (-16 LUFS), Broadcast (-23 LUFS)
- **Dynamics-aware processing** — Proprietary algorithm preserves musical
  dynamics; loud sections hit target while quiet sections remain proportionally
  quieter
- **Silence trimming** — Automatic detection and removal of leading/trailing
  dead air
- **Real-time progress tracking** — Server-sent events with percentage-based
  job status
- **Secure file handling** — Files stored on AWS S3 with presigned URLs;
  auto-deleted after 30 days
- **Priority queue** — Paid tiers receive higher job priority via separate
  Asynq queue weights

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.24+ |
| Web framework | Gin |
| Templates | `html/template` + Tailwind CSS v4 |
| Frontend | Vanilla JS (no React, no HTMX) |
| Audio processing | FFmpeg (two-pass loudnorm + alimiter chain) |
| Job queue | Asynq + Redis |
| Database | Turso (libSQL / SQLite) |
| File storage | AWS S3 (presigned URLs, multipart upload) |
| Payments | Stripe (Hosted Checkout, webhook-driven tier sync) |
| Email | Resend (transactional + broadcast) |
| Deployment | Docker Compose + Traefik on Hetzner CX22 |
| DNS / SSL | Cloudflare |

---

## Architecture

This repo is **open-core**. The `core/` and `pkg/` directories contain the
application logic and compile standalone. Enterprise features live in a private
`ee/` directory that is **not included** in this repo.

Go build tags control which implementation is wired in:

```bash
# Community Edition (stubs only — prints "rebuild with -tags ee")
go build ./core/...

# Enterprise Edition (full wiring)
go build -tags ee ./core/...
```

```
levelmix/
├── core/                    # Open-source application logic
│   ├── cmd/
│   │   ├── server/         # Web server (main.go)
│   │   ├── worker/         # Background audio processor
│   │   └── cleanup/        # S3 lifecycle cleanup job
│   ├── internal/
│   │   ├── audio/          # FFmpeg processing pipeline
│   │   └── handlers/       # HTTP request handlers
│   ├── static/             # CSS, JS, images, favicon
│   └── templates/          # HTML templates
├── ee/                     # Enterprise features (private, not included)
│   ├── auth/              # Authentication system
│   ├── cleanup/           # S3 and consent cleanup
│   ├── payment/           # Stripe integration
│   └── storage/           # Turso + S3 implementations
├── pkg/                   # Shared interfaces (open-source)
│   ├── email/             # Email service
│   └── storage/           # Storage interfaces and models
├── go.mod
└── README.md
```

---

## Audio Processing Pipeline

Two processing modes — both FFmpeg-based:

**Precise mode** (default): Two-pass `loudnorm` filter for metrically accurate
LUFS output. Best for podcasts and broadcast targets.

**Dynamics-preserving mode**: Single-pass `volume` + `alimiter` chain. Maintains
musical dynamics for DJ mixes and music content.

**Silence trimming**: `FFprobe` duration detection + `silencedetect` filter
removes leading/trailing dead air before normalization.

Pipeline stages: Upload → S3 → Validate → Analyze → Queue → Normalize → Store → Download

Progress is tracked via FFmpeg stderr parsing and broadcast to the client via
server-sent events.

---

## Business Model

Freemium SaaS. Processing is measured in **audio-hours**, not file count.

| Tier | Price | Monthly Processing |
|---|---|---|
| Free | €0 | 2 hours |
| Premium | €9/mo or €90/yr | 10 hours |
| Professional | €24/mo or €240/yr | 40 hours |
| Enterprise | Contact | Custom |

Both paid tiers include a 7-day free trial. Subscription tiers are synced from
Stripe webhooks to `users.subscription_tier` in Turso.

---

## Quick Start

### Prerequisites

- Go 1.24+
- FFmpeg installed on your system
- Redis server
- AWS S3 bucket
- Turso database account

### Installation

```bash
# Clone the repository
git clone https://github.com/simonlewi/levelmix.git
cd levelmix

# Install dependencies
go mod download

# Configure environment
cp .env.example .env
# Edit .env with your credentials

# Apply database schema
turso db create levelmix-dev
turso db shell levelmix-dev < ee/storage/sql/schema.sql

# Start Redis
redis-server

# Start the web server
go run -tags ee ./core/cmd/server

# Start the worker (separate terminal)
go run -tags ee ./core/cmd/worker

# Visit http://localhost:8080
```

### Enterprise Setup

The `ee/` directory is not included. To run the full application, implement:

- `ee/storage/` — `AudioStorage` and `MetadataStorage`
  (see `pkg/storage/interfaces.go`)
- `ee/auth/` — Authentication middleware and handlers
- `ee/payment/` — Stripe payment processing
- `ee/cleanup/` — S3 lifecycle and consent cleanup

Each `core/cmd/*/run_ee.go` file shows exactly which `ee/` packages are imported
and how they're wired up. Use those as your implementation reference.

---

## Contributing

Contributions are welcome on the open-source components (`core/`, `pkg/`).

```bash
# Fork the repo and create a feature branch
git checkout -b feature/your-feature

# Install dependencies and linter
go mod download
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run tests
go test ./...
go test -race ./...

# Check formatting and lint
go fmt ./...
golangci-lint run

# Submit a pull request with a clear description
```

**Good areas to contribute:**
- Additional audio format support (FLAC, AAC, OGG)
- Audio processing performance improvements
- Test coverage expansion
- Documentation improvements
- UI/UX refinements

### Getting Help

- **Issues:** Bug reports and feature requests via GitHub Issues
- **Discussions:** Community questions and ideas via GitHub Discussions

---

## License

`core/` and `pkg/` are licensed under the **Apache License 2.0**.

The `ee/` directory (not included) is proprietary and all rights reserved.

See [LICENSE](./LICENSE) for full terms.

---

## About

Built and maintained by [Simon](https://github.com/simonlewi) /
[Tricode Digital AB](https://levelmix.io), Sweden.

LevelMix is an independent SaaS product. The open-core model means the audio
processing pipeline, job queue architecture, and HTTP handlers are publicly
auditable — enterprise authentication, payments, and storage are kept private.
