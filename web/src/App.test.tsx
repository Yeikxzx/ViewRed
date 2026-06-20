import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';

// App.test.tsx is a routing test: it only asserts which view wrapper
// renders for a given path. It must not depend on the SSE stack, so
// mock the useSSE hook to a no-op. Behavior of useSSE is covered in
// src/lib/sse.test.ts and the per-view suites (e.g. DashboardView).
vi.mock('./lib/sse', () => ({
  useSSE: () => ({
    lastEvent: null,
    connectionStatus: 'disconnected',
    error: null,
    reconnect: () => {},
  }),
}));

vi.mock('./lib/api', () => ({
  getPackets: vi.fn(() => Promise.resolve({ packets: [], total: 0, offset: 0, limit: 100 })),
  getTopTalkers: vi.fn(() => Promise.resolve({ since: '', top_talkers: [] })),
  getProtocols: vi.fn(() => Promise.resolve({ since: '', protocols: [] })),
  getRecentAlerts: vi.fn().mockResolvedValue([
    {
      id: '1',
      timestamp: '2026-05-18T10:00:00Z',
      severity: 'CRITICAL',
      title: 'Alert 1',
      description: 'Desc 1',
      source: '1.2.3.4',
      destination: 'Desconocido',
      confidence: 90,
      origin: 'IA Detector',
      timeAgo: 'Hace un momento',
      acknowledged: false,
    },
  ]),
  getVulnerabilities: vi.fn().mockResolvedValue([]),
  acknowledgeAlert: vi.fn().mockResolvedValue(undefined),
  mitigateVulnerability: vi.fn().mockResolvedValue(undefined),
  getTopology: vi.fn(() => Promise.resolve({ nodes: [], edges: [] })),
}));

afterEach(() => cleanup());

const renderAt = async (path: string) => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>
  );
  // Wait for microtasks to complete to suppress "act" warnings from useEffects
  await new Promise(resolve => setTimeout(resolve, 0));
};

const expectView = (testId: string) => {
  // The view wrappers expose stable id attributes that act as behavioral
  // hooks (e.g. #dashboard-view-wrapper, #captura-view-wrapper).
  // We use getByTestId only for the NotFoundView case, which is added by
  // this migration and uses data-testid by convention.
  if (testId === 'not-found-view-wrapper') {
    expect(screen.getByTestId('not-found-view-wrapper')).toBeInTheDocument();
    return;
  }
  const el = document.querySelector(`#${testId}`);
  expect(el, `expected element #${testId} to be in the document`).not.toBeNull();
};

describe('App routing', () => {
  it('redirects the root path to /dashboard', async () => {
    await renderAt('/');
    expectView('dashboard-view-wrapper');
  });

  it('renders DashboardView at /dashboard', async () => {
    await renderAt('/dashboard');
    expectView('dashboard-view-wrapper');
  });

  it('renders CapturaView at /captura', async () => {
    await renderAt('/captura');
    expectView('captura-view-wrapper');
  });

  it('renders FiltrosView at /filtros', async () => {
    await renderAt('/filtros');
    expectView('filtros-view-wrapper');
  });

  it('renders AlertasView at /alertas', async () => {
    await renderAt('/alertas');
    expectView('alerts-view-wrapper');
  });

  it('renders TopologiaView at /topologia', async () => {
    await renderAt('/topologia');
    expectView('topologia-view-wrapper');
  });

  it('renders VulnerabilidadesView at /vulnerabilidades', async () => {
    await renderAt('/vulnerabilidades');
    expectView('vulnerabilities-view-wrapper');
  });

  it('renders AsistenteView at /asistente', async () => {
    await renderAt('/asistente');
    expectView('ai-assistant-wrapper');
  });

  it('renders ConfiguracionView at /configuracion', async () => {
    await renderAt('/configuracion');
    expectView('settings-view-wrapper');
  });

  it('renders NotFoundView for unknown paths', async () => {
    await renderAt('/this-route-does-not-exist');
    expectView('not-found-view-wrapper');
  });
});
