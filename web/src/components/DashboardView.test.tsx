import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest';
import { render, cleanup } from '@testing-library/react';
import { useSSEStore } from '../store/sse-store';
import type { NetworkAlert } from '../types';
import DashboardView from './DashboardView';

// The metrics stream URL the dashboard subscribes to. Must match the
// production wiring in DashboardView and the broker topic registered
// in internal/api/router.go.
const METRICS_URL = '/api/v1/stream/metrics';

// Mock the useSSE hook so the component test does not need a real
// EventSource. The store still receives addMessage() calls in production;
// in tests we drive the store directly through setState() to control
// the data the component reads.
const mockUseSSE = vi.fn();
vi.mock('../lib/sse', () => ({
  useSSE: (...args: unknown[]) => mockUseSSE(...args),
}));

const noopSetView = vi.fn();
const noopSetSelectedAlert = vi.fn();
const noopReconnect = vi.fn();

const renderDashboard = (alerts: NetworkAlert[] = []) =>
  render(
    <DashboardView
      setView={noopSetView}
      alerts={alerts}
      setSelectedAlert={noopSetSelectedAlert}
    />,
  );

beforeEach(() => {
  mockUseSSE.mockReset();
  mockUseSSE.mockReturnValue({
    lastEvent: null,
    connectionStatus: 'connected',
    error: null,
    reconnect: noopReconnect,
  });
  useSSEStore.setState({
    messages: new Map(),
    connectionStatus: 'connected',
    captureStatus: 'unknown',
    lastError: null,
  });
});

afterEach(() => {
  cleanup();
});

// ---------------------------------------------------------------------------
// SSE wiring
// ---------------------------------------------------------------------------

describe('DashboardView SSE wiring', () => {
  it('subscribes to /api/v1/stream/metrics via useSSE', () => {
    renderDashboard();
    expect(mockUseSSE).toHaveBeenCalledWith(METRICS_URL);
  });

  it('renders KPI values from the metrics event in the store', () => {
    const payload = {
      packets_per_sec: 1234.5,
      bytes_per_sec: 4_200_000,
      active_connections: 87,
      device_count: 42,
    };
    useSSEStore.setState({
      messages: new Map([[METRICS_URL, payload]]),
    });
    renderDashboard();

    // The four KPI cards keep their stable ids from the original mock
    // implementation; the value text is derived from the live payload.
    const kpi0 = document.getElementById('kpi-card-0');
    expect(kpi0, 'expected kpi-card-0 to be in the document').not.toBeNull();
    // packets_per_sec=1234.5 → "1,234.5"
    expect(kpi0!.textContent).toContain('1,234.5');

    const kpi1 = document.getElementById('kpi-card-1');
    expect(kpi1, 'expected kpi-card-1 to be in the document').not.toBeNull();
    // bytes_per_sec=4_200_000 → "4.2 MB"
    expect(kpi1!.textContent).toContain('4.2');

    const kpi2 = document.getElementById('kpi-card-2');
    expect(kpi2, 'expected kpi-card-2 to be in the document').not.toBeNull();
    // active_connections=87 → "87"
    expect(kpi2!.textContent).toContain('87');

    const kpi3 = document.getElementById('kpi-card-3');
    expect(kpi3, 'expected kpi-card-3 to be in the document').not.toBeNull();
    // device_count=42 → "42"
    expect(kpi3!.textContent).toContain('42');
  });
});

// ---------------------------------------------------------------------------
// Loading state
// ---------------------------------------------------------------------------

describe('DashboardView loading state', () => {
  it('shows a skeleton for each KPI when SSE is still connecting', () => {
    mockUseSSE.mockReturnValue({
      lastEvent: null,
      connectionStatus: 'connecting',
      error: null,
      reconnect: noopReconnect,
    });
    useSSEStore.setState({ connectionStatus: 'connecting' });
    renderDashboard();

    // The first KPI card slot renders a skeleton placeholder (no value
    // text) while the stream is connecting.
    const skeleton = document.getElementById('kpi-card-0-skeleton');
    expect(skeleton, 'expected kpi-card-0-skeleton placeholder').not.toBeNull();
    // Real KPI values must not leak through while loading.
    const kpi0 = document.getElementById('kpi-card-0');
    expect(kpi0).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Error state
// ---------------------------------------------------------------------------

describe('DashboardView error state', () => {
  it('shows the "Conexión SSE perdida" banner when disconnected with a lastError', () => {
    mockUseSSE.mockReturnValue({
      lastEvent: null,
      connectionStatus: 'disconnected',
      error: 'connection lost',
      reconnect: noopReconnect,
    });
    useSSEStore.setState({
      connectionStatus: 'disconnected',
      lastError: 'connection lost',
    });
    renderDashboard();

    const banner = document.getElementById('sse-error-banner');
    expect(banner, 'expected sse-error-banner to be in the document').not.toBeNull();
    expect(banner!.textContent).toContain('Conexión SSE perdida');
  });

  it('does not show the error banner when the stream is healthy', () => {
    useSSEStore.setState({
      messages: new Map([
        [
          METRICS_URL,
          {
            packets_per_sec: 1,
            bytes_per_sec: 1,
            active_connections: 1,
            device_count: 1,
          },
        ],
      ]),
    });
    renderDashboard();

    expect(document.getElementById('sse-error-banner')).toBeNull();
  });
});
