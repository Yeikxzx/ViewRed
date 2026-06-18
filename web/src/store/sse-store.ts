import { create } from 'zustand';

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

export type CaptureStatus = 'running' | 'paused' | 'unknown';

export interface SSEStoreState {
  messages: Map<string, unknown>;
  connectionStatus: ConnectionStatus;
  captureStatus: CaptureStatus;
  lastError: string | null;
  addMessage: (topic: string, data: unknown) => void;
  setConnectionStatus: (status: ConnectionStatus) => void;
  setCaptureStatus: (status: CaptureStatus) => void;
  setLastError: (err: string | null) => void;
  clearMessages: () => void;
}

export const useSSEStore = create<SSEStoreState>((set) => ({
  messages: new Map(),
  connectionStatus: 'disconnected',
  captureStatus: 'unknown',
  lastError: null,
  addMessage: (topic, data) =>
    set((s) => {
      const next = new Map(s.messages);
      next.set(topic, data);
      return { messages: next };
    }),
  setConnectionStatus: (status) => set({ connectionStatus: status }),
  setCaptureStatus: (status) => set({ captureStatus: status }),
  setLastError: (err) => set({ lastError: err }),
  clearMessages: () => set({ messages: new Map() }),
}));
