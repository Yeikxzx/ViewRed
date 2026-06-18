import { describe, it, expect, beforeEach } from 'vitest';
import { useSSEStore, type ConnectionStatus, type CaptureStatus } from './sse-store';

describe('useSSEStore', () => {
  beforeEach(() => {
    // Reset store state between tests
    useSSEStore.setState({
      messages: new Map(),
      connectionStatus: 'disconnected',
      captureStatus: 'unknown',
      lastError: null,
    });
  });

  describe('initial state', () => {
    it('starts with empty messages Map', () => {
      const { messages } = useSSEStore.getState();
      expect(messages).toBeInstanceOf(Map);
      expect(messages.size).toBe(0);
    });

    it('starts with connectionStatus "disconnected"', () => {
      const { connectionStatus } = useSSEStore.getState();
      expect(connectionStatus).toBe('disconnected');
    });

    it('starts with captureStatus "unknown"', () => {
      const { captureStatus } = useSSEStore.getState();
      expect(captureStatus).toBe('unknown');
    });

    it('starts with lastError null', () => {
      const { lastError } = useSSEStore.getState();
      expect(lastError).toBeNull();
    });
  });

  describe('addMessage', () => {
    it('stores a message under the given topic', () => {
      const data = { foo: 'bar' };
      useSSEStore.getState().addMessage('packets', data);

      const { messages } = useSSEStore.getState();
      expect(messages.get('packets')).toEqual(data);
      expect(messages.size).toBe(1);
    });

    it('preserves previous topics when adding a new one', () => {
      useSSEStore.getState().addMessage('packets', { id: 1 });
      useSSEStore.getState().addMessage('topology', { nodes: 5 });

      const { messages } = useSSEStore.getState();
      expect(messages.size).toBe(2);
      expect(messages.get('packets')).toEqual({ id: 1 });
      expect(messages.get('topology')).toEqual({ nodes: 5 });
    });

    it('overwrites the latest payload for an existing topic', () => {
      useSSEStore.getState().addMessage('packets', { id: 1 });
      useSSEStore.getState().addMessage('packets', { id: 2 });

      const { messages } = useSSEStore.getState();
      expect(messages.size).toBe(1);
      expect(messages.get('packets')).toEqual({ id: 2 });
    });

    it('produces an immutable Map (does not mutate previous state)', () => {
      const before = useSSEStore.getState().messages;
      useSSEStore.getState().addMessage('packets', { id: 1 });
      const after = useSSEStore.getState().messages;

      expect(after).not.toBe(before);
      expect(before.size).toBe(0);
      expect(after.size).toBe(1);
    });

    it('accepts arbitrary unknown payloads', () => {
      const payloads: unknown[] = [
        'plain string',
        42,
        null,
        { nested: { deep: true } },
        [1, 2, 3],
      ];
      payloads.forEach((payload, i) => {
        useSSEStore.getState().addMessage(`topic-${i}`, payload);
      });

      const { messages } = useSSEStore.getState();
      expect(messages.size).toBe(payloads.length);
      payloads.forEach((payload, i) => {
        expect(messages.get(`topic-${i}`)).toEqual(payload);
      });
    });
  });

  describe('setConnectionStatus', () => {
    it.each<ConnectionStatus>(['connecting', 'connected', 'disconnected', 'reconnecting'])(
      'updates connectionStatus to "%s"',
      (status) => {
        useSSEStore.getState().setConnectionStatus(status);
        expect(useSSEStore.getState().connectionStatus).toBe(status);
      }
    );
  });

  describe('setCaptureStatus', () => {
    it.each<CaptureStatus>(['running', 'paused', 'unknown'])(
      'updates captureStatus to "%s"',
      (status) => {
        useSSEStore.getState().setCaptureStatus(status);
        expect(useSSEStore.getState().captureStatus).toBe(status);
      }
    );
  });

  describe('setLastError', () => {
    it('stores an error message', () => {
      useSSEStore.getState().setLastError('connection lost');
      expect(useSSEStore.getState().lastError).toBe('connection lost');
    });

    it('clears the error when set to null', () => {
      useSSEStore.getState().setLastError('connection lost');
      useSSEStore.getState().setLastError(null);
      expect(useSSEStore.getState().lastError).toBeNull();
    });
  });

  describe('clearMessages', () => {
    it('empties the messages Map', () => {
      useSSEStore.getState().addMessage('a', 1);
      useSSEStore.getState().addMessage('b', 2);
      useSSEStore.getState().addMessage('c', 3);
      expect(useSSEStore.getState().messages.size).toBe(3);

      useSSEStore.getState().clearMessages();

      const { messages } = useSSEStore.getState();
      expect(messages).toBeInstanceOf(Map);
      expect(messages.size).toBe(0);
    });

    it('produces a fresh Map instance (does not mutate the previous one)', () => {
      useSSEStore.getState().addMessage('a', 1);
      const before = useSSEStore.getState().messages;

      useSSEStore.getState().clearMessages();
      const after = useSSEStore.getState().messages;

      expect(after).not.toBe(before);
      expect(before.size).toBe(1);
      expect(after.size).toBe(0);
    });
  });

  describe('isolation between actions', () => {
    it('does not affect unrelated state when adding a message', () => {
      useSSEStore.setState({
        connectionStatus: 'connected',
        captureStatus: 'running',
        lastError: 'previous',
      });

      useSSEStore.getState().addMessage('packets', { id: 1 });

      const state = useSSEStore.getState();
      expect(state.connectionStatus).toBe('connected');
      expect(state.captureStatus).toBe('running');
      expect(state.lastError).toBe('previous');
      expect(state.messages.get('packets')).toEqual({ id: 1 });
    });
  });
});
