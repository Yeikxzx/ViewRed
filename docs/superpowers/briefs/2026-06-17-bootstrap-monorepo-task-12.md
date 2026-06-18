# Task 12 Brief: Final integration verification

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 12 of 12** in the ViewRed bootstrap (the LAST task). Tasks 1-11 are done.

Your job: **verify that everything works end-to-end**. No code changes expected — only running commands and reporting results. If you find issues, fix them in this task (or report BLOCKED if the issue is too big).

### Lessons from previous tasks

**The Task 7 e2e smoke test timed out** when the implementer tried to start both Go backend and Vite dev server as background processes, then test them together. The implementer never recovered.

For Task 12, do the smoke test in a **simpler, sequential way** that doesn't risk the same timeout:

1. Run all static verifications first (tests, builds, cross-compile) — these are fast and never hang.
2. For the live server test, do JUST the Go backend (start, curl, kill by port). Skip the full Vite + browser test — the user will verify that themselves when they use the app.

### Files already in the directory (DO NOT TOUCH except as needed)

- All existing files — KEEP
- You may need to commit at the end if you make any tweaks (e.g., killing stuck processes doesn't count, but fixing a typo in a file would)

---

## Task: Final integration verification

**Files:** none (verification only — but you may need to fix and commit small issues if you find them)

### Phase 1: Static verifications (fast, do these first)

- [ ] **Step 1: Run full Go test suite**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./... 2>&1 | Tee-Object -FilePath test-output.log
```

Expected: 8/8 tests pass across 4 packages (`cmd/viewred`, `internal/api`, `internal/api/handlers`, `internal/config`).

If anything fails, **STOP and report BLOCKED**. Do not try to fix unrelated test failures.

- [ ] **Step 2: Run web test suite**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed\web
pnpm test 2>&1 | Tee-Object -FilePath test-output.log
```

Expected: 2/2 tests pass.

If anything fails, **STOP and report BLOCKED**.

- [ ] **Step 3: Build for current OS**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go build -o viewred.exe ./cmd/viewred
```

Expected: `viewred.exe` created at project root (gitignored, no commit needed).

- [ ] **Step 4: Cross-compile both targets**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
New-Item -ItemType Directory -Path dist -Force
GOOS=linux GOARCH=amd64 go build -o dist/viewred-linux-amd64      ./cmd/viewred
GOOS=windows GOARCH=amd64 go build -o dist/viewred-windows-amd64.exe ./cmd/viewred
```

Expected: both binaries created in `dist/`.

- [ ] **Step 5: Verify the binary works**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
./viewred.exe --version
```

Expected: prints `viewred 0.1.0-dev`. The binary should exit after printing (no cleanup needed).

### Phase 2: Live server test (the tricky one)

- [ ] **Step 6: Clean up any stuck processes FIRST**

```powershell
# Before starting anything new, kill any leftover viewred/go processes
Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
Get-Process -Name "viewred","go" -ErrorAction SilentlyContinue | Where-Object { $_.StartTime -gt (Get-Date).AddMinutes(-30) } | Stop-Process -Force -ErrorAction SilentlyContinue
```

Expected: nothing was running, this is just defensive cleanup.

- [ ] **Step 7: Run the Go backend with strict timeout**

Use `Start-Process` to start the server in the background, then test, then kill by port.

```powershell
# Start the Go backend in the background
cd C:\Users\Yeikxz\Desktop\ViewRed
Start-Process -FilePath ".\viewred.exe" -NoNewWindow
Start-Sleep -Seconds 3   # Give the server 3 seconds to start
```

- [ ] **Step 8: Verify the server is listening**

```powershell
Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
```

Expected: shows a row with `LocalPort: 8080`. If empty, the server didn't start — report BLOCKED.

- [ ] **Step 9: Test the API endpoints**

```powershell
# Test /api/v1/health directly
$health = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/health" -Method Get
$health
# Expected: status: "ok", uptime_seconds: <number>

# Test /api/v1/health through config (set VIEWRED_CONFIG to a non-existent path to test defaults)
$env:VIEWRED_CONFIG = "C:\nonexistent\config.yaml"
# (But we already have the server running with default config — re-test the same endpoint)
$health2 = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/health" -Method Get
$health2
$env:VIEWRED_CONFIG = $null

# Test that a non-existent path returns 404 JSON (not a 5xx)
$notFound = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/nonexistent" -Method Get
$notFound
# Expected: error: "not_found", message: "Web UI not built..."

# Test that a non-GET method on /api/v1/health returns 404 (Gin default, not 405 — we accepted this in Task 3)
$notAllowed = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/v1/health" -Method Post -UseBasicParsing
$notAllowed.StatusCode
# Expected: 404 (NOT 405 — this is the documented Gin default we accepted in Task 3)
```

Expected: all four calls return JSON with the expected fields.

- [ ] **Step 10: Clean up the server (port-based kill, NOT name-based)**

```powershell
# Kill by port owner — this works even when the process name doesn't match
Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
Get-Process -Name "viewred" -ErrorAction SilentlyContinue | Where-Object { $_.StartTime -gt (Get-Date).AddMinutes(-10) } | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

# Verify nothing is left
Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
Get-Process -Name "viewred" -ErrorAction SilentlyContinue
```

Expected: both `Get-NetTCPConnection` and `Get-Process` return empty (no output).

- [ ] **Step 11: Commit the test outputs (if any) and binary artifacts**

The `viewred.exe` and `dist/` binaries are gitignored. No commit needed for them. The test output logs are also gitignored. **No commit is needed** if everything works.

If you had to fix anything, commit those fixes:
```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git status
git add -A
git commit -m "chore: plan 1 final integration tweaks" --allow-empty
```

---

## Verification

Before reporting back, all of these must be true:

1. `go test ./...` — 8/8 pass.
2. `pnpm test` (in web/) — 2/2 pass.
3. `viewred.exe` exists at project root.
4. `dist/viewred-linux-amd64` and `dist/viewred-windows-amd64.exe` exist.
5. `./viewred.exe --version` prints `viewred 0.1.0-dev`.
6. Server starts on :8080 when run.
7. `GET /api/v1/health` returns `{"status":"ok","uptime_seconds":N}`.
8. Server is killed cleanly after the test (no leftover processes).
9. `git status` is clean (or has only untracked files: `.atl/`, `que es viewred.md`, etc.).

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-12.md`

Include:
- Results of each step (test counts, build outputs, smoke test JSON responses)
- The cleanup verification (port + process checks after kill)
- Any issues found and how you fixed them
- Final `git status` output
- Commit SHA if you made any changes
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject, or "no commit needed")
- One-line summary (e.g. "8+2 tests pass, both binaries built, server smoke test OK")
- Your concerns, if any
- The report file path

## When you're stuck

If `go test` or `pnpm test` fails for reasons you can't fix in this task, **STOP and report BLOCKED**. Do not try to fix unrelated issues — this is a verification task, not an implementation task.

If the smoke test hangs or processes won't die, **kill them by port** (per Step 10), report what happened, and continue.