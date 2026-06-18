# Task 5 Brief: Move AI Studio template into web/ and rebrand

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 5 of 12** in the ViewRed bootstrap. Tasks 1, 2, 3, 4 are done (Go backend with health endpoint, YAML config).

The user already created a base UI template in `diseño-templete/diseño-base/` (an export from Google AI Studio). Your job: **move it into the standard `web/` location** and **rebrand it** to ViewRed (name, scripts, dev port, host binding). The Gemini integration is left intact in this task — Task 6 swaps it for a placeholder LLM client.

### Interfaces and decisions from earlier tasks

- **Module path**: `github.com/viewred/viewred` (do not change).
- **Existing Go backend**: serves on `127.0.0.1:8080`. Web dev server will run on `127.0.0.1:3000` and proxy `/api/*` to the backend (configured in Task 7).
- **Current template state**:
  - Located at `diseño-templete/diseño-base/`
  - Uses `@google/genai` (Gemini API) for the AsistenteView — KEEP for now, Task 6 will swap
  - Has `dev` script: `vite --port=3000 --host=0.0.0.0` — CHANGE to `127.0.0.1`
  - Has `name: "react-example"` — CHANGE to `viewred-web`
  - Has `<title>` likely as "AI Studio" or similar — CHANGE to "ViewRed"
- **Cross-platform note**: You are on Windows. Use PowerShell. The brief shows PowerShell commands for the move step.

### Files already in the directory (DO NOT TOUCH)

- `go.mod`, `go.sum`, `cmd/`, `internal/`, `docs/`, `diseño-templete/` (will be moved/deleted in this task)
- All other files: leave alone.

---

## Task: Move AI Studio template into web/ and rebrand

**Files:**
- Create: `web/` directory with all template files
- Delete: `diseño-templete/` directory (after move)
- Modify: `web/package.json` (replace with rebrand version)
- Modify: `web/index.html` (title only)
- Modify: `web/README.md` (replace with ViewRed-specific notes)

- [ ] **Step 1: Move template into web/**

```powershell
# From C:\Users\Yeikxz\Desktop\Viewred
New-Item -ItemType Directory -Path web -Force
Move-Item -Path "diseño-templete/diseño-base/*" -Destination web -Force
Remove-Item -Path "diseño-templete" -Recurse -Force
```

Verify with `Get-ChildItem web`. Expected: the template contents are now under `web/`, and `diseño-templete/` no longer exists.

Note: The PowerShell `Move-Item` with the wildcard path works in Windows. If for some reason it doesn't, an alternative is:

```powershell
Get-ChildItem -Path "diseño-templete/diseño-base" | ForEach-Object { Move-Item -Path $_.FullName -Destination "web" -Force }
Remove-Item -Path "diseño-templete" -Recurse -Force
```

- [ ] **Step 2: Replace web/package.json**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\web\package.json` with the content below. This rebrands the package, switches the dev host to `127.0.0.1`, adds lint/test scripts, and adds the libraries we'll need in later tasks.

```json
{
  "name": "viewred-web",
  "private": true,
  "version": "0.1.0-dev",
  "type": "module",
  "scripts": {
    "dev": "vite --port=3000 --host=127.0.0.1",
    "build": "tsc --noEmit && vite build",
    "preview": "vite preview",
    "lint": "tsc --noEmit",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "@google/genai": "^2.4.0",
    "@hookform/resolvers": "^3.9.1",
    "@radix-ui/react-dialog": "^1.1.2",
    "@radix-ui/react-dropdown-menu": "^2.1.2",
    "@radix-ui/react-label": "^2.1.0",
    "@radix-ui/react-popover": "^1.1.2",
    "@radix-ui/react-select": "^2.1.2",
    "@radix-ui/react-slot": "^1.1.0",
    "@radix-ui/react-tabs": "^1.1.1",
    "@radix-ui/react-toast": "^1.2.2",
    "@tailwindcss/vite": "^4.1.14",
    "@tanstack/react-virtual": "^3.10.9",
    "@vitejs/plugin-react": "^5.0.4",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.1",
    "date-fns": "^4.1.0",
    "dotenv": "^17.2.3",
    "lucide-react": "^0.546.0",
    "react": "^19.0.1",
    "react-dom": "^19.0.1",
    "react-flow": "^11.11.4",
    "react-hook-form": "^7.53.2",
    "react-markdown": "^9.0.1",
    "recharts": "^3.8.1",
    "sonner": "^1.7.0",
    "tailwind-merge": "^2.5.4",
    "zod": "^3.23.8"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.6.3",
    "@testing-library/react": "^16.0.1",
    "@types/node": "^22.14.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^5.0.4",
    "autoprefixer": "^10.4.21",
    "jsdom": "^25.0.1",
    "tailwindcss": "^4.1.14",
    "tsx": "^4.21.0",
    "typescript": "~5.8.2",
    "vite": "^6.2.3",
    "vitest": "^2.1.5"
  }
}
```

Notes on the dependencies:
- `motion` (Framer Motion) was in the original template but is NOT included here. The spec says no long animations. We don't need it for V1.
- `express` was in the original template as a dev server. We removed it — the Go backend serves the API in dev, and `go:embed` handles prod (later task).

- [ ] **Step 3: Update web/index.html title**

Edit `C:\Users\Yeikxz\Desktop\ViewRed\web\index.html` and change the `<title>` tag to read "ViewRed". Leave the rest of the file as-is.

- [ ] **Step 4: Replace web/README.md**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\web\README.md` with:

````markdown
# ViewRed Web UI

Vite + React 19 + TypeScript + Tailwind 4 frontend for ViewRed.

## Dev

```bash
pnpm install     # or: npm install
pnpm dev         # starts on http://127.0.0.1:3000
```

Vite proxies `/api/*` requests to `http://127.0.0.1:8080` where the Go backend runs.

## Build

```bash
pnpm build       # outputs to dist/
```

The Go binary embeds `dist/` via `go:embed` (configured in plan 3).

## Test

```bash
pnpm test        # runs Vitest
```
````

- [ ] **Step 5: Commit**

```powershell
cd C:\Users\Yeikxz\Desktop\ViewRed
git add web/ -A
git status  # verify the right files are staged
git commit -m "feat(web): move AI Studio template into web/ and rebrand to ViewRed"
```

Note: `diseño-templete/` was never committed (it was a working-directory-only directory), so we don't need `git rm` — we just stage `web/` and commit. The `diseño-templete/` directory is already deleted from the working tree by Step 1, and since it was never tracked, git doesn't care.

If you see `git status` listing files you don't expect (e.g., `.env` files from the original template that should not be committed), STOP and inspect. The original template has a `.env.example` which is fine to commit; a real `.env` would not be (it's gitignored).

---

## Verification

Before reporting back, verify:

1. `web/` exists with the template's contents (`src/`, `public/`, `index.html`, `package.json`, etc.).
2. `diseño-templete/` no longer exists.
3. `web/package.json` has the exact content from the brief (rebranded name, scripts, deps).
4. `web/index.html` has `<title>ViewRed</title>`.
5. `web/README.md` has the new ViewRed-specific content.
6. `git status` after the commit shows a clean working tree.
7. The commit message uses `feat(web):` prefix.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-05.md`

Include:
- What you implemented
- The move operation (which files moved, total count)
- `git status` output before and after the commit
- Files modified (package.json, index.html, README.md)
- Commit SHA + subject
- Any concerns (e.g., unexpected files in the template that needed handling)

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line summary (e.g. "moved ~25 files, rebranded package.json, --host now 127.0.0.1, working tree clean")
- Your concerns, if any
- The report file path

## When you're stuck

If the PowerShell `Move-Item` doesn't work as expected, or `git status` shows unexpected files, **stop and ask** with NEEDS_CONTEXT. Don't guess.