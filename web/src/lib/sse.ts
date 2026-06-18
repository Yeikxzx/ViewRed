import { useCallback, useEffect, useRef, useState } from 'react';
import { useSSEStore, type ConnectionStatus } from '../store/sse-store';

/**
 * Result of {@link useSSE}.
 *
 * `lastEvent` is the most recent payload the server pushed for this URL.
 * `connectionStatus` mirrors the shared store value and reflects the current
 * state of the underlying EventSource.
 *
 * `error` is `null` while the stream is healthy; otherwise it carries the
 * last connection error message observed.
 *
 * `reconnect` lets the caller force a fresh attempt: it closes the current
 * source and schedules an immediate reconnection (backoff counter resets).
 */
export interface UseSSEResult<T> {
  lastEvent: T | null;
  connectionStatus: ConnectionStatus;
  error: string | null;
  reconnect: () => void;
}

/** Backoff schedule in ms: 1s, 2s, 4s, 8s, 16s, then capped at 30s. */
const BACKOFF_STEPS_MS = [1000, 2000, 4000, 8000, 16000, 30000];
const BACKOFF_CAP_MS = 30000;
const INITIAL_BACKOFF_MS = 1000;

function backoffFor(attempt: number): number {
  if (attempt < 0) return INITIAL_BACKOFF_MS;
  if (attempt < BACKOFF_STEPS_MS.length) {
    return BACKOFF_STEPS_MS[attempt]!;
  }
  return BACKOFF_CAP_MS;
}

function parsePayload(raw: unknown): unknown {
  if (raw == null) return null;
  if (typeof raw !== 'string') return raw;
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'string') return err;
  return 'connection error';
}

/**
 * useSSE opens a native `EventSource` for `url` and exposes a React-friendly
 * surface backed by the global Zustand store.
 *
 * - Writes every message to the store under the URL key.
 * - Updates the store's `connectionStatus` and `lastError`.
 * - Reconnects automatically with exponential backoff
 *   (1s, 2s, 4s, 8s, 16s, 30s, 30s, ...).
 * - Cleans up the source and any pending timer on unmount.
 * - `reconnect()` closes the current source and retries immediately.
 */
export function useSSE<T = unknown>(url: string): UseSSEResult<T> {
  const [lastEvent, setLastEvent] = useState<T | null>(null);
  const [connectionStatus, setLocalStatus] = useState<ConnectionStatus>('connecting');
  const [error, setError] = useState<string | null>(null);

  const sourceRef = useRef<EventSource | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const attemptRef = useRef(0);
  const closedRef = useRef(false);

  const syncStatus = useCallback((status: ConnectionStatus) => {
    setLocalStatus(status);
    useSSEStore.getState().setConnectionStatus(status);
  }, []);

  const syncError = useCallback((msg: string | null) => {
    setError(msg);
    useSSEStore.getState().setLastError(msg);
  }, []);

  const clearTimer = useCallback(() => {
    if (timerRef.current != null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const openSource = useCallback(() => {
    if (closedRef.current) return;
    syncStatus('connecting');

    const source = new EventSource(url);
    sourceRef.current = source;

    source.addEventListener('open', () => {
      if (closedRef.current) return;
      attemptRef.current = 0;
      syncError(null);
      syncStatus('connected');
    });

    source.addEventListener('message', (event) => {
      if (closedRef.current) return;
      const payload = parsePayload((event as MessageEvent).data);
      setLastEvent(payload as T);
      useSSEStore.getState().addMessage(url, payload);
    });

    source.addEventListener('error', (event) => {
      if (closedRef.current) return;
      syncError(errorMessage(event));
      syncStatus('reconnecting');

      if (sourceRef.current === source) {
        source.close();
        sourceRef.current = null;
      }

      const delay = backoffFor(attemptRef.current);
      attemptRef.current += 1;
      clearTimer();
      timerRef.current = setTimeout(() => {
        timerRef.current = null;
        openSource();
      }, delay);
    });
  }, [url, syncStatus, syncError, clearTimer]);

  // Mount: open the source. Unmount: close + clear timer.
  useEffect(() => {
    closedRef.current = false;
    openSource();
    return () => {
      closedRef.current = true;
      clearTimer();
      if (sourceRef.current) {
        sourceRef.current.close();
        sourceRef.current = null;
      }
      // Leave the store's status alone — other components may still be connected.
    };
  }, [openSource, clearTimer]);

  const reconnect = useCallback(() => {
    if (closedRef.current) return;
    clearTimer();
    if (sourceRef.current) {
      sourceRef.current.close();
      sourceRef.current = null;
    }
    attemptRef.current = 0;
    openSource();
  }, [openSource, clearTimer]);

  return { lastEvent, connectionStatus, error, reconnect };
}
