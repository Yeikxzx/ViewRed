# Task 7 Brief: Configure Vite for dev proxy to Go backend

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 7 of 12** in the ViewRed bootstrap. Tasks 1-6 are done (Go backend, config, web/ rebranded, Gemini swapped for LLM client).

Your job: configure **Vite** to:
- Bind dev server to `127.0.0.1:3000` (already set, but make it explicit and `strictPort`).
- Proxy `/api/*` requests to the Go backend at `http://127.0.0.1:8080`.
- Set up vitest for the existing tests (we already have `web/src/lib/llm.test.ts` from Task 6).
- Verify the dev workflow (Go backend + Vite dev server + API proxy) works end-to-end.

### Interfaces and decisions from earlier tasks

- **Backend** is at `http://127.0.0.1:8080` (Go binary started via `go run ./cmd/viewred`).
- **Web dev server** will run on `http://127.0.0.1:3000` (Vite).
- **API prefix** is `/api/v1` (e.g., `/api/v1/health`).
- **No TDD** for this task — it's pure config. The verification is `pnpm test` (vitest runs) and the dev server starts.
- **Cross-platform note**: You are on Windows. PowerShell for commands. Make sure the Vite config uses `path.resolve(__dirname, ...)` which works on Windows (since Vite normalizes paths).

### Files already in the directory (DO NOT TOUCH except as specified)

- All Go files (`cmd/`, `internal/`) — KEEP
- `web/src/lib/llm.ts`, `web/src/lib/llm.test.ts` (from Task 6) — KEEP, these tests should pass after this task
- All other `web/` files — KEEP
- `web/package.json` — KEEP, do not modify
- `web/tsconfig.json` — KEEP

---

## Task: Configure Vite for dev proxy to Go backend

**Files:**
- Modify: `web/vite.config.ts` (replace with new config)
- Create: `web/src/test-setup.ts` (vitest setup)

- [ ] **Step 1: Replace vite.config.ts**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\web\vite.config.ts` with:

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    host: '127.0.0.1',
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
});
```

- [ ] **Step 2: Create test-setup.ts**

Create `C:\Users\Yeikxz\Desktop\ViewRed\web\src\test-setup.ts`:

```typescript
import '@testing-library/jest-dom';
```

- [ ] **Step 3: Run existing vitest tests to confirm setup works**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed\web
pnpm test
```

(Use `npm test` if pnpm isn't available.)

Expected: 2 tests from `llm.test.ts` PASS (`MockLLMClient returns a placeholder response`, `MockLLMClient reports as available`). This confirms the vitest config + test-setup file work.

- [ ] **Step 4: Verify dev workflow (Go backend + Vite + proxy)**

This is a multi-process test. On Windows with PowerShell, use the **port-based kill** approach (not name-based) for cleanup — this avoids the bug we hit in Task 3 where `Stop-Process -Name "viewred"` left a process running.

**In one terminal (or background job)**, start the Go backend:

```powershell
cd C:\Users\Yeikxz\Desktop\ViewRed
Start-Process -FilePath "go" -ArgumentList "run","./cmd/viewred" -NoNewWindow
```

**Wait 2-3 seconds** for it to start, then verify it's listening:

```powershell
Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
```

Expected: shows a row with `LocalPort: 8080`.

**In the SAME terminal**, start the Vite dev server (also in background):

```powershell
cd C:\Users\Yeikxz\Desktop\ViewRed\web
Start-Process -FilePath "pnpm" -ArgumentList "dev" -NoNewWindow
```

**Wait 3-5 seconds** for Vite to start, then verify it's listening:

```powershell
Get-NetTCPConnection -LocalPort 3000 -State Listen -ErrorAction SilentlyContinue
```

Expected: shows a row with `LocalPort: 3000`.

**Now test the proxy from the same terminal**:

```powershell
# Direct to backend (should return JSON)
$direct = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/health" -Method Get
$direct
# Expected: status: "ok", uptime_seconds: N

# Through Vite proxy (should ALSO return JSON, proxied to backend)
$proxied = Invoke-RestMethod -Uri "http://127.0.0.1:3000/api/v1/health" -Method Get
$proxied
# Expected: same JSON, proving the proxy works
```

**Cleanup (IMPORTANT — use port-based kill)**:

```powershell
# Find and kill the process listening on port 3000 (Vite)
Get-NetTCPConnection -LocalPort 3000 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }

# Find and kill the process listening on port 8080 (Go)
Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }

# Sometimes the parent go.exe / node.exe are still around — kill them too
Get-Process -Name "go","node","viewred" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

# Verify nothing is left on those ports
Get-NetTCPConnection -LocalPort 3000,8080 -State Listen -ErrorAction SilentlyContinue
```

Expected: empty output (no processes listening).

- [ ] **Step 5: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add web/vite.config.ts web/src/test-setup.ts
git status
git commit -m "feat(web): configure Vite dev server with API proxy and test setup"
```

---

## Verification

Before reporting back, verify:

1. `web/vite.config.ts` matches the brief exactly.
2. `web/src/test-setup.ts` exists with the brief content.
3. `pnpm test` (or `npm test`) passes — 2/2 tests.
4. The dev workflow works: Go on :8080 + Vite on :3000 + proxy returns the same JSON.
5. **No processes left running** after cleanup (verify with the final `Get-NetTCPConnection` call).
6. Single commit on `main` with the expected message.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-07.md`

Include:
- What you implemented
- Test result from `pnpm test`
- Output of the proxy verification (the actual JSON from `Invoke-RestMethod`)
- The cleanup result (was anything left running?)
- Commit SHA + subject
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line summary (e.g. "vitest 2/2 pass, proxy works, clean shutdown verified")
- Your concerns, if any
- The report file path

## When you're stuck

If the proxy doesn't work as expected, processes won't die, or the dev server fails to start, **stop and ask** with NEEDS_CONTEXT. Don't guess.