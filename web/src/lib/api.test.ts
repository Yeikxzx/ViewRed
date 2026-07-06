import { describe, it, expect, vi, afterEach } from 'vitest';
import { getRecentAlerts } from './api';

describe('getRecentAlerts severity mapping', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockFetch(alerts: { severity: string }[]) {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        alerts: alerts.map((a, idx) => ({
          id: idx + 1,
          time: '2026-05-18T10:00:00Z',
          severity: a.severity,
          source: '1.2.3.4',
          score: 80,
          title: 'Test Alert',
          description: 'Test description',
          context: {},
        })),
        limit: 4,
        offset: 0,
        total: alerts.length,
      }),
    } as unknown as Response);
  }

  it('preserves backend CRITICAL severity', async () => {
    mockFetch([{ severity: 'CRITICAL' }]);
    const result = await getRecentAlerts();
    expect(result[0].severity).toBe('CRITICAL');
  });

  it('preserves backend HIGH severity instead of downgrading to INFO', async () => {
    mockFetch([{ severity: 'HIGH' }]);
    const result = await getRecentAlerts();
    expect(result[0].severity).toBe('HIGH');
  });

  it('preserves backend WARNING severity', async () => {
    mockFetch([{ severity: 'WARNING' }]);
    const result = await getRecentAlerts();
    expect(result[0].severity).toBe('WARNING');
  });

  it('falls back unknown severities to INFO', async () => {
    mockFetch([{ severity: 'LOW' }]);
    const result = await getRecentAlerts();
    expect(result[0].severity).toBe('INFO');
  });

  it('is case-insensitive when normalizing backend severity', async () => {
    mockFetch([{ severity: 'high' }]);
    const result = await getRecentAlerts();
    expect(result[0].severity).toBe('HIGH');
  });
});
