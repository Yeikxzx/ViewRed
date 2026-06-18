import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { render, cleanup, act } from '@testing-library/react';
import CapturaView from './CapturaView';
import { useSSEStore } from '../store/sse-store';

// The polling endpoint must match the production route registered in
// internal/api/router.go (`/api/v1/health`) and the URL the component
// is supposed to fetch.
const HEALTH_URL = '/api/v1/health';

// The polling interval (2s) is part of the plan's spec for Task 24.
// We test the actual interval value the component registers so a
// regression to "every 100ms" or "every 30s" gets caught.
const POLL_INTERVAL_MS = 2000;

const noopToggle = vi.fn();

const renderCaptura = () =>
  render(<CapturaView isCapturing={false} toggleCapture={noopToggle} />);

/**
 * Build a `Response`-like object the component's fetch wrapper can
 * consume. jsdom (Vitest's default environment) ships a global
 * `Response` constructor, but we want the response to be deterministic
 * per-test, so we patch `globalThis.fetch` directly and return the
 * canned object instead of going through the real network.
 */
function jsonResponse(body: unknown, init: { status?: number } = {}): Response {
  return new Response(JSON.stringify(body), {
    status: init.status ?? 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

const okHealth = (captureStatus: 'ok' | 'down' = 'ok') => ({
  status: captureStatus === 'ok' ? 'ok' : 'degraded',
  uptime_seconds: 123,
  subsystems: [
    { name: 'storage', status: 'ok' },
    { name: 'capture', status: captureStatus },
  ],
});

beforeEach(() => {
  // Reset store + mocks between tests so a test never inherits state
  // from the previous one.
  useSSEStore.setState({
    messages: new Map(),
    connectionStatus: 'disconnected',
    captureStatus: 'unknown',
    lastError: null,
  });
  noopToggle.mockReset();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

/**
 * Helper: stub the global `fetch` with a function that returns a
 * queue of canned responses. Each call to `fetch` consumes the next
 * response. If the queue is empty the stub rejects so we never hang
 * a test on an unexpected request.
 */
function stubFetchSequence(responses: Array<Response | Error>) {
  let i = 0;
  const mock = vi.fn(() => {
    const next = responses[i++];
    if (next === undefined) {
      return Promise.reject(new Error(`unexpected fetch call #${i}`));
    }
    if (next instanceof Error) {
      return Promise.reject(next);
    }
    return Promise.resolve(next);
  });
  vi.stubGlobal('fetch', mock);
  return mock;
}

/**
 * Wait until `predicate` returns truthy, advancing the fake clock in
 * 10ms ticks and flushing microtasks after each tick. Mirrors what
 * `waitFor` does under the hood, but works under
 * `vi.useFakeTimers()` where the real setTimeout polling that
 * `@testing-library/react`'s `waitFor` relies on is suspended.
 */
async function waitForState(
  predicate: () => boolean,
  options: { timeoutMs?: number; tickMs?: number } = {},
): Promise<void> {
  const timeoutMs = options.timeoutMs ?? 5000;
  const tickMs = options.tickMs ?? 10;
  const maxTicks = Math.ceil(timeoutMs / tickMs);
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(tickMs);
    });
  }
  throw new Error(`waitForState: predicate did not become true within ${timeoutMs}ms`);
}

/**
 * Wait for the next fetch call to land on the mock. Same idea as
 * `waitForState` but specialized for the call-count assertion.
 */
async function waitForFetch(mockFetch: ReturnType<typeof vi.fn>) {
  await waitForState(() => mockFetch.mock.calls.length > 0);
}

// ---------------------------------------------------------------------------
// Health polling
// ---------------------------------------------------------------------------

describe('CapturaView health polling', () => {
  it('fetches /api/v1/health on mount', async () => {
    vi.useFakeTimers();
    const mockFetch = stubFetchSequence([jsonResponse(okHealth('ok'))]);
    renderCaptura();

    await waitForFetch(mockFetch);
    expect(mockFetch).toHaveBeenCalledWith(HEALTH_URL);
  });

  it('re-polls the health endpoint on the configured interval', async () => {
    vi.useFakeTimers();
    const mockFetch = stubFetchSequence([
      jsonResponse(okHealth('ok')),
      jsonResponse(okHealth('ok')),
      jsonResponse(okHealth('ok')),
    ]);
    renderCaptura();

    await waitForFetch(mockFetch);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    // Advance the fake clock by 1 tick: another fetch should fire.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    });
    expect(mockFetch).toHaveBeenCalledTimes(2);

    // And again — confirms the interval is registered, not a one-shot.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    });
    expect(mockFetch).toHaveBeenCalledTimes(3);
  });

  it('stops polling when the component unmounts', async () => {
    vi.useFakeTimers();
    const mockFetch = stubFetchSequence([
      jsonResponse(okHealth('ok')),
      jsonResponse(okHealth('ok')),
      jsonResponse(okHealth('ok')),
    ]);
    const { unmount } = renderCaptura();

    await waitForFetch(mockFetch);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    unmount();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 5);
    });

    // No further fetches after unmount — interval was cleaned up.
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// captureStatus derivation
// ---------------------------------------------------------------------------

describe('CapturaView captureStatus from health', () => {
  it('sets captureStatus to "running" when the capture subsystem reports ok', async () => {
    vi.useFakeTimers();
    stubFetchSequence([jsonResponse(okHealth('ok'))]);
    renderCaptura();

    await waitForState(() => useSSEStore.getState().captureStatus === 'running');
    expect(useSSEStore.getState().captureStatus).toBe('running');
  });

  it('sets captureStatus to "paused" when the capture subsystem reports down', async () => {
    vi.useFakeTimers();
    stubFetchSequence([jsonResponse(okHealth('down'))]);
    renderCaptura();

    await waitForState(() => useSSEStore.getState().captureStatus === 'paused');
    expect(useSSEStore.getState().captureStatus).toBe('paused');
  });

  it('leaves captureStatus as "unknown" when the health payload has no capture subsystem', async () => {
    vi.useFakeTimers();
    const mockFetch = stubFetchSequence([
      jsonResponse({
        status: 'ok',
        uptime_seconds: 5,
        subsystems: [{ name: 'storage', status: 'ok' }],
      }),
    ]);
    renderCaptura();

    await waitForFetch(mockFetch);
    // Wait one more tick of microtasks so any post-fetch state update
    // has flushed before we assert.
    await act(async () => {
      await Promise.resolve();
    });
    expect(useSSEStore.getState().captureStatus).toBe('unknown');
  });

  it('sets captureStatus to "unknown" and records an error when fetch fails', async () => {
    vi.useFakeTimers();
    stubFetchSequence([new TypeError('NetworkError when attempting to fetch resource')]);
    renderCaptura();

    await waitForState(
      () =>
        useSSEStore.getState().captureStatus === 'unknown' &&
        /network/i.test(useSSEStore.getState().lastError ?? ''),
    );
    const state = useSSEStore.getState();
    expect(state.captureStatus).toBe('unknown');
    expect(state.lastError).toMatch(/network/i);
  });

  it('sets captureStatus to "unknown" when the response is not ok and has no capture subsystem', async () => {
    // 503 with empty subsystems (the case when storage is down and
    // the ErrorQuerier is the only registered subsystem). The
    // component must surface that as "unknown" rather than crash.
    vi.useFakeTimers();
    const mockFetch = stubFetchSequence([
      jsonResponse({ status: 'down', uptime_seconds: 1, subsystems: [] }, { status: 503 }),
    ]);
    renderCaptura();

    await waitForFetch(mockFetch);
    await act(async () => {
      await Promise.resolve();
    });
    expect(useSSEStore.getState().captureStatus).toBe('unknown');
  });
});

// ---------------------------------------------------------------------------
// Rendered status indicator
// ---------------------------------------------------------------------------

describe('CapturaView status indicator', () => {
  it('renders the running badge text "Captura Activa" when the store reports running', async () => {
    useSSEStore.setState({ captureStatus: 'running' });
    vi.useFakeTimers();
    stubFetchSequence([jsonResponse(okHealth('ok'))]);
    renderCaptura();

    await waitForState(() => {
      const badge = document.getElementById('captura-status-badge');
      return badge !== null && badge.textContent?.includes('Captura Activa') === true;
    });
    const badge = document.getElementById('captura-status-badge');
    expect(badge).not.toBeNull();
    expect(badge!.textContent).toContain('Captura Activa');
  });

  it('renders the paused badge text "Captura Pausada" when the store reports paused', async () => {
    useSSEStore.setState({ captureStatus: 'paused' });
    vi.useFakeTimers();
    stubFetchSequence([jsonResponse(okHealth('down'))]);
    renderCaptura();

    await waitForState(() => {
      const badge = document.getElementById('captura-status-badge');
      return badge !== null && badge.textContent?.includes('Captura Pausada') === true;
    });
    const badge = document.getElementById('captura-status-badge');
    expect(badge).not.toBeNull();
    expect(badge!.textContent).toContain('Captura Pausada');
  });

  it('renders an "Estado desconocido" placeholder when the store reports unknown', () => {
    useSSEStore.setState({ captureStatus: 'unknown' });
    // No fetch mock needed — the unknown state is the initial state
    // before any poll completes.
    renderCaptura();

    const badge = document.getElementById('captura-status-badge');
    expect(badge).not.toBeNull();
    expect(badge!.textContent).toContain('Estado desconocido');
  });
});

// ---------------------------------------------------------------------------
// Error banner
// ---------------------------------------------------------------------------

describe('CapturaView error banner', () => {
  it('shows the health-unreachable banner when the last poll failed', async () => {
    vi.useFakeTimers();
    stubFetchSequence([new TypeError('Failed to fetch')]);
    renderCaptura();

    await waitForState(() => {
      const banner = document.getElementById('captura-health-error-banner');
      return (
        banner !== null &&
        banner.textContent?.includes('No se pudo contactar el servicio de salud') === true
      );
    });
    const banner = document.getElementById('captura-health-error-banner');
    expect(banner).not.toBeNull();
    expect(banner!.textContent).toContain('No se pudo contactar el servicio de salud');
  });

  it('hides the error banner when the latest poll succeeded', async () => {
    vi.useFakeTimers();
    stubFetchSequence([jsonResponse(okHealth('ok'))]);
    renderCaptura();

    await waitForState(() => useSSEStore.getState().captureStatus === 'running');
    expect(document.getElementById('captura-health-error-banner')).toBeNull();
  });

  it('renders a "Reintentar" button inside the health error banner', async () => {
    vi.useFakeTimers();
    stubFetchSequence([new TypeError('Failed to fetch')]);
    renderCaptura();

    await waitForState(
      () => document.getElementById('captura-health-error-banner') !== null,
    );
    expect(
      document.getElementById('captura-health-retry-btn'),
      'expected captura-health-retry-btn',
    ).not.toBeNull();
  });

  it('triggers an immediate re-poll when the user clicks "Reintentar"', async () => {
    vi.useFakeTimers();
    // Two responses so the second one (after retry) is the one that
    // resolves the assertion.
    const mockFetch = stubFetchSequence([
      new TypeError('Failed to fetch'),
      new TypeError('Failed to fetch'),
      jsonResponse(okHealth('ok')),
    ]);
    renderCaptura();

    await waitForState(
      () => document.getElementById('captura-health-error-banner') !== null,
    );
    // First poll has happened (and failed).
    expect(mockFetch).toHaveBeenCalledTimes(1);

    const retryBtn = document.getElementById('captura-health-retry-btn') as HTMLButtonElement;
    expect(retryBtn).not.toBeNull();

    await act(async () => {
      retryBtn.click();
    });
    // Clicking the button should have re-triggered the effect and
    // fired a second fetch immediately, without waiting for the 2s
    // interval.
    expect(mockFetch.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});

// ---------------------------------------------------------------------------
// Empty state (spec §12 — "No hay captura configurada")
// ---------------------------------------------------------------------------

describe('CapturaView empty state', () => {
  it('shows the empty state when the health payload has no capture subsystem', async () => {
    vi.useFakeTimers();
    stubFetchSequence([
      jsonResponse({
        status: 'ok',
        uptime_seconds: 5,
        subsystems: [{ name: 'storage', status: 'ok' }],
      }),
    ]);
    renderCaptura();

    // The empty state lives behind `captura-empty-state` and replaces
    // the "Estado desconocido" badge.
    await waitForState(() => {
      const el = document.getElementById('captura-empty-state');
      return el !== null;
    });
    const empty = document.getElementById('captura-empty-state');
    expect(empty, 'expected captura-empty-state').not.toBeNull();
    expect(empty!.textContent).toMatch(/captura/i);
  });

  it('does not render the empty state while the first poll is still in flight', () => {
    // On mount, before any successful poll, the view must be in
    // "loading", not "empty".
    useSSEStore.setState({ captureStatus: 'unknown' });
    renderCaptura();

    expect(document.getElementById('captura-empty-state')).toBeNull();
  });
});
