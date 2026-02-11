import { useQuery } from '@tanstack/react-query';
import { listGames, getGame } from '@/api/games';

/** Fetch and cache the list of available games. Stale time of 5 minutes (game list changes rarely). */
export function useGames() {
  return useQuery({
    queryKey: ['games'],
    queryFn: listGames,
    staleTime: 5 * 60 * 1000,
  });
}

/** Fetch and cache a single game by type. Only runs when gameType is truthy. */
export function useGame(gameType: string) {
  return useQuery({
    queryKey: ['games', gameType],
    queryFn: () => getGame(gameType),
    enabled: !!gameType,
  });
}
