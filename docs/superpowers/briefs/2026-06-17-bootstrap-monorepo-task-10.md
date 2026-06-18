# Task 10 Brief: Add GitHub Actions CI

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 10 of 12** in the ViewRed bootstrap. Tasks 1-9 are done (Go backend, web/, Makefile, docker-compose).

Your job: create a GitHub Actions workflow file at `.github/workflows/ci.yml` that runs on every push/PR and tests the build on both Windows and Linux. Static file only — no execution needed (GitHub Actions runs the workflow when pushed).

### Interfaces and decisions from earlier tasks

- **Backend**: Go module at `github.com/viewred/viewred`. Tested via `go test ./...`.
- **Web**: pnpm-based Vite project in `web/`. Tested via `cd web && pnpm test`.
- **Go version**: 1.22+ (we have 1.26).
- **Node version**: 20+ (we have 24).
- **Cross-platform matrix**: ubuntu-latest + windows-latest.
- **No TDD** — this is a static file.

### Files already in the directory (DO NOT TOUCH)

- All existing files — KEEP
- You are only creating `.github/workflows/ci.yml`

---

## Task: Add GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write CI workflow**

Create `C:\Users\Yeikxz\Desktop\ViewRed\.github\workflows\ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    name: Test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, windows-latest]

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install web dependencies
        run: |
          cd web
          npm install

      - name: Run Go tests
        run: go test ./...

      - name: Run web tests
        run: |
          cd web
          npm test

      - name: Build (linux/amd64)
        if: matrix.os == 'ubuntu-latest'
        run: |
          GOOS=linux GOARCH=amd64 go build -o dist/viewred-linux-amd64 ./cmd/viewred

      - name: Build (windows/amd64)
        if: matrix.os == 'windows-latest'
        run: |
          GOOS=windows GOARCH=amd64 go build -o dist/viewred-windows-amd64.exe ./cmd/viewred
```

- [ ] **Step 2: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add .github/
git status
git commit -m "ci: add GitHub Actions matrix for Windows+Linux"
```

---

## Verification

Before reporting back, verify:

1. `.github/workflows/ci.yml` exists with the exact content from the brief.
2. Single commit on `main` with the expected message (`ci: add GitHub Actions matrix for Windows+Linux`).
3. YAML is valid (no syntax errors, no tab/space issues — YAML prefers spaces).

**Do NOT try to run the GitHub Actions workflow** — you don't have push access, and even if you did, the workflow runs on GitHub's infrastructure, not locally.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-10.md`

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