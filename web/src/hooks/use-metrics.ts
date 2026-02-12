import { useQuery } from '@tanstack/react-query';
import { getServerMetrics } from '@/api/servers';

/**
 * Polls the metrics endpoint for a game server every 5 seconds.
 *
 * - Only fetches when enabled (server is in Ready or Allocated state).
 * - Uses retry: 1 because metrics-server may not be installed.
 */
export function useServerMetrics(name: string, enabled: boolean) {
  return useQuery({
    queryKey: ['servers', name, 'metrics'],
    queryFn: () => getServerMetrics(name),
    refetchInterval: 5000,
    enabled: enabled && !!name,
    retry: 1,
  });
}
