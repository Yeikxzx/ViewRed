import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest';
import { render, cleanup, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';
import type { AlertBackend } from './lib/api';

const toastError = vi.fn();
const toastWarning = vi.fn();
const toastInfo = vi.fn();

vi.mock('./lib/notify', () => ({
  AppToaster: () => null,
  notify: {
    error: (...args: unknown[]) => toastError(...args),
    warning: (...args: unknown[]) => toastWarning(...args),
    info: (...args: unknown[]) => toastInfo(...args),
    success: vi.fn(),
    confirm: vi.fn(),
  },
}));

const mockUseSSE = vi.fn();
vi.mock('./lib/sse', () => ({
  useSSE: (...args: unknown[]) => mockUseSSE(...args),
}));

vi.mock('./lib/api', () => ({
  getPackets: vi.fn(() => Promise.resolve({ packets: [], total: 0, offset: 0, limit: 100 })),
  getTopTalkers: vi.fn(() => Promise.resolve({ since: '', top_talkers: [] })),
  getProtocols: vi.fn(() => Promise.resolve({ since: '', protocols: [] })),
  getRecentAlerts: vi.fn().mockResolvedValue([]),
  getVulnerabilities: vi.fn().mockResolvedValue([]),
  acknowledgeAlert: vi.fn().mockResolvedValue(undefined),
  mitigateVulnerability: vi.fn().mockResolvedValue(undefined),
  getTopology: vi.fn(() => Promise.resolve({ nodes: [], edges: [] })),
  getScanHistory: vi.fn(() => Promise.resolve([])),
  restartCapture: vi.fn().mockResolvedValue(undefined),
  stopCapture: vi.fn().mockResolvedValue(undefined),
  clearAlerts: vi.fn().mockResolvedValue(undefined),
  deleteAlert: vi.fn().mockResolvedValue(undefined),
}));

const renderApp = (path = '/alertas') =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>
  );

function backendAlert(overrides: Partial<AlertBackend> = {}): AlertBackend {
  return {
    id: 1,
    time: '2026-05-18T10:00:00Z',
    severity: 'HIGH',
    source: '10.0.0.5',
    score: 92,
    title: 'SSH Brute Force',
    description: 'Repeated login attempts',
    context: {},
    ...overrides,
  };
}

// Stable SSE return objects so React effect dependencies do not flip on
// every render and trigger an ingestion loop.
const noEvent = { lastEvent: null, connectionStatus: 'connected' as const, error: null, reconnect: vi.fn() };
const runningStatus = { lastEvent: { status: 'running' }, connectionStatus: 'connected' as const, error: null, reconnect: vi.fn() };

function withAlert(event: AlertBackend) {
  return { lastEvent: event, connectionStatus: 'connected' as const, error: null, reconnect: vi.fn() };
}

beforeEach(() => {
  mockUseSSE.mockReset();
  mockUseSSE.mockImplementation((url: string) => {
    if (url === '/api/v1/stream/capture-status') return runningStatus;
    return noEvent;
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Backend-to-frontend severity normalization via live SSE
// ---------------------------------------------------------------------------

describe('App SSE alert severity normalization', () => {
  it('preserves backend HIGH severity end-to-end instead of downgrading to INFO', async () => {
    const highEvent = backendAlert({ id: 99, severity: 'HIGH' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(highEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastWarning).toHaveBeenCalledTimes(1));

    // A downgrade to INFO would have triggered notify.info instead.
    expect(toastInfo).not.toHaveBeenCalled();
  });

  it('preserves backend CRITICAL severity from SSE', async () => {
    const criticalEvent = backendAlert({ id: 1, severity: 'CRITICAL', title: 'Malware Beacon' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(criticalEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1));
  });

  it('preserves backend WARNING severity from SSE', async () => {
    const warningEvent = backendAlert({ id: 2, severity: 'WARNING', title: 'Port Scan' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(warningEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastWarning).toHaveBeenCalledTimes(1));
  });

  it('falls back unknown backend severity to INFO through SSE', async () => {
    const lowEvent = backendAlert({ id: 3, severity: 'LOW', title: 'Noise Event' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(lowEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastInfo).toHaveBeenCalledTimes(1));
  });

  it('is case-insensitive when normalizing backend severity through SSE', async () => {
    const lowercaseHighEvent = backendAlert({ id: 8, severity: 'high', title: 'Case Test' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(lowercaseHighEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastWarning).toHaveBeenCalledTimes(1));

    expect(toastWarning).toHaveBeenCalledWith('Case Test', { description: '10.0.0.5' });
  });
});

// ---------------------------------------------------------------------------
// Toast behavior for live alerts
// ---------------------------------------------------------------------------

describe('App SSE alert toast handling', () => {
  it('raises notify.error for CRITICAL severity', async () => {
    const criticalEvent = backendAlert({ id: 4, severity: 'CRITICAL', title: 'Malware Beacon' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(criticalEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1));

    expect(toastError).toHaveBeenCalledWith('Malware Beacon', { description: '10.0.0.5' });
  });

  it('raises notify.warning for HIGH severity', async () => {
    const highEvent = backendAlert({ id: 5, severity: 'HIGH', title: 'SSH Brute Force' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(highEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastWarning).toHaveBeenCalledTimes(1));

    expect(toastWarning).toHaveBeenCalledWith('SSH Brute Force', { description: '10.0.0.5' });
  });

  it('raises notify.warning for WARNING severity', async () => {
    const warningEvent = backendAlert({ id: 6, severity: 'WARNING', title: 'Port Scan' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(warningEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastWarning).toHaveBeenCalledTimes(1));

    expect(toastWarning).toHaveBeenCalledWith('Port Scan', { description: '10.0.0.5' });
  });

  it('raises notify.info for INFO severity', async () => {
    const lowEvent = backendAlert({ id: 7, severity: 'LOW', title: 'Noise Event' });
    mockUseSSE.mockImplementation((url: string) => {
      if (url === '/api/v1/stream/alerts') return withAlert(lowEvent);
      if (url === '/api/v1/stream/capture-status') return runningStatus;
      return noEvent;
    });

    renderApp();
    await waitFor(() => expect(toastInfo).toHaveBeenCalledTimes(1));

    expect(toastInfo).toHaveBeenCalledWith('Noise Event', { description: '10.0.0.5' });
  });
});
