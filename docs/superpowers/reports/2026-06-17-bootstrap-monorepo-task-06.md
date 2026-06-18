# Task 6 Report: Swap Gemini for placeholder LLM client (TDD)

**Date**: 2026-06-17
**Task**: 6 of 12 — ViewRed bootstrap

---

## What was implemented

1. **`web/src/lib/llm.ts`** — LLM client abstraction with:
   - `Message` interface (`role: 'user' | 'assistant' | 'system'`, `content: string`)
   - `LLMClient` interface (`chat()`, optional `stream()`, `isAvailable()`)
   - `MockLLMClient` class returning a placeholder Spanish string
   - Default `llm` export as a `MockLLMClient` instance
2. **`web/src/lib/llm.test.ts`** — 2 vitest tests for `MockLLMClient`
3. **`web/src/App.tsx`** — `handleSendMessage` rewritten to call `llm.chat()` instead of `setTimeout` + hardcoded strings
4. **`web/src/components/AsistenteView.tsx`** — added `llm` import and runtime availability check
5. **`web/package.json`** — removed `@google/genai` dependency (replaced with `reactflow` fix)
6. **`web/pnpm-lock.yaml`** — updated lockfile after dep changes

## TDD evidence

### RED
```
Error: Failed to load url ./llm (resolved id: ./llm) in .../llm.test.ts. Does the file exist?
Test Files  1 failed (1)
Tests       no tests
```

### GREEN
```
✓ src/lib/llm.test.ts (2 tests) 2ms
Test Files  1 passed (1)
Tests       2 passed (2)
```

## Files created

- `web/src/lib/llm.ts` (34 lines)
- `web/src/lib/llm.test.ts` (19 lines)

## Files modified

- `web/src/App.tsx` — replaced `setTimeout` + hardcoded replies with `llm.chat(messages)`
- `web/src/components/AsistenteView.tsx` — added `llm` import and `useEffect` for `isAvailable()`
- `web/package.json` — removed `@google/genai`, fixed `react-flow` → `reactflow` (pre-existing package name error)
- `web/pnpm-lock.yaml` — updated by pnpm install

## Grep for Gemini references

**Before the swap:**
- `node_modules/@google/genai/` — only in installed packages
- `web/package.json` — `"@google/genai": "^2.4.0"`
- `web/package-lock.json` — lockfile entries

**No Gemini usage in any source `.ts` or `.tsx` file.** The Gemini import was never committed to the source — only the dependency in `package.json` remained.

**After the swap:** Zero references to `@google/genai` or `GoogleGenAI` anywhere in `web/src/` or `package.json`.

## AsistenteView.tsx changes

### Before (lines 1-5 are unchanged imports except)
```
// No Gemini import existed — there was nothing to remove
```
### After (line 4 added)
```typescript
import { llm, type Message } from '../lib/llm';
```

### Before (component body)
```
export default function AsistenteView({...}) {
  const [inputValue, setInputValue] = useState('');
```

### After (added state + effect)
```typescript
export default function AsistenteView({...}) {
  const [inputValue, setInputValue] = useState('');
  const [llmAvailable, setLlmAvailable] = useState(false);

  useEffect(() => {
    llm.isAvailable().then(setLlmAvailable);
  }, []);
```

### Before (header status line)
```
Modelo Activo: Qwen 2.5 (3B Local) • CONECTADO
```
### After (dynamic status based on `llm.isAvailable()`)
```
Modelo Activo: Qwen 2.5 (3B Local) • {llmAvailable ? 'CONECTADO' : 'MOCK'}
```

## App.tsx changes

The entire `handleSendMessage` function was replaced (lines 69-103):

**Before**: Added user message, then `setTimeout(() => { ... hardcoded conditional responses ... }, 800)`.

**After**: Added user message, built `Message[]` from `updatedHistory`, called `const replyContent = await llm.chat(messages)`, then added bot message with the real mock response.

## `@google/genai` removal

- Grep confirmed zero source-code references to `@google/genai` or `GoogleGenAI`
- Removed the line from `web/package.json`
- Ran `pnpm install` which updated `pnpm-lock.yaml`
- Build verified clean

## Commit

```
b651a9e feat(web): swap Gemini for placeholder LLM client interface
```

6 files changed, 5458 insertions(+), 26 deletions(-)
- create mode 100644 web/pnpm-lock.yaml
- create mode 100644 web/src/lib/llm.test.ts
- create mode 100644 web/src/lib/llm.ts

## Concerns

1. **No actual Gemini in source code**: The brief assumed `AsistenteView.tsx` used Gemini, but the real codebase had zero Gemini source imports — only the dangling `@google/genai` dep in `package.json`. This means the "swap" was primarily creating the abstraction layer and wiring it in, not replacing existing code.
2. **Pre-existing `react-flow` package name error**: `package.json` had `"react-flow": "^11.11.4"` which doesn't exist in npm — the real package is `reactflow`. This blocked `pnpm install`. Fixed as part of this task (not imported in source, safe change).
3. **`pnpm-lock.yaml` was not tracked**: The project used npm before (has `package-lock.json` in .gitignore? Not shown in git status). pnpm install created a new lockfile. Brief confirmed this is acceptable — only one lockfile exists.
