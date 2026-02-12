import { useCallback, useEffect, useRef, useState } from 'react';
import { useAuthStore } from '@/stores/auth-store';

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error';

interface UseConsoleOptions {
  serverName: string;
  onMessage: (data: string) => void;
  enabled?: boolean;
}

interface UseConsoleReturn {
  status: ConnectionStatus;
  sendCommand: (cmd: string) => void;
  disconnect: () => void;
}

const MAX_RECONNECT_ATTEMPTS = 5;
const MAX_BACKOFF_MS = 30_000;

/**
 * Custom WebSocket hook for game server console connections.
 *
 * - Uses native WebSocket API (not react-use-websocket due to React 19 peer dep conflict).
 * - JWT auth via query param (browser WebSocket API cannot set headers during upgrade).
 * - Exponential backoff reconnection: 1s, 2s, 4s, 8s, 16s capped at 30s.
 */
export function useConsole({ serverName, onMessage, enabled = true }: UseConsoleOptions): UseConsoleReturn {
  const [status, setStatus] = useState<ConnectionStatus>('disconnected');

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectCountRef = useRef(0);
  const intentionalCloseRef = useRef(false);
  // Stable ref for the onMessage callback to avoid reconnecting on every render
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  const connect = useCallback(() => {
    const token = useAuthStore.getState().token;
    if (!token || !serverName) return;

    // Close existing connection if any
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/api/v1/gameservers/${serverName}/console?token=${token}`;

    setStatus('connecting');
    const ws = new WebSocket(url);

    ws.onopen = () => {
      setStatus('connected');
      reconnectCountRef.current = 0;
    };

    ws.onmessage = (event: MessageEvent) => {
      onMessageRef.current(event.data);
    };

    ws.onclose = (event: CloseEvent) => {
      wsRef.current = null;
      setStatus('disconnected');

      // If not intentional close and under retry limit, reconnect with backoff
      if (!intentionalCloseRef.current && event.code !== 1000 && reconnectCountRef.current < MAX_RECONNECT_ATTEMPTS) {
        const delay = Math.min(1000 * Math.pow(2, reconnectCountRef.current), MAX_BACKOFF_MS);
        reconnectCountRef.current += 1;
        reconnectTimeoutRef.current = setTimeout(connect, delay);
      }
    };

    ws.onerror = () => {
      setStatus('error');
    };

    wsRef.current = ws;
  }, [serverName]);

  const sendCommand = useCallback((cmd: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'command', data: cmd }));
    }
  }, []);

  const disconnect = useCallback(() => {
    intentionalCloseRef.current = true;
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close(1000);
      wsRef.current = null;
    }
    setStatus('disconnected');
  }, []);

  useEffect(() => {
    if (enabled) {
      intentionalCloseRef.current = false;
      reconnectCountRef.current = 0;
      connect();
    }

    return () => {
      intentionalCloseRef.current = true;
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
      if (wsRef.current) {
        wsRef.current.close(1000);
        wsRef.current = null;
      }
    };
  }, [enabled, connect]);

  return { status, sendCommand, disconnect };
}
