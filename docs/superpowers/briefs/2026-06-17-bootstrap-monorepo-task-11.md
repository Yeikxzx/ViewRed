# Task 11 Brief: Write top-level README

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 11 of 12** in the ViewRed bootstrap. Tasks 1-10 are done (Go backend, web/, Makefile, docker-compose, CI).

Your job: create the top-level `README.md` with quick start, build, project structure, and links to other docs. **Note**: I'm sending you a corrected version of the brief because the plan's original README references stale Go 1.22+ and Node 20+ (both EOL as of June 2026). Use the corrected toolchain versions below.

### Interfaces and decisions from earlier tasks

- **Backend**: Go module at `github.com/viewred/viewred`. Runs on `127.0.0.1:8080`.
- **Web**: in `web/`, runs on `127.0.0.1:3000` (dev) and proxies `/api/*` to backend.
- **Toolchain (current as of June 2026)**: Go 1.25+ (project has 1.26), Node 22+ (user has 24), pnpm 9+ (project uses pnpm), make (optional — user doesn't have it yet).
- **No TDD** — this is a docs file.
- **License**: MIT (per spec).

### Files already in the directory (DO NOT TOUCH)

- All existing files — KEEP
- You are only creating the top-level `README.md`

---

## Task: Write top-level README

**Files:**
- Create: `README.md` (at project root)

- [ ] **Step 1: Write README**

Create `C:\Users\Yeikxz\Desktop\ViewRed\README.md` with the content below. **Use this version, NOT the one in the plan** (the plan has stale toolchain versions):

```markdown
# ViewRed

Network Detection & Response (NDR) tool with local AI, for small teams managing 20-100 device networks.

> Status: V1 in development. Plan 1 of 8 (bootstrap) is complete. See [the docs/superpowers/plans/ directory](./docs/superpowers/plans/) for upcoming work.

## What is ViewRed?

ViewRed listens to your network traffic passively, detects anomalies and threats with local ML models, runs vulnerability scans, and explains what's happening through a local LLM. Everything runs on your own machine. No data leaves the network.

See [`que es viewred.md`](./que es viewred.md) for the full product spec and [`docs/superpowers/specs/`](./docs/superpowers/specs/) for the technical design.

## Quick start (dev)

Prerequisites: Go 1.25+, Node 22+, pnpm 9+, optionally Docker (for the dev DB), optionally `make` (Git Bash or WSL on Windows).

```bash
# 1. Clone and set up
git clone <repo-url> viewred
cd viewred

# 2. Install web dependencies
cd web && pnpm install && cd ..

# 3. Run tests
#    Go tests:
go test ./...
#    Web tests:
cd web && pnpm test
#    (or use `make test` if you have make installed)

# 4. Start dev (two terminals)
# Terminal 1: Go backend on :8080
go run ./cmd/viewred
# Terminal 2: Vite dev server on :3000
cd web && pnpm dev
```

Open http://127.0.0.1:3000 in your browser. The Go backend serves `/api/v1/health` directly at http://127.0.0.1:8080/api/v1/health.

## Build

```bash
# Build binary for current OS
go build -o viewred.exe ./cmd/viewred

# Or use make (if installed)
make build         # binary for current OS
make build-all     # cross-compile to dist/

# Or cross-compile directly
GOOS=linux GOARCH=amd64 go build -o dist/viewred-linux-amd64      ./cmd/viewred
GOOS=windows GOARCH=amd64 go build -o dist/viewred-windows-amd64.exe ./cmd/viewred
```

## Project structure

```
cmd/viewred/         Go entry point
internal/            Go packages (api, config, ...)
web/                 React + Vite frontend
docs/superpowers/    Specs and plans
deploy/              Docker compose and deployment artifacts
.github/workflows/   CI (GitHub Actions matrix Windows + Linux)
```

## Documentation

- [`que es viewred.md`](./que es viewred.md) — product definition
- [`design.md`](./design.md) — UI/UX design spec
- [`docs/superpowers/specs/`](./docs/superpowers/specs/) — technical specs
- [`docs/superpowers/plans/`](./docs/superpowers/plans/) — implementation plans

## License

MIT.
```

- [ ] **Step 2: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add README.md
git status
git commit -m "docs: add top-level README with quick start and structure"
```

---

## Verification

Before reporting back, verify:

1. `README.md` exists at the project root with the brief content (note: the toolchain versions are 1.25/22/pnpm, NOT the stale 1.22/20 from the plan).
2. Single commit on `main` with the expected message.
3. The README references all the right files (`que es viewred.md`, `design.md`, `docs/superpowers/`).

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-11.md`

Include:
- What you implemented
- `git status` before and after the commit
- Commit SHA + subject
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line summary
- Your concerns, if any
- The report file path

## When you're stuck

If anything unusual, **stop and ask** with NEEDS_CONTEXT.