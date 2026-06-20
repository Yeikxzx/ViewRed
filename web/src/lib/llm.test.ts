import { describe, it, expect } from 'vitest';
import { BackendLLMClient } from './llm';

describe('BackendLLMClient', () => {
  it('throws an error if chat() is called directly', async () => {
    const client = new BackendLLMClient();
    await expect(client.chat([
      { role: 'user', content: '¿Qué pasó en la red?' },
    ])).rejects.toThrow('chat() is deprecated, use stream()');
  });

  it('reports as available', async () => {
    const client = new BackendLLMClient();
    expect(await client.isAvailable()).toBe(true);
  });
});
