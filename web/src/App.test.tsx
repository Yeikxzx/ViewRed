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

afterEach(() => cleanup());

const renderAt = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>
  );

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
  it('redirects the root path to /dashboard', () => {
    renderAt('/');
    expectView('dashboard-view-wrapper');
  });

  it('renders DashboardView at /dashboard', () => {
    renderAt('/dashboard');
    expectView('dashboard-view-wrapper');
  });

  it('renders CapturaView at /captura', () => {
    renderAt('/captura');
    expectView('captura-view-wrapper');
  });

  it('renders FiltrosView at /filtros', () => {
    renderAt('/filtros');
    expectView('filtros-view-wrapper');
  });

  it('renders AlertasView at /alertas', () => {
    renderAt('/alertas');
    expectView('alerts-view-wrapper');
  });

  it('renders TopologiaView at /topologia', () => {
    renderAt('/topologia');
    expectView('topologia-view-wrapper');
  });

  it('renders VulnerabilidadesView at /vulnerabilidades', () => {
    renderAt('/vulnerabilidades');
    expectView('vulnerabilities-view-wrapper');
  });

  it('renders AsistenteView at /asistente', () => {
    renderAt('/asistente');
    expectView('ai-assistant-wrapper');
  });

  it('renders ConfiguracionView at /configuracion', () => {
    renderAt('/configuracion');
    expectView('settings-view-wrapper');
  });

  it('renders NotFoundView for unknown paths', () => {
    renderAt('/this-route-does-not-exist');
    expectView('not-found-view-wrapper');
  });
});
