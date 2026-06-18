# Task 8 Report: Add Makefile for cross-platform builds

**Date**: 2026-06-17

## What was implemented

Created `Makefile` at project root with cross-platform build orchestration targets:

- **build**: Go binary for current OS (with `-ldflags` for version)
- **build-all**: Cross-compile for Windows + Linux amd64
- **test**, **test-go**, **test-web**: Test runners
- **dev**, **dev-backend**, **dev-web**: Development servers
- **lint**: Go vet + web lint
- **clean**: Remove build artifacts
- **help**: Print available targets

## Verification

### `make help`

`make` is **not available** on this Windows system (not on PATH, no Git Bash, no Chocolatey with admin rights). The Makefile was verified by inspection — all target names, recipe lines, and variable references match the brief.

### `make build` (manual equivalent)

```
> go build -ldflags "-X main.Version=0.1.0-dev" -o viewred.exe ./cmd/viewred
> .\viewred.exe --version
viewred 0.1.0-dev
```

The Go build succeeds. The binary exits after printing the version (no cleanup needed).

### Tab verification

Confirmed via hex inspection: all recipe lines under targets use TAB characters (ASCII 0x09), not spaces. The Makefile will parse correctly under GNU Make.

## Commit

```
618e5fe chore: add cross-platform Makefile for build, test, dev
```

## Concerns

- `make` is not on PATH on this Windows machine. The Makefile is syntactically verified (tabs correct) and functionally correct by inspection, but couldn't run `make help` or `make build` directly. If the user needs `make` locally, they should install it via Git Bash, WSL, or `choco install make` (as admin).
