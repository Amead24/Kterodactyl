import { useRef, useState } from 'react';
import { useConsole, type ConnectionStatus } from '@/hooks/use-console';
import TerminalComponent, { type TerminalHandle } from '@/components/console/terminal';
import { Input } from '@/components/ui/input';

interface ConsolePanelProps {
  serverName: string;
  enabled: boolean;
}

function statusColor(status: ConnectionStatus): string {
  switch (status) {
    case 'connected':
      return 'bg-green-500';
    case 'connecting':
      return 'bg-yellow-500';
    case 'disconnected':
    case 'error':
      return 'bg-red-500';
  }
}

function statusLabel(status: ConnectionStatus): string {
  switch (status) {
    case 'connected':
      return 'Connected';
    case 'connecting':
      return 'Connecting...';
    case 'disconnected':
      return 'Disconnected';
    case 'error':
      return 'Error';
  }
}

/**
 * Console panel combining the xterm.js terminal display with a connection status
 * indicator and a command input bar.
 *
 * When `enabled` is false (server not in Ready/Allocated state), shows a placeholder message.
 */
export function ConsolePanel({ serverName, enabled }: ConsolePanelProps) {
  const terminalRef = useRef<TerminalHandle>(null);
  const [input, setInput] = useState('');

  const { status, sendCommand } = useConsole({
    serverName,
    onMessage: (data) => terminalRef.current?.write(data),
    enabled,
  });

  if (!enabled) {
    return (
      <div className="flex h-full items-center justify-center">
        <p className="text-muted-foreground">Console available when server is running</p>
      </div>
    );
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (input.trim() && status === 'connected') {
      sendCommand(input);
      setInput('');
    }
  }

  return (
    <div className="flex h-full flex-col gap-2">
      {/* Status bar */}
      <div className="flex items-center gap-2 text-sm">
        <span className={`inline-block size-2.5 rounded-full ${statusColor(status)}`} />
        <span className="text-muted-foreground">{statusLabel(status)}</span>
      </div>

      {/* Terminal */}
      <div className="min-h-0 flex-1">
        <TerminalComponent ref={terminalRef} />
      </div>

      {/* Command input */}
      <form onSubmit={handleSubmit} className="flex gap-2">
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Type a command..."
          disabled={status !== 'connected'}
          className="font-mono"
        />
        <button
          type="submit"
          disabled={status !== 'connected' || !input.trim()}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
        >
          Send
        </button>
      </form>
    </div>
  );
}
