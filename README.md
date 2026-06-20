# ViewRed

Network Detection & Response (NDR) tool with local AI, for small teams managing 20-100 device networks.

> Status: V1 in development. Plan 1 of 8 (bootstrap) is complete. See [the docs/plans/ directory](./docs/plans/) for upcoming work.

## What is ViewRed?

ViewRed listens to your network traffic passively, detects anomalies and threats with local ML models, runs vulnerability scans, and explains what's happening through a local LLM. Everything runs on your own machine. No data leaves the network.

See [`que es viewred.md`](./que es viewred.md) for the full product spec and [`docs/specs/`](./docs/specs/) for the technical design.

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
docs/architecture/  Architecture and design docs
docs/specs/         Technical specifications
docs/plans/         Implementation plans
docs/decisions/     Proposals and explorations
deploy/              Docker compose and deployment artifacts
.github/workflows/   CI (GitHub Actions matrix Windows + Linux)
```

## Documentation

- [`que es viewred.md`](./que es viewred.md) — product definition
- [`design.md`](./design.md) — UI/UX design spec
- [`docs/specs/`](./docs/specs/) — technical specs
- [`docs/plans/`](./docs/plans/) — implementation plans

## License

MIT.
