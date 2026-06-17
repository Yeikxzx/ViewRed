import { describe, it, expect } from 'vitest';
import { MockLLMClient } from './llm';

describe('MockLLMClient', () => {
  it('returns a placeholder response', async () => {
    const client = new MockLLMClient();
    const response = await client.chat([
      { role: 'user', content: '¿Qué pasó en la red?' },
    ]);
    expect(response).toBeTruthy();
    expect(typeof response).toBe('string');
    expect(response.length).toBeGreaterThan(0);
  });

  it('reports as available', async () => {
    const client = new MockLLMClient();
    expect(await client.isAvailable()).toBe(true);
  });
});
