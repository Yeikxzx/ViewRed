# ViewRed — cross-platform build orchestration.
# Tested on Linux and Windows (via Git Bash / WSL). PowerShell users can
# invoke go, pnpm, and docker directly.

BINARY_NAME := viewred
VERSION ?= 0.1.0-dev
LDFLAGS := -X main.Version=$(VERSION)

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

.PHONY: help build build-all test test-go test-web dev dev-backend dev-web clean lint

help:
	@echo "ViewRed $(VERSION) — make targets:"
	@echo "  make build         Build the Go binary for current OS"
	@echo "  make build-all     Cross-compile binaries for Windows + Linux"
	@echo "  make test          Run all tests (Go + web)"
	@echo "  make test-go       Run Go tests only"
	@echo "  make test-web      Run web tests only"
	@echo "  make dev           Run Go backend + Vite dev server"
	@echo "  make lint          Run all linters"
	@echo "  make clean         Remove build artifacts"

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)$(EXE_EXT) ./cmd/viewred

build-all:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/viewred
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64      ./cmd/viewred

test: test-go test-web

test-go:
	go test ./...

test-web:
	cd web && pnpm test

dev:
	@echo "Run in two terminals:"
	@echo "  Terminal 1: make dev-backend"
	@echo "  Terminal 2: make dev-web"

dev-backend:
	go run ./cmd/viewred

dev-web:
	cd web && pnpm dev

lint:
	go vet ./...
	cd web && pnpm lint

clean:
	rm -rf $(BINARY_NAME) $(BINARY_NAME).exe dist/ web/dist/ web/node_modules/
