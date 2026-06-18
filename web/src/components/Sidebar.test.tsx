import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Sidebar from './Sidebar';

afterEach(() => cleanup());

interface RenderOpts {
  initialPath?: string;
  activeAlertCount?: number;
  collapsed?: boolean;
}

const renderSidebar = ({
  initialPath = '/dashboard',
  activeAlertCount = 0,
  collapsed = false,
}: RenderOpts = {}) =>
  render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Sidebar
        collapsed={collapsed}
        setCollapsed={() => {}}
        activeAlertCount={activeAlertCount}
      />
    </MemoryRouter>
  );

// Each menu entry has a stable id on the rendered anchor; these ids are part
// of the Sidebar's public surface and are used as behavioral hooks in tests.
const NAV_IDS = [
  'nav-dashboard',
  'nav-captura',
  'nav-filtros',
  'nav-alertas',
  'nav-topologia',
  'nav-vulnerabilidades',
  'nav-asistente',
  'nav-configuracion',
] as const;

const EXPECTED_HREFS: Record<(typeof NAV_IDS)[number], string> = {
  'nav-dashboard': '/dashboard',
  'nav-captura': '/captura',
  'nav-filtros': '/filtros',
  'nav-alertas': '/alertas',
  'nav-topologia': '/topologia',
  'nav-vulnerabilidades': '/vulnerabilidades',
  'nav-asistente': '/asistente',
  'nav-configuracion': '/configuracion',
};

describe('Sidebar navigation', () => {
  it('renders an anchor for every menu item with the correct href', () => {
    renderSidebar();
    for (const id of NAV_IDS) {
      const link = document.getElementById(id);
      expect(link, `expected element #${id} to be in the document`).not.toBeNull();
      expect(link!.tagName).toBe('A');
      expect(link!.getAttribute('href')).toBe(EXPECTED_HREFS[id]);
    }
  });

  it('marks the link matching the current route as active', () => {
    renderSidebar({ initialPath: '/topologia' });
    const active = document.getElementById('nav-topologia');
    expect(active).not.toBeNull();
    // NavLink sets aria-current="page" on the active anchor.
    expect(active!.getAttribute('aria-current')).toBe('page');
  });

  it('does not mark unrelated links as active', () => {
    renderSidebar({ initialPath: '/topologia' });
    const dashboard = document.getElementById('nav-dashboard');
    expect(dashboard).not.toBeNull();
    expect(dashboard!.getAttribute('aria-current')).toBeNull();
  });

  it('updates the active link when the route changes', () => {
    renderSidebar({ initialPath: '/dashboard' });
    const dashboard = document.getElementById('nav-dashboard');
    const captura = document.getElementById('nav-captura');
    expect(dashboard!.getAttribute('aria-current')).toBe('page');
    expect(captura!.getAttribute('aria-current')).toBeNull();

    fireEvent.click(captura!);

    expect(dashboard!.getAttribute('aria-current')).toBeNull();
    expect(captura!.getAttribute('aria-current')).toBe('page');
  });

  it('shows the alert badge when activeAlertCount is greater than zero', () => {
    renderSidebar({ activeAlertCount: 7 });
    const alertLink = document.getElementById('nav-alertas');
    expect(alertLink).not.toBeNull();
    expect(alertLink!.textContent).toContain('7');
  });

  it('does not show the alert badge when activeAlertCount is zero', () => {
    renderSidebar({ activeAlertCount: 0 });
    const alertLink = document.getElementById('nav-alertas');
    expect(alertLink).not.toBeNull();
    // Badge character is absent when count is zero.
    expect(alertLink!.textContent ?? '').not.toMatch(/\b[1-9]\d*\b/);
  });
});
