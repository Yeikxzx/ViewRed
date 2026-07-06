# ViewRed — cross-platform build orchestration.
# Tested on Linux and Windows (via Git Bash / WSL). PowerShell users can
# invoke go, pnpm, and docker directly.

BINARY_NAME := viewred
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

LDFLAGS := -s -w \
	-X github.com/viewred/viewred/internal/version.Version=$(VERSION) \
	-X github.com/viewred/viewred/internal/version.Commit=$(COMMIT) \
	-X github.com/viewred/viewred/internal/version.BuildDate=$(BUILD_DATE)

# Cross-compile targets
GOOS ?=
GOARCH ?=

# Detect platform if not set
ifeq ($(OS),Windows_NT)
	PLATFORM := windows
	EXE_EXT := .exe
else
	PLATFORM := linux
	EXE_EXT :=
endif

.PHONY: help build build-all build-release build-release-nocgo test test-go test-web dev dev-backend dev-web clean lint embed-prepare web-build package-linux release

help:
	@echo "ViewRed $(VERSION) — make targets:"
	@echo "  make build           Build binary for current OS"
	@echo "  make build-all       Cross-compile Windows + Linux binaries"
	@echo "  make build-release   Full release build: web + binaries (CGO for Windows)"
	@echo "  make build-release-nocgo  Release build without CGO (Linux-only, for CI)"
	@echo "  make release         Full release: build-release + Linux packages + checksums"
	@echo "  make test            Run all tests (Go + web)"
	@echo "  make test-go         Run Go tests only"
	@echo "  make test-web        Run web tests only"
	@echo "  make dev             Run Go backend + Vite dev server"
	@echo "  make lint            Run all linters"
	@echo "  make clean           Remove build artifacts"
	@echo "  make web-build       Build web frontend (pnpm build)"
	@echo "  make package-linux   Build .deb and .rpm packages (requires nfpm)"

# --- Embed preparation ---
# Copies web/dist and migrations into cmd/viewred/ so Go embed can find them.
embed-prepare:
	@echo "Preparing embedded assets..."
	@rm -rf cmd/viewred/webdist cmd/viewred/migrations 2>/dev/null || true
	@if [ -d web/dist ]; then cp -r web/dist cmd/viewred/webdist; else echo "WARNING: web/dist not found — run 'cd web && pnpm build' first"; fi
	@cp -r internal/storage/migrations cmd/viewred/migrations

# --- Build web frontend ---
web-build:
	cd web && pnpm build

# --- Single-platform build ---
build: embed-prepare
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)$(EXE_EXT) ./cmd/viewred

build-all:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/viewred
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64      ./cmd/viewred

# --- Release build: web + binaries ---
# Does NOT cross-compile with CGO (ONNX Runtime needs CGO on Windows).
# For full release with ML support, use 'build-release-cgo'.
build-release: clean web-build embed-prepare
	@echo "=== Building ViewRed $(VERSION) release ==="
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/viewred
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64      ./cmd/viewred

# --- Release without CGO (Linux-only, for CI without C compiler) ---
build-release-nocgo: clean web-build embed-prepare
	@echo "=== Building ViewRed $(VERSION) release (no CGO) ==="
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/viewred
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64      ./cmd/viewred

# --- Package Linux packages (requires nfpm CLI) ---
package-linux:
	@echo "Packaging Linux .deb and .rpm..."
	VERSION=$(VERSION) nfpm package --config nfpm.yaml --packager deb --target dist/
	VERSION=$(VERSION) nfpm package --config nfpm.yaml --packager rpm --target dist/

# --- Full release: binaries + Linux packages + checksums ---
release: build-release package-linux
	cd dist && sha256sum viewred-* viewred_* > checksums.txt
	@echo "=== Release $(VERSION) complete ==="
	@echo "Artifacts in dist/:"
	@ls -la dist/
	@echo ""
	@echo "Windows NSIS installer (requires makensis):"
	@echo "  makensis -DVERSION=$(VERSION) installer/windows/viewred.nsi"

# --- Testing ---
test: test-go test-web

test-go:
	go test ./...

test-web:
	cd web && pnpm test

# --- Development ---
dev:
	@echo "Run in two terminals:"
	@echo "  Terminal 1: make dev-backend"
	@echo "  Terminal 2: make dev-web"

dev-backend:
	go run ./cmd/viewred

dev-web:
	cd web && pnpm dev

# --- Linting ---
lint:
	go vet ./...
	cd web && pnpm lint

# --- Cleanup ---
clean:
	rm -rf $(BINARY_NAME) $(BINARY_NAME).exe dist/ web/dist/ web/node_modules/ cmd/viewred/webdist/ cmd/viewred/migrations/
