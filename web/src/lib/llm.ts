// LLM client abstraction. The real OllamaClient is implemented in plan 6.
// For plan 1, we use MockLLMClient which returns a placeholder string.

export interface Message {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface LLMClient {
  chat(messages: Message[]): Promise<string>;
  stream?(messages: Message[], onToken: (token: string) => void): Promise<void>;
  isAvailable(): Promise<boolean>;
}

export class MockLLMClient implements LLMClient {
  async chat(messages: Message[]): Promise<string> {
    const last = messages[messages.length - 1];
    return `[Mock LLM] Recibí tu mensaje: "${last?.content ?? ''}". ` +
      `La integración real con Ollama (Qwen 2.5 3B) llega en el plan 6.`;
  }

  async isAvailable(): Promise<boolean> {
    return true;
  }
}

// Default export is the mock for now. Plan 6 replaces this with OllamaClient.
export const llm: LLMClient = new MockLLMClient();
