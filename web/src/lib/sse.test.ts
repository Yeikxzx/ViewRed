import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSSEStore } from '../store/sse-store';
import { useSSE } from './sse';

// ---------------------------------------------------------------------------
// EventSource mock
// ---------------------------------------------------------------------------
// jsdom does not implement EventSource. We provide a controllable mock class
// that records every instance, lets the test dispatch synthetic events, and
// supports a stable `readyState` that mirrors the real one.

type Listener = (ev: unknown) => void;

const READY_CONNECTING = 0;
const READY_OPEN = 1;
const READY_CLOSED = 2;

class MockEventSource {
  static instances: MockEventSource[] = [];
  static nextUrl: string | null = null;
  static nextErrorFactory: ((self: MockEventSource) => Error | null) | null = null;

  url: string;
  withCredentials = false;
  readyState: number = READY_CONNECTING;
  onopen: Listener | null = null;
  onmessage: Listener | null = null;
  onerror: Listener | null = null;
  private listeners = new Map<string, Set<Listener>>();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);

    // Defer "open" to a microtask so listeners attached right after
    // construction still see the transition.
    queueMicrotask(() => {
      this.readyState = READY_OPEN;
      this.dispatch('open', {});
      if (this.onopen) this.onopen({} as MessageEvent);
    });
  }

  addEventListener(type: string, listener: Listener) {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set());
    this.listeners.get(type)!.add(listener);
  }

  removeEventListener(type: string, listener: Listener) {
    this.listeners.get(type)?.delete(listener);
  }

  close() {
    this.readyState = READY_CLOSED;
  }

  /** Test helper: dispatch a synthetic event of the given type. */
  dispatch(type: string, data: unknown) {
    const target = this.listeners.get(type);
    if (target) {
      for (const fn of target) fn(data);
    }
    if (type === 'message' && this.onmessage) this.onmessage(data);
    if (type === 'error' && this.onerror) this.onerror(data);
  }

  /** Test helper: trigger the native error path. */
  triggerError(err: Error | null = null) {
    this.dispatch('error', err ?? new Error('connection lost'));
  }
}

beforeEach(() => {
  MockEventSource.instances = [];
  MockEventSource.nextUrl = null;
  MockEventSource.nextErrorFactory = null;
  (globalThis as unknown as { EventSource: typeof EventSource }).EventSource =
    MockEventSource as unknown as typeof EventSource;

  // Reset the zustand store to a known baseline.
  useSSEStore.setState({
    messages: new Map(),
    connectionStatus: 'disconnected',
    captureStatus: 'unknown',
    lastError: null,
  });
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

/**
 * Return the most recent MockEventSource instance.
 *
 * `renderHook` runs effects synchronously (wrapped in `act`), so the
 * constructor — and therefore `MockEventSource.instances.push(this)` — has
 * already executed by the time the caller reaches this helper. The same
 * holds after `vi.advanceTimersByTimeAsync` fires a reconnect callback:
 * `openSource()` runs synchronously and pushes the new instance immediately.
 *
 * We deliberately avoid `@testing-library/react`'s `waitFor` here because
 * `waitFor` polls via real `setTimeout`, and any test that has switched to
 * `vi.useFakeTimers()` would deadlock waiting for those polls to fire.
 */
async function waitForSource(): Promise<MockEventSource> {
  const last = MockEventSource.instances[MockEventSource.instances.length - 1];
  if (!last) {
    throw new Error('waitForSource: no EventSource instance was created');
  }
  return last;
}

// ---------------------------------------------------------------------------
// useSSE
// ---------------------------------------------------------------------------

describe('useSSE', () => {
  it('opens an EventSource for the given URL on mount', async () => {
    const { result } = renderHook(() => useSSE('/api/v1/stream/metrics'));

    // While the connection is being established, status should be 'connecting'.
    expect(result.current.connectionStatus).toBe('connecting');
    expect(result.current.lastEvent).toBeNull();
    expect(result.current.error).toBeNull();

    const source = await waitForSource();
    expect(source.url).toBe('/api/v1/stream/metrics');
  });

  it('writes incoming messages to the zustand store under the URL key', async () => {
    const { result } = renderHook(() => useSSE('/api/v1/stream/metrics'));

    const source = await waitForSource();
    await act(async () => {
      source.dispatch('message', { data: JSON.stringify({ packets_per_sec: 42 }) });
    });

    expect(result.current.lastEvent).toEqual({ packets_per_sec: 42 });
    expect(useSSEStore.getState().messages.get('/api/v1/stream/metrics')).toEqual({
      packets_per_sec: 42,
    });
  });

  it('falls back to the raw string when the payload is not valid JSON', async () => {
    const { result } = renderHook(() => useSSE('/api/v1/stream/alerts'));

    const source = await waitForSource();
    await act(async () => {
      source.dispatch('message', { data: 'not-json' });
    });

    expect(result.current.lastEvent).toBe('not-json');
    expect(useSSEStore.getState().messages.get('/api/v1/stream/alerts')).toBe(
      'not-json',
    );
  });

  it('marks the connection as connected after the open event', async () => {
    const { result } = renderHook(() => useSSE('/api/v1/stream/metrics'));

    expect(result.current.connectionStatus).toBe('connecting');

    const source = await waitForSource();
    await act(async () => {
      // open is dispatched by the constructor in a microtask; allow it to run.
      await Promise.resolve();
    });

    expect(source.readyState).toBe(READY_OPEN);
    expect(result.current.connectionStatus).toBe('connected');
  });

  it('captures errors and stores the message in the hook + zustand store', async () => {
    const { result } = renderHook(() => useSSE('/api/v1/stream/metrics'));

    const source = await waitForSource();
    await act(async () => {
      source.triggerError(new Error('boom'));
    });

    expect(result.current.error).toBe('boom');
    expect(useSSEStore.getState().lastError).toBe('boom');
  });

  it('reconnects with exponential backoff after a connection error', async () => {
    vi.useFakeTimers();

    const { result } = renderHook(() => useSSE('/api/v1/stream/metrics'));

    // First source opens.
    const first = await waitForSource();

    // Trigger an error to start the reconnect sequence.
    await act(async () => {
      first.triggerError(new Error('dropped'));
    });

    // After the error: status -> reconnecting, error captured.
    expect(result.current.connectionStatus).toBe('reconnecting');

    // No new source yet — the first backoff is 1s.
    expect(MockEventSource.instances.length).toBe(1);

    // Advance 1000ms: the first retry fires, opening source #2.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
    expect(MockEventSource.instances.length).toBe(2);

    // Drop the second connection — backoff should double to 2s.
    const second = MockEventSource.instances[1]!;
    await act(async () => {
      second.triggerError(new Error('dropped again'));
    });
    expect(MockEventSource.instances.length).toBe(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(MockEventSource.instances.length).toBe(3);

    // Drop the third — backoff should double to 4s.
    const third = MockEventSource.instances[2]!;
    await act(async () => {
      third.triggerError(new Error('dropped 3'));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000);
    });
    expect(MockEventSource.instances.length).toBe(4);

    // Drop the fourth — backoff should double to 8s.
    const fourth = MockEventSource.instances[3]!;
    await act(async () => {
      fourth.triggerError(new Error('dropped 4'));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(8000);
    });
    expect(MockEventSource.instances.length).toBe(5);
  });

  it('caps the reconnect backoff at 30 seconds', async () => {
    vi.useFakeTimers();

    renderHook(() => useSSE('/api/v1/stream/metrics'));

    // Drive the first 4 attempts to reach the 8s slot, then keep dropping.
    const sequence = [1000, 2000, 4000, 8000];
    for (let i = 0; i < sequence.length; i++) {
      const source = await waitForSource();
      await act(async () => {
        source.triggerError(new Error('drop'));
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(sequence[i]!);
      });
    }
    // After 5 attempts (1s, 2s, 4s, 8s, 16s would be next) the cap kicks in.
    // Drop the 5th, advance 16s and expect a 6th source.
    const fifth = await waitForSource();
    await act(async () => {
      fifth.triggerError(new Error('drop'));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(16000);
    });
    const sixth = MockEventSource.instances[5]!;
    await act(async () => {
      sixth.triggerError(new Error('drop'));
    });

    // Next backoff would be 32s, but we cap to 30s.
    const beforeSeventh = MockEventSource.instances.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(31000);
    });
    const afterSeventh = MockEventSource.instances.length;
    expect(afterSeventh).toBe(beforeSeventh + 1);
  });

  it('exposes a reconnect() function that closes the current source and retries', async () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useSSE('/api/v1/stream/metrics'));

    const first = await waitForSource();
    expect(MockEventSource.instances.length).toBe(1);
    expect(first.readyState).not.toBe(READY_CLOSED);

    await act(async () => {
      result.current.reconnect();
    });

    expect(first.readyState).toBe(READY_CLOSED);

    // reconnect() resets the backoff counter and opens a new source
    // synchronously.
    expect(MockEventSource.instances.length).toBe(2);
  });

  it('cleans up the EventSource on unmount and does not write to the store afterwards', async () => {
    const { result, unmount } = renderHook(() =>
      useSSE('/api/v1/stream/metrics'),
    );

    const source = await waitForSource();
    expect(source.readyState).not.toBe(READY_CLOSED);

    unmount();

    expect(source.readyState).toBe(READY_CLOSED);
    // After unmount the hook's value reference is detached, but the store
    // is global. Verify the source was closed and that no further events
    // propagate by dispatching one and ensuring nothing changes.
    const before = useSSEStore.getState().messages.get('/api/v1/stream/metrics');
    await act(async () => {
      source.dispatch('message', { data: JSON.stringify({ x: 1 }) });
    });
    const after = useSSEStore.getState().messages.get('/api/v1/stream/metrics');
    expect(after).toBe(before);
  });

  it('updates the store connectionStatus alongside the hook value', async () => {
    const { result } = renderHook(() => useSSE('/api/v1/stream/metrics'));

    expect(useSSEStore.getState().connectionStatus).toBe('connecting');
    expect(result.current.connectionStatus).toBe('connecting');

    const source = await waitForSource();
    await act(async () => {
      await Promise.resolve();
    });

    expect(useSSEStore.getState().connectionStatus).toBe('connected');
    expect(result.current.connectionStatus).toBe('connected');

    await act(async () => {
      source.triggerError(new Error('drop'));
    });
    expect(useSSEStore.getState().connectionStatus).toBe('reconnecting');
  });
});
