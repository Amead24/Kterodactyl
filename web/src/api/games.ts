import { apiFetch } from '@/api/client';
import type { GameResponse, ListResponse } from '@/types/api';

/** GET /games -- List all available games. */
export function listGames(): Promise<ListResponse<GameResponse>> {
  return apiFetch<ListResponse<GameResponse>>('/games');
}

/** GET /games/{gameType} -- Get a single game by type. */
export function getGame(gameType: string): Promise<GameResponse> {
  return apiFetch<GameResponse>(`/games/${gameType}`);
}
