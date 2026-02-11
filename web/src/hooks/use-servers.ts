import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  listServers,
  getServer,
  createServer,
  deleteServer,
  startServer,
  stopServer,
  restartServer,
} from '@/api/servers';
import type { CreateGameServerRequest } from '@/types/api';

/** Fetch and cache the list of game servers. Polls every 5 seconds for live status updates. */
export function useServers() {
  return useQuery({
    queryKey: ['servers'],
    queryFn: listServers,
    refetchInterval: 5000,
  });
}

/** Fetch and cache a single game server by name. Polls every 2 seconds for responsive detail page updates. */
export function useServer(name: string) {
  return useQuery({
    queryKey: ['servers', name],
    queryFn: () => getServer(name),
    refetchInterval: 2000,
    enabled: !!name,
  });
}

/** Create a new game server. Invalidates server list on success. */
export function useCreateServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateGameServerRequest) => createServer(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (error: Error) => {
      toast.error(`Failed to create server: ${error.message}`);
    },
  });
}

/** Delete a game server. Invalidates server list on success. */
export function useDeleteServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deleteServer(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete server: ${error.message}`);
    },
  });
}

/** Start a stopped game server. Invalidates server list and detail cache on success. */
export function useStartServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => startServer(name),
    onSuccess: (_data, name) => {
      queryClient.invalidateQueries({ queryKey: ['servers'] });
      queryClient.invalidateQueries({ queryKey: ['servers', name] });
    },
    onError: (error: Error) => {
      toast.error(`Failed to start server: ${error.message}`);
    },
  });
}

/** Stop a running game server. Invalidates server list and detail cache on success. */
export function useStopServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => stopServer(name),
    onSuccess: (_data, name) => {
      queryClient.invalidateQueries({ queryKey: ['servers'] });
      queryClient.invalidateQueries({ queryKey: ['servers', name] });
    },
    onError: (error: Error) => {
      toast.error(`Failed to stop server: ${error.message}`);
    },
  });
}

/** Restart a game server. Invalidates server list and detail cache on success. */
export function useRestartServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => restartServer(name),
    onSuccess: (_data, name) => {
      queryClient.invalidateQueries({ queryKey: ['servers'] });
      queryClient.invalidateQueries({ queryKey: ['servers', name] });
    },
    onError: (error: Error) => {
      toast.error(`Failed to restart server: ${error.message}`);
    },
  });
}
