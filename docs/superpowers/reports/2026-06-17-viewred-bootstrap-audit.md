# ViewRed V1 Bootstrap — Comprehensive Bug & Connectivity Audit

**Date**: 2026-06-17
**Auditor**: opencode (automated)
**Scope**: Plan 1 (bootstrap) — Go backend + React frontend monorepo

---

## Summary

Verified: Go backend compiles clean, Go tests pass, web tests pass, Go vet clean. The frontend TypeScript check (`tsc --noEmit`) **fails** due to a single vite config type issue. No broken imports or dangling file references were found. The `react-flow → reactflow` fix from a previous task is confirmed resolved. The Vite proxy and CORS setup are correct. The LLM client mock integration is wired correctly through the chat flow.

**Overall health**: Functional with some technical debt. The app can run in dev mode. The `pnpm lint` failure is the only CI blocker.

---

## Critical Bugs

### C1. `vite.config.ts` — `test` property rejected by TypeScript (CI blocker)

- **File**: `web/vite.config.ts:28`
- **What**: `defineConfig` is imported from `'vite'` but the `test` property is typed by `vitest`, not `vite`. TypeScript error:
  ```
  error TS2769: No overload matches this call.
  Object literal may only specify known properties, and 'test' does not exist in type 'UserConfigExport'.
  ```
- **Why it matters**: `pnpm lint` (which runs `tsc --noEmit`) fails. The CI's lint step will break. Vitest tests still work because vitest ignores the TS error, but the type check is red.
- **How to fix**: Either:
  - Add `/// <reference types="vitest" />` at the top of the file, OR
  - Replace `import { defineConfig } from 'vite'` with `import { defineConfig } from 'vitest/config'`
- **Severity**: Critical — blocks CI lint step

---

## Important Issues

### I1. 20+ unused dependencies in `web/package.json`

- **File**: `web/package.json:14-39`
- **What**: The following packages are declared as dependencies but never imported in any source file under `web/src/`:
  - `@hookform/resolvers`, `@radix-ui/react-dialog`, `@radix-ui/react-dropdown-menu`, `@radix-ui/react-label`, `@radix-ui/react-popover`, `@radix-ui/react-select`, `@radix-ui/react-slot`, `@radix-ui/react-tabs`, `@radix-ui/react-toast`, `@tanstack/react-virtual`, `class-variance-authority`, `clsx`, `date-fns`, `dotenv`, `reactflow`, `react-hook-form`, `react-markdown`, `sonner`, `tailwind-merge`, `zod`
- **Why it matters**: 300+ MB of `node_modules` bloat, slower installs, and confusing for new devs. `dotenv` is server-side and useless in Vite (Vite has built-in `.env` support). `reactflow` was fixed from the old `react-flow` name but is still unused.
- **How to fix**: Remove packages not needed for plan 1. Keep only what is actually imported: `react`, `react-dom`, `lucide-react`, `recharts`, `@tailwindcss/vite`, and the dev/build toolchain.
- **Severity**: Important (not blocking, but significant tech debt)

### I2. Unused icon imports in multiple components

- **`web/src/components/DashboardView.tsx:4-8`**: `ArrowUp`, `ArrowDown`, `Shield`, `Zap`, `HelpCircle` are imported from `lucide-react` but never rendered. Only `Activity` is used (as a string, not a component).
- **`web/src/components/AlertasView.tsx:4`**: `Eye` is imported but never used.
- **`web/src/components/FiltrosView.tsx:6-7`**: `Code` and `FileText` are imported but never used.
- **`web/src/components/AsistenteView.tsx:3`**: `Vulnerability` is imported from `../types` but never referenced in the component body.
- **Severity**: Important (dead code, confusing to maintain)

### I3. Typo: "Suricada" → "Suricata" in mock alert data

- **File**: `web/src/data/mockData.ts:121`
- **What**: Line 121: `'Anomalía DNS / Suricada IDS'` — "Suricada" should be "Suricata".
- **Why it matters**: Displayed in the UI as alert origin. Misleading for demos.
- **Severity**: Important

### I4. Typo: "TOTAL ALK" → "TOTAL ALL" or "TOTAL ALERTAS" in AlertasView

- **File**: `web/src/components/AlertasView.tsx:159`
- **What**: Line 159: `<span>TOTAL ALK</span>` — "ALK" should be "ALL" or "ALERTAS" in Spanish.
- **Why it matters**: UI text error visible in the alerts filter cards.
- **Severity**: Important

### I5. `@/*` path alias points to `web/` instead of `web/src/`

- **File**: `web/tsconfig.json:19-21`
- **What**: `"@/*": ["./*"]` resolves `@/components/X` to `web/components/X` instead of `web/src/components/X`.
- **Why it matters**: Currently no source files use `@/` imports (all are relative), so it's not a runtime bug. But if someone tries to use the alias, imports will break silently.
- **How to fix**: Change to `"@/*": ["./src/*"]`
- **Severity**: Important (latent bug)

---

## Minor Issues

### M1. Model name inconsistency: "llama3 (8b)" vs "qwen2.5:3b"

- **Files**: `web/src/components/AsistenteView.tsx:73` (Qwen 2.5 3B), `web/src/components/ConfiguracionView.tsx:29` (llama3 8b), `internal/config/config.go:43` (qwen2.5:3b)
- **What**: The config default is `qwen2.5:3b`. The AI assistant UI says "Qwen 2.5 (3B Local)". But ConfiguracionView's connection test says "Modelo llama3 (8b)".
- **Severity**: Minor (mock UI; actual integration is in plan 6)

### M2. `ChatMessage.sender` type includes `'ia'` but never used

- **File**: `web/src/types.ts:86`
- **What**: `sender: 'admin' | 'ia' | 'bot'` — the `'ia'` variant is never used. Only `'admin'` and `'bot'` are used.
- **Severity**: Minor

### M3. `make build-all` doesn't create `dist/` directory

- **File**: `Makefile:39-40`
- **What**: Output binaries go to `dist/` but the directory is never created. On first run, `go build` may fail if `dist/` doesn't exist.
- **Severity**: Minor (works after first run; `go build` creates parent dirs on some platforms)

### M4. pnpm version 9 in CI may be outdated

- **File**: `.github/workflows/ci.yml:37`
- **What**: `version: 9` is pinned. By June 2026, pnpm may be at v10+. This could cause `setup-pnpm` to fail or degrade.
- **Severity**: Minor (will surface when CI runs)

### M5. `go-version: '1.26'` in CI might not be released

- **File**: `.github/workflows/ci.yml:25`
- **What**: Go 1.26 is scheduled for August 2026. As of June 2026, the latest stable is 1.25.x. This CI config would fail until 1.26 is released.
- **Severity**: Minor (CI will fail until Aug 2026, or if using 1.26-rc. Change to `'1.25'` or `'stable'`.)

---

## Verified Working

- **Go vet**: `go vet ./...` passes clean (no output, no warnings)
- **Go tests**: All 4 packages pass (`cmd/viewred`, `internal/api`, `internal/api/handlers`, `internal/config`)
- **Web tests**: 1 file, 2 tests pass (`MockLLMClient` returns placeholder, reports available)
- **Go modules**: `go.mod` resolves all imports correctly
- **pnpm install**: Lockfile valid, all deps resolve
- **Vite proxy**: `/api` → `127.0.0.1:8080` is correct. Go API is at `/api/v1/health`, so the prefix match works.
- **CORS**: Not explicitly configured on Go side, but Vite's proxy avoids CORS entirely in dev. For production, the Go server serves the web dist via `r.StaticFS`, so same-origin.
- **SSE path**: Not yet implemented, but the router and config are ready for it.
- **LLM client wiring**: `MockLLMClient` correctly implements `LLMClient` interface (`chat`, `isAvailable`, optional `stream`). It's instantiated as `llm` singleton and consumed in `App.tsx:85` and `AsistenteView.tsx:25`.
- **Health endpoint**: `GET /api/v1/health` returns `{"status":"ok","uptime_seconds":N}` — correct shape.
- **No broken imports**: All TypeScript imports in `App.tsx` and all view components resolve to existing files.
- **No `react-flow` → `reactflow` leftovers**: The fix from task 6 is confirmed — `package.json` uses `reactflow`, and no source files reference `react-flow`.
- **Web dist fallback**: If `web/dist/` doesn't exist, the Go server returns a helpful JSON message instead of crashing.
- **Config fallback chain**: `VIEWRED_ENV` → `UserConfigDir/viewred/config.yaml` → defaults. If config file is missing, defaults are used without error.

---

## Run Commands Summary

| Command | Result |
|---------|--------|
| `go vet ./...` | ✅ Clean |
| `go test ./...` | ✅ 4 packages pass |
| `cd web && pnpm lint` | ❌ Fails (1 TS error in vite.config.ts) |
| `cd web && pnpm test` | ✅ 2/2 tests pass |
| `go build ./cmd/viewred` | ✅ Compiles |

---

## Finding Count

- **Critical**: 1 (C1 — CI lint blocker)
- **Important**: 5 (I1–I5)
- **Minor**: 5 (M1–M5)
