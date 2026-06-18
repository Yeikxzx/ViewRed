# Task 6 Brief: Swap Gemini for placeholder LLM client (TDD)

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 6 of 12** in the ViewRed bootstrap. Tasks 1-5 are done (Go backend with config, web/ template rebranded and under standard location).

The current `AsistenteView.tsx` uses Google's Gemini API via `@google/genai` (inherited from the AI Studio template). Your job: **swap Gemini for a placeholder LLM client interface** that the real Ollama client will plug into in plan 6.

### Interfaces and decisions from earlier tasks

- **Module path**: `github.com/viewred/viewred` (do not change, but not relevant to this task).
- **Web structure**: `web/` is now the standard location with React 19 + Vite 6 + TypeScript.
- **`AsistenteView.tsx`**: currently uses `@google/genai`'s `GoogleGenAI` class and `ai.models.generateContent()` (or similar). You will replace this with calls to our `llm.chat()`.
- **TDD strictly enforced**: this task has 2 tests in `llm.test.ts` that must pass.
- **`@google/genai` removal**: after the swap, the dep is unused. You may remove it from `package.json` (preferred for cleanliness), or leave it if other code uses it. Grep first to confirm.

### Files already in the directory (DO NOT TOUCH except as specified)

- All Go files (`cmd/`, `internal/`) — KEEP
- All `web/` files except `src/lib/` (which you're creating) and `src/components/AsistenteView.tsx` (which you're modifying)
- All docs/ — KEEP
- `diseño-templete/` was deleted in Task 5

---

## Task: Swap Gemini for placeholder LLM client (TDD)

**Files:**
- Create: `web/src/lib/llm.ts`
- Create: `web/src/lib/llm.test.ts`
- Modify: `web/src/components/AsistenteView.tsx` (use LLMClient instead of Gemini)
- Possibly: `web/package.json` (remove `@google/genai` if no longer used)

### Cycle 1: LLMClient interface and mock

- [ ] **Step 1: Write failing test for LLMClient interface**

Create `C:\Users\Yeikxz\Desktop\ViewRed\web\src\lib\llm.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { MockLLMClient } from './llm';

describe('MockLLMClient', () => {
  it('returns a placeholder response', async () => {
    const client = new MockLLMClient();
    const response = await client.chat([
      { role: 'user', content: '¿Qué pasó en la red?' },
    ]);
    expect(response).toBeTruthy();
    expect(typeof response).toBe('string');
    expect(response.length).toBeGreaterThan(0);
  });

  it('reports as available', async () => {
    const client = new MockLLMClient();
    expect(await client.isAvailable()).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed\web
pnpm test
```

(If `pnpm` is not installed, use `npm test` instead.)

Expected: FAIL with `Cannot find module './llm'`.

- [ ] **Step 3: Implement LLMClient interface and MockLLMClient**

Create `C:\Users\Yeikxz\Desktop\ViewRed\web\src\lib\llm.ts`:

```typescript
// LLM client abstraction. The real OllamaClient is implemented in plan 6.
// For plan 1, we use MockLLMClient which returns a placeholder string.

export interface Message {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface LLMClient {
  chat(messages: Message[]): Promise<string>;
  stream?(messages: Message[], onToken: (token: string) => void): Promise<void>;
  isAvailable(): Promise<boolean>;
}

export class MockLLMClient implements LLMClient {
  async chat(messages: Message[]): Promise<string> {
    const last = messages[messages.length - 1];
    return `[Mock LLM] Recibí tu mensaje: "${last?.content ?? ''}". ` +
      `La integración real con Ollama (Qwen 2.5 3B) llega en el plan 6.`;
  }

  async isAvailable(): Promise<boolean> {
    return true;
  }
}

// Default export is the mock for now. Plan 6 replaces this with OllamaClient.
export const llm: LLMClient = new MockLLMClient();
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed\web
pnpm test
```

Expected: PASS for both tests.

### Cycle 2: Replace Gemini in AsistenteView

- [ ] **Step 5: Find all Gemini usage in the codebase**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed\web
grep -rn "google/genai\|GoogleGenAI\|gemini" src/
```

(On PowerShell, use `Select-String -Path src/*.tsx,src/*.ts -Pattern "google/genai|GoogleGenAI|gemini" -CaseSensitive:$false`.)

Expected: list of files using Gemini. Most likely only `AsistenteView.tsx`, but check all matches.

- [ ] **Step 6: Replace Gemini with llm client in AsistenteView**

Edit `C:\Users\Yeikxz\Desktop\ViewRed\web\src\components\AsistenteView.tsx`:

1. **Remove** the Gemini import:
   ```typescript
   import { GoogleGenAI } from "@google/genai";
   ```

2. **Add** the llm client import at the top of the file (adjust path if needed):
   ```typescript
   import { llm, type Message } from "../lib/llm";
   ```

3. **Replace** any Gemini usage. Common patterns you might find:
   - `const ai = new GoogleGenAI({ apiKey: ... })` — remove this line
   - `const response = await ai.models.generateContent({ ... })` — replace with `const response = await llm.chat(messages)`
   - `process.env.GEMINI_API_KEY` — remove (or replace with a comment: `// TODO plan 6: use OLLAMA_URL from runtime config`)

4. **Adapt the result handling**. The Gemini response might be `response.text` while `llm.chat()` returns a string directly. Adjust the code so the result flows naturally into the UI.

   For example, if you see:
   ```typescript
   const response = await ai.models.generateContent({ model: "gemini-...", contents: ... });
   const text = response.text;
   ```
   
   Replace with:
   ```typescript
   const messages: Message[] = [...];
   const text = await llm.chat(messages);
   ```

5. **Save the file** when done. Don't worry about exact UI output — the mock returns a placeholder string that will be displayed in the chat. The UI integration is what matters here.

If you find Gemini usage in OTHER files besides `AsistenteView.tsx`, fix those too. Grep again to confirm no Gemini references remain in `src/`.

- [ ] **Step 7: Verify build still works**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed\web
pnpm install
pnpm build
```

Expected: build completes (with maybe minor warnings about unused imports). If build fails with TypeScript errors, fix them — usually by removing now-unused Gemini types or imports.

- [ ] **Step 8: Optionally remove `@google/genai` from package.json**

After the swap, check if anything still uses `@google/genai`:

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed\web
grep -rn "google/genai\|@google/genai" src/ package.json
```

If nothing uses it, **remove** the dep from `web/package.json`:

```diff
   "dependencies": {
-    "@google/genai": "^2.4.0",
     "@hookform/resolvers": "^3.9.1",
```

Then run `pnpm install` to update `pnpm-lock.yaml` (or `package-lock.json` if using npm).

- [ ] **Step 9: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add web/src/ web/package.json web/pnpm-lock.yaml 2>/dev/null
git add web/package-lock.json 2>/dev/null
git status
git commit -m "feat(web): swap Gemini for placeholder LLM client interface"
```

(`pnpm-lock.yaml` and `package-lock.json` are mutually exclusive — only one exists depending on which package manager was used. The `2>/dev/null` is to suppress errors if the file doesn't exist.)

---

## Verification

Before reporting back, verify:

1. `web/src/lib/llm.ts` and `web/src/lib/llm.test.ts` exist with content from the brief.
2. `pnpm test` (or `npm test`) passes 2 tests.
3. `pnpm build` completes without errors.
4. `AsistenteView.tsx` no longer imports `@google/genai` and uses `llm.chat()`.
5. `grep` confirms no `@google/genai` references remain in `src/`.
6. (If you removed the dep) `package.json` no longer has `@google/genai` and `pnpm install` updated the lockfile.
7. Single commit on `main` with the expected message.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-06.md`

Include:
- What you implemented
- TDD evidence: RED (test fails with `Cannot find module`) and GREEN (2/2 tests pass)
- Files created/modified
- What you found when grepping for Gemini (which files had references, what patterns)
- The exact code changes you made in AsistenteView.tsx
- Whether you removed `@google/genai` from package.json (and why/why not)
- Commit SHA + subject
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line test summary
- Your concerns, if any
- The report file path

## When you're stuck

If the AsistenteView's Gemini usage is too complex to refactor, the build fails for reasons unrelated to your changes, or anything unusual, **stop and ask** with NEEDS_CONTEXT. Don't guess.