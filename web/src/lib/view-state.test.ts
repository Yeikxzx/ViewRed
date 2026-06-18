import { describe, it, expect, vi } from 'vitest';
import {
  deriveDashboardViewState,
  deriveCapturaViewState,
  type DashboardViewStateInput,
  type CapturaViewStateInput,
} from './view-state';

// ---------------------------------------------------------------------------
// deriveDashboardViewState
// ---------------------------------------------------------------------------

describe('deriveDashboardViewState', () => {
  const baseInput: DashboardViewStateInput = {
    metrics: undefined,
    connectionStatus: 'connecting',
    lastError: null,
    retry: () => {},
  };

  it('returns loading when no metrics have arrived and the connection is healthy', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'connecting',
    });
    expect(state.status).toBe('loading');
  });

  it('returns loading on first render even when connectionStatus is connected but no payload', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'connected',
      metrics: undefined,
    });
    expect(state.status).toBe('loading');
  });

  it('returns error when the SSE connection is disconnected with a recorded lastError', () => {
    const retry = vi.fn();
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'disconnected',
      lastError: 'connection lost',
      retry,
    });
    expect(state.status).toBe('error');
    if (state.status === 'error') {
      expect(state.message).toBe('connection lost');
      expect(state.retry).toBe(retry);
    }
  });

  it('error state wins over partial data (does not leak stale metrics)', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'disconnected',
      lastError: 'fail',
      metrics: {
        packets_per_sec: 999,
        bytes_per_sec: 1,
        active_connections: 1,
        device_count: 1,
      },
      retry: () => {},
    });
    // Spec: "Error states MUST NOT render partial or stale data underneath
    // the error message." So the ViewState should be `error`, not `data`.
    expect(state.status).toBe('error');
  });

  it('returns empty when all metrics are zero (data still arrived)', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'connected',
      lastError: null,
      metrics: {
        packets_per_sec: 0,
        bytes_per_sec: 0,
        active_connections: 0,
        device_count: 0,
      },
    });
    expect(state.status).toBe('empty');
    if (state.status === 'empty') {
      expect(state.message).toMatch(/tráfico/i);
    }
  });

  it('returns data when at least one metric is non-zero', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'connected',
      lastError: null,
      metrics: {
        packets_per_sec: 0,
        bytes_per_sec: 0,
        active_connections: 0,
        device_count: 5,
      },
    });
    expect(state.status).toBe('data');
    if (state.status === 'data') {
      expect(state.data.device_count).toBe(5);
    }
  });

  it('returns data when packets_per_sec is non-zero', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'connected',
      metrics: {
        packets_per_sec: 1.5,
        bytes_per_sec: 0,
        active_connections: 0,
        device_count: 0,
      },
    });
    expect(state.status).toBe('data');
  });

  it('returns data when bytes_per_sec is non-zero', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'connected',
      metrics: {
        packets_per_sec: 0,
        bytes_per_sec: 100,
        active_connections: 0,
        device_count: 0,
      },
    });
    expect(state.status).toBe('data');
  });

  it('returns data when active_connections is non-zero', () => {
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'connected',
      metrics: {
        packets_per_sec: 0,
        bytes_per_sec: 0,
        active_connections: 1,
        device_count: 0,
      },
    });
    expect(state.status).toBe('data');
  });

  it('disconnected without a lastError falls back to loading (not error)', () => {
    // connectionStatus can be 'disconnected' transiently (e.g. on mount
    // before the EventSource is opened) without a recorded lastError.
    // That should not surface the error UI yet.
    const state = deriveDashboardViewState({
      ...baseInput,
      connectionStatus: 'disconnected',
      lastError: null,
    });
    expect(state.status).toBe('loading');
  });
});

// ---------------------------------------------------------------------------
// deriveCapturaViewState
// ---------------------------------------------------------------------------

describe('deriveCapturaViewState', () => {
  const baseInput: CapturaViewStateInput = {
    captureStatus: 'unknown',
    hasPolled: false,
    healthError: null,
    retry: () => {},
  };

  it('returns loading on first mount before any successful poll', () => {
    const state = deriveCapturaViewState(baseInput);
    expect(state.status).toBe('loading');
  });

  it('returns error when the last health poll failed', () => {
    const retry = vi.fn();
    const state = deriveCapturaViewState({
      ...baseInput,
      hasPolled: true,
      healthError: 'HTTP 503',
      retry,
    });
    expect(state.status).toBe('error');
    if (state.status === 'error') {
      expect(state.message).toBe('HTTP 503');
      expect(state.retry).toBe(retry);
    }
  });

  it('error state wins over a known captureStatus (no stale data underneath)', () => {
    const state = deriveCapturaViewState({
      ...baseInput,
      hasPolled: true,
      captureStatus: 'running',
      healthError: 'fail',
    });
    expect(state.status).toBe('error');
  });

  it('returns empty after a successful poll with no capture subsystem', () => {
    const state = deriveCapturaViewState({
      ...baseInput,
      hasPolled: true,
      captureStatus: 'unknown',
      healthError: null,
    });
    expect(state.status).toBe('empty');
    if (state.status === 'empty') {
      expect(state.message).toMatch(/captura/i);
    }
  });

  it('returns data when capture subsystem is running', () => {
    const state = deriveCapturaViewState({
      ...baseInput,
      hasPolled: true,
      captureStatus: 'running',
    });
    expect(state.status).toBe('data');
    if (state.status === 'data') {
      expect(state.data.status).toBe('running');
    }
  });

  it('returns data when capture subsystem is paused', () => {
    const state = deriveCapturaViewState({
      ...baseInput,
      hasPolled: true,
      captureStatus: 'paused',
    });
    expect(state.status).toBe('data');
    if (state.status === 'data') {
      expect(state.data.status).toBe('paused');
    }
  });
});
