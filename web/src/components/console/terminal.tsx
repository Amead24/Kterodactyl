import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export interface TerminalHandle {
  write: (data: string) => void;
}

interface TerminalProps {
  onData?: (data: string) => void;
}

/**
 * xterm.js terminal component with auto-fit via ResizeObserver.
 *
 * Exposes a `write` method via ref for the parent to push data into the terminal.
 * Optional `onData` callback fires when the user types directly in the terminal.
 */
const TerminalComponent = forwardRef<TerminalHandle, TerminalProps>(
  ({ onData }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const terminalRef = useRef<Terminal | null>(null);

    useImperativeHandle(ref, () => ({
      write: (data: string) => {
        terminalRef.current?.write(data);
      },
    }));

    useEffect(() => {
      if (!containerRef.current) return;

      const terminal = new Terminal({
        cursorBlink: true,
        fontSize: 14,
        fontFamily: 'JetBrains Mono, Fira Code, monospace',
        theme: {
          background: '#1a1b26',
          foreground: '#a9b1d6',
        },
        scrollback: 5000,
        convertEol: true,
      });

      const fitAddon = new FitAddon();
      terminal.loadAddon(fitAddon);
      terminal.open(containerRef.current);
      fitAddon.fit();

      terminalRef.current = terminal;

      // Wire user input if callback provided
      let dataDisposable: { dispose: () => void } | undefined;
      if (onData) {
        dataDisposable = terminal.onData(onData);
      }

      // Resize terminal when container size changes
      const resizeObserver = new ResizeObserver(() => {
        try {
          fitAddon.fit();
        } catch {
          // Ignore fit errors during rapid resize or unmount
        }
      });
      resizeObserver.observe(containerRef.current);

      return () => {
        resizeObserver.disconnect();
        dataDisposable?.dispose();
        terminal.dispose();
        terminalRef.current = null;
      };
    }, [onData]);

    return <div ref={containerRef} className="h-full w-full" />;
  },
);

TerminalComponent.displayName = 'Terminal';

export default TerminalComponent;
