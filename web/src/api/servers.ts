import { apiFetch, apiUpload } from '@/api/client';
import type {
  GameServerResponse,
  CreateGameServerRequest,
  UpdateGameServerRequest,
  ListResponse,
  MetricsResponse,
  ModFileResponse,
} from '@/types/api';

/** GET /gameservers -- List all game servers for the current user. */
export function listServers(): Promise<ListResponse<GameServerResponse>> {
  return apiFetch<ListResponse<GameServerResponse>>('/gameservers');
}

/** GET /gameservers/{name} -- Get a single game server by name. */
export function getServer(name: string): Promise<GameServerResponse> {
  return apiFetch<GameServerResponse>(`/gameservers/${name}`);
}

/** POST /gameservers -- Create a new game server. */
export function createServer(
  data: CreateGameServerRequest,
): Promise<GameServerResponse> {
  return apiFetch<GameServerResponse>('/gameservers', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/** PUT /gameservers/{name} -- Update a game server's parameters. */
export function updateServer(
  name: string,
  data: UpdateGameServerRequest,
): Promise<GameServerResponse> {
  return apiFetch<GameServerResponse>(`/gameservers/${name}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/** DELETE /gameservers/{name} -- Delete a game server. */
export function deleteServer(name: string): Promise<void> {
  return apiFetch<void>(`/gameservers/${name}`, {
    method: 'DELETE',
  });
}

/** POST /gameservers/{name}/start -- Start a stopped game server. */
export function startServer(name: string): Promise<GameServerResponse> {
  return apiFetch<GameServerResponse>(`/gameservers/${name}/start`, {
    method: 'POST',
  });
}

/** POST /gameservers/{name}/stop -- Stop a running game server. */
export function stopServer(name: string): Promise<GameServerResponse> {
  return apiFetch<GameServerResponse>(`/gameservers/${name}/stop`, {
    method: 'POST',
  });
}

/** POST /gameservers/{name}/restart -- Restart a game server. */
export function restartServer(name: string): Promise<GameServerResponse> {
  return apiFetch<GameServerResponse>(`/gameservers/${name}/restart`, {
    method: 'POST',
  });
}

/** GET /gameservers/{name}/metrics -- Get resource usage for a game server. */
export function getServerMetrics(name: string): Promise<MetricsResponse> {
  return apiFetch<MetricsResponse>(`/gameservers/${name}/metrics`);
}

/** POST /gameservers/{name}/mods -- Upload a mod file. */
export function uploadMod(
  name: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<ModFileResponse> {
  return apiUpload<ModFileResponse>(`/gameservers/${name}/mods`, file, onProgress);
}

/** GET /gameservers/{name}/mods -- List installed mods. */
export function listMods(name: string): Promise<ListResponse<ModFileResponse>> {
  return apiFetch<ListResponse<ModFileResponse>>(`/gameservers/${name}/mods`);
}

/** DELETE /gameservers/{name}/mods/{filename} -- Delete a mod. */
export function deleteMod(name: string, filename: string): Promise<void> {
  return apiFetch<void>(`/gameservers/${name}/mods/${encodeURIComponent(filename)}`, {
    method: 'DELETE',
  });
}
