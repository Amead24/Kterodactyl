import { cn } from '@/lib/utils';
import type { GameServerResponse } from '@/types/api';

type ServerState = GameServerResponse['state'];

const stateStyles: Record<ServerState, string> = {
  Creating: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
  Starting: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
  Ready: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
  Allocated: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300',
  Shutdown: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
  Error: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300',
};

interface ServerStatusBadgeProps {
  state: ServerState;
  className?: string;
}

/** Badge showing the current game server state with color-coded styling. */
export function ServerStatusBadge({ state, className }: ServerStatusBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        stateStyles[state] ?? stateStyles.Error,
        className,
      )}
    >
      {state}
    </span>
  );
}
