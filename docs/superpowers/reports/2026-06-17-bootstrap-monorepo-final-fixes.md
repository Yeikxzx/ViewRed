# Bootstrap Monorepo — Final Fixes Report

**Date:** 2026-06-17

## Summary

Applied 1 Critical fix, 5 Important fixes. 6 Minor items deferred.

## Commits

| SHA | Subject |
|-----|---------|
| `d05e7b0` | `fix(ci): bump Go to 1.26 to match go.mod` |
| `1c06275` | `docs: add product and UI design documents to VCS` |
| `7f74b1a` | `fix(web): clean up dependencies add merge-yield test` |
| `8f4de31` | `fix(makefile): add dev-backend and dev-web to .PHONY` |

## Changes by Fix

### CRITICAL #1 — CI Go version mismatch

- **File:** `.github/workflows/ci.yml:25`
- **Change:** `go-version: '1.25'` → `go-version: '1.26'`
- **Why:** `go.mod` requires go 1.26.3; CI would fail at the first `go test`.
- **Verified:** `go test ./...` passes (4 packages, all OK).

### IMPORTANT #2 — Uncommitted docs

- **Files:** `que es viewred.md`, `design.md`
- **Change:** `git add` and committed.
- **Why:** Both files are linked from README.md. Without them, links are broken on fresh clone.
- **Note:** `docs/` and `openspec/` directories remain untracked per instruction scope.

### IMPORTANT #3 — Unused `type Message` import

- **File:** `web/src/components/AsistenteView.tsx:4`
- **Change:** `import { llm, type Message } from '../lib/llm'` → `import { llm } from '../lib/llm'`
- **Why:** Only `llm` is used in this file. The unused `Message` type would cause `tsc --noEmit` failure under strict settings.
- **Verified:** `pnpm test` passes.

### IMPORTANT #4 — Duplicate `@vitejs/plugin-react`

- **File:** `web/package.json:26`
- **Change:** Removed from `dependencies` (kept in `devDependencies` at line 48).
- **Why:** A Vite plugin belongs in devDeps only.
- **Verified:** `pnpm install` ran successfully, lockfile updated, `pnpm test` passes.

### IMPORTANT #5 — Missing `.PHONY` targets

- **File:** `Makefile:22`
- **Change:** Added `dev-backend` and `dev-web` to the `.PHONY` line.
- **Why:** These targets exist in the file but were not declared phony, which could cause spurious "up to date" errors if a file with that name existed.

### IMPORTANT #6 — Partial YAML merge not tested

- **File:** `internal/config/config_test.go`
- **Change:** Added 3 assertions after the existing field checks:
  ```go
  assert.Equal(t, 5, cfg.Scanner.MaxConcurrentScans)
  assert.Equal(t, 0.85, cfg.AI.MLAnomalyThreshold)
  assert.Equal(t, 600, cfg.Scanner.ScanTimeoutSeconds)
  ```
- **Why:** The test verified only the fields that the YAML override sets, missing the merge-contract: unset fields must retain defaults.
- **Verified:** `go test ./internal/config/...` passes.

## Test Results

```
# Go tests
ok  github.com/viewred/viewred/cmd/viewred       1.560s
ok  github.com/viewred/viewred/internal/api       1.784s
ok  github.com/viewred/viewred/internal/api/handlers  1.606s
ok  github.com/viewred/viewred/internal/config    1.931s

# Web tests
 Test Files  1 passed (1)
      Tests  2 passed (2)
```

## Deferred Items (Minor)

The following Minor items were NOT fixed in this pass, per instructions:

| # | Issue | Why deferred |
|---|-------|-------------|
| 8 | Stale AI Studio artifacts (`web/metadata.json`, `web/assets/.aistudio/.gitignore`) | Defer to plan 2 |
| 9 | `**/.env` redundant with `.env` in gitignore | Trivial but not part of this fix pass |
| 10 | `indent_size = 4` for `[*.go]` in editorconfig | Style, not functional |
| 11 | `rm -rf` in Makefile `clean` without safety check | Accepted tradeoff per plan |
| 12 | Missing package doc comment in `internal/config/config.go` | Code style, not functional |
| 13 | `--version`/`-v` flag handling untested | Would need new test file, beyond scope |

## Concerns

- `pnpm install` auto-upgraded several indirect deps (React 19.2.7, Vite 6.4.3, etc.) due to the resolution changes from removing the duplicate entry. Lockfile was updated as a side effect. This is safe — same semver ranges — but worth noting.
- The `docs/` and `openspec/` directories remain untracked. If the README links to them, those links are still broken on fresh clone. Adding them was not in scope for this fix.

---

**Status:** DONE
