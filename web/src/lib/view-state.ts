/**
 * ViewState<T> — shared loading/error/empty/data pattern from spec §12.
 *
 * Every view that consumes API data SHOULD derive a `ViewState<T>` from
 * its raw inputs (store values, hook return values, etc.) and render
 * the corresponding branch. The four branches are mutually exclusive:
 *
 * - `loading` — we have not received any data yet.
 * - `error`   — the data source failed. The view must surface a message
 *                and a `retry` callback; per spec, no stale data may
 *                leak underneath the error message.
 * - `empty`   — the source is healthy, but the payload is empty
 *                (e.g., all KPI counters are zero, or a health poll
 *                reported no capture subsystem).
 * - `data`    — the view has something useful to render.
 */
export type ViewState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string; retry: () => void }
  | { status: 'empty'; message: string; action?: { label: string; onClick: () => void } }
  | { status: 'data'; data: T };

// ---------------------------------------------------------------------------
// DashboardView state derivation
// ---------------------------------------------------------------------------

/**
 * Shape of the metrics payload pushed by the broker's `/stream/metrics`
 * handler. Kept in sync with the Go handler in
 * `internal/api/handlers/sse.go`.
 */
export interface DashboardMetrics {
  packets_per_sec: number;
  bytes_per_sec: number;
  active_connections: number;
  device_count: number;
}

export interface DashboardViewStateInput {
  metrics: DashboardMetrics | undefined;
  connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'reconnecting';
  lastError: string | null;
  retry: () => void;
}

/**
 * Derive the dashboard's ViewState from the SSE inputs.
 *
 * Priority (highest first):
 *  1. error   — the SSE stream is down AND a non-null `lastError` is
 *               recorded. Without a recorded error the disconnected
 *               state is transient (e.g. on mount) and falls through.
 *  2. loading — no metrics payload has arrived yet.
 *  3. empty   — the payload arrived but every counter is zero. This
 *               matches the "No hay tráfico" scenario in spec §12.
 *  4. data    — at least one counter is non-zero.
 *
 * Spec §12: "Error states MUST NOT render partial or stale data
 * underneath the error message." → error wins over data.
 */
export function deriveDashboardViewState(
  input: DashboardViewStateInput,
): ViewState<DashboardMetrics> {
  if (input.connectionStatus === 'disconnected' && input.lastError !== null) {
    return { status: 'error', message: input.lastError, retry: input.retry };
  }
  if (input.metrics === undefined) {
    return { status: 'loading' };
  }
  if (isAllZeroMetrics(input.metrics)) {
    return {
      status: 'empty',
      message: 'No hay tráfico en este momento. Esperando paquetes…',
    };
  }
  return { status: 'data', data: input.metrics };
}

function isAllZeroMetrics(m: DashboardMetrics): boolean {
  return (
    m.packets_per_sec === 0 &&
    m.bytes_per_sec === 0 &&
    m.active_connections === 0 &&
    m.device_count === 0
  );
}

// ---------------------------------------------------------------------------
// CapturaView state derivation
// ---------------------------------------------------------------------------

/**
 * Coarse capture status derived from the health endpoint's `capture`
 * subsystem. See `CapturaView.deriveCaptureStatus` for the upstream
 * mapping. The view needs a separate "have we ever polled?" flag
 * because `unknown` is ambiguous: it could mean "we haven't polled yet"
 * (loading) or "we polled and there is no capture subsystem" (empty).
 */
export type CaptureStatus = 'running' | 'paused' | 'unknown';

export interface CapturaViewStateInput {
  captureStatus: CaptureStatus;
  hasPolled: boolean;
  healthError: string | null;
  retry: () => void;
}

/**
 * Derive the captura view's ViewState from the health-polling inputs.
 *
 * Priority (highest first):
 *  1. error   — the last health poll failed (`healthError != null`).
 *               Wins over any cached captureStatus; we never want to
 *               display "Captura Activa" while the backend is down.
 *  2. loading — the view mounted but has not yet completed a
 *               successful poll.
 *  3. empty   — we polled successfully, the backend is healthy, but
 *               there is no `capture` subsystem in the response (e.g.
 *               the binary was built without capture wired up).
 *  4. data    — the capture subsystem is either `running` or
 *               `paused`; render the live status.
 */
export function deriveCapturaViewState(
  input: CapturaViewStateInput,
): ViewState<{ status: CaptureStatus }> {
  if (input.healthError !== null) {
    return { status: 'error', message: input.healthError, retry: input.retry };
  }
  if (!input.hasPolled) {
    return { status: 'loading' };
  }
  if (input.captureStatus === 'unknown') {
    return {
      status: 'empty',
      message: 'No hay captura configurada. Inicia el worker para ver datos.',
    };
  }
  return { status: 'data', data: { status: input.captureStatus } };
}
