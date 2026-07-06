import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { NetworkAlert } from '../types';
import AlertasView from './AlertasView';

const baseAlert: NetworkAlert = {
  id: '1',
  timestamp: '2026-05-18T10:00:00Z',
  severity: 'HIGH',
  title: 'Telnet Brute Force',
  description: 'Repeated Telnet attempts',
  source: '10.0.0.5',
  destination: '10.0.0.2',
  confidence: 92,
  origin: 'Deteccion Local',
  timeAgo: 'Hace un momento',
  acknowledged: false,
  port: 23,
};

function renderAlerts(alerts: NetworkAlert[] = [baseAlert]) {
  return render(
    <AlertasView
      alerts={alerts}
      onAcknowledgeAlert={vi.fn()}
      onDeleteAlert={vi.fn()}
      onClearAlerts={vi.fn()}
      setSelectedAlertToExplain={vi.fn()}
      setView={vi.fn()}
    />,
  );
}

afterEach(() => {
  cleanup();
});

describe('AlertasView severity filters', () => {
  it('shows HIGH alerts in the Advertencia filter', () => {
    renderAlerts();

    fireEvent.click(screen.getByRole('button', { name: /Advertencia/i }));

    expect(screen.getByText('Telnet Brute Force')).toBeInTheDocument();
  });
});
