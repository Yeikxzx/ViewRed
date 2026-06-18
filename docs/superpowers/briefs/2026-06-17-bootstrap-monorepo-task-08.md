# Task 8 Brief: Add Makefile for cross-platform builds

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 8 of 12** in the ViewRed bootstrap. Tasks 1-7 are done (Go backend, config, web rebranded, LLM client, Vite proxy). This task is a **mechanical refactor** — no TDD, just a config file with verification.

Your job: create a top-level `Makefile` that orchestrates the build, test, and dev workflow cross-platform. The Makefile should work on both Linux (where `make` is standard) and Windows (via Git Bash or WSL).

### Interfaces and decisions from earlier tasks

- **Backend**: Go module at `github.com/viewred/viewred`. Binary at `./cmd/viewred`. Built with `go build`.
- **Web**: in `web/`, uses pnpm (or npm) and Vite.
- **Version constant**: `Version` in `cmd/viewred/main.go` (set to `0.1.0-dev` by default). The Makefile passes `VERSION` via `-ldflags` to override at build time.
- **Cross-platform note**: You are on Windows. The Makefile uses `$(OS)` detection which works in Git Bash and WSL. If `make` is not on your PATH, install it or use Git Bash.

### Files already in the directory (DO NOT TOUCH)

- All existing files (Go, web, docs, .gitignore, etc.) — KEEP
- You are only CREATING the `Makefile`

---

## Task: Add Makefile for cross-platform builds

**Files:**
- Create: `Makefile` (at project root)

- [ ] **Step 1: Write Makefile**

Create `C:\Users\Yeikxz\Desktop\ViewRed\Makefile` with the content below.

```makefile
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

.PHONY: help build build-all test test-go test-web dev clean lint

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
```

Note: Use a tab character (NOT spaces) to indent the recipes under each target. Make is whitespace-sensitive.

- [ ] **Step 2: Verify Makefile works on current OS**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
make help
```

Expected: prints the help text exactly as defined in the Makefile.

If `make` is not on your PATH, run via Git Bash (it includes make):
```bash
"C:\Program Files\Git\bin\bash.exe" -c "cd /c/Users/Yeikxz/Desktop/Viewred && make help"
```

Or skip this step and just commit — the file is correct by inspection.

- [ ] **Step 3: Verify build works**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
make build
./viewred.exe --version
```

Expected: prints `viewred 0.1.0-dev`.

The build produces `viewred.exe` (Windows) or `viewred` (Linux). The binary exits after printing the version — no server stays running, no cleanup needed.

If the build is slow, that's normal (Go compile can take 5-15 seconds).

- [ ] **Step 4: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add Makefile
git status
git commit -m "chore: add cross-platform Makefile for build, test, dev"
```

---

## Verification

Before reporting back, verify:

1. `Makefile` exists at the project root with the content from the brief.
2. `make help` prints the help text (or you've inspected the file and confirmed it's correct).
3. `make build` succeeds; `./viewred.exe --version` prints `viewred 0.1.0-dev`.
4. Single commit on `main` with the expected message.

**NOTE**: After `make build`, the binary `viewred.exe` is created at the project root. This is fine — it's gitignored. You do NOT need to clean it up before committing.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-08.md`

Include:
- What you implemented
- `make help` output (or confirmation that you inspected the file)
- `make build` output + `./viewred.exe --version` output
- Commit SHA + subject
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line summary
- Your concerns, if any
- The report file path

## When you're stuck

If `make` isn't available and you can't run it, OR if the build fails, **stop and ask** with NEEDS_CONTEXT. Don't guess.