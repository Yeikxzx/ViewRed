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
