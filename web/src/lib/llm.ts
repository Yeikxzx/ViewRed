// LLM client abstraction. The real OllamaClient connects to the Go backend.
// The backend acts as a proxy to local Ollama and handles DB persistence.

export interface Message {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface LLMClient {
  chat(messages: Message[]): Promise<string>;
  stream(messages: Message[], onToken: (token: string) => void): Promise<void>;
  isAvailable(): Promise<boolean>;
}

export class BackendLLMClient implements LLMClient {
  // We use a single conversation_id for MVP V1 to have a global chat
  private conversationId = '00000000-0000-0000-0000-000000000001';

  async chat(messages: Message[]): Promise<string> {
    throw new Error('chat() is deprecated, use stream()');
  }

  async stream(messages: Message[], onToken: (token: string) => void): Promise<void> {
    const res = await fetch('/api/v1/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        conversation_id: this.conversationId,
        messages: messages,
      }),
    });

    if (!res.ok) {
      throw new Error(`HTTP error! status: ${res.status}`);
    }

    if (!res.body) throw new Error('No body in response');
    
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      
      const chunk = decoder.decode(value, { stream: true });
      // SSE format parsing: data: {"message": {"content": "..."}}\n\n
      const lines = chunk.split('\n');
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const dataStr = line.slice(6);
            if (dataStr === '[DONE]') break;
            
            const data = JSON.parse(dataStr);
            if (data.message && data.message.content) {
              onToken(data.message.content);
            }
          } catch (e) {
            // Ignore parse errors on partial chunks
          }
        }
      }
    }
  }

  async isAvailable(): Promise<boolean> {
    // For now we assume the Go backend is always available to proxy.
    // If Ollama is down, the stream will throw an error and UI handles it.
    return true;
  }
}

export const llm: LLMClient = new BackendLLMClient();
