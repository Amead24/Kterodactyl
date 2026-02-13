import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { createBackup, listBackups, deleteBackup, restoreBackup } from '@/api/backups';
import { toast } from 'sonner';

/** Fetch and cache backups for a server. Polls every 10 seconds. */
export function useBackups(serverName: string, enabled = true) {
  return useQuery({
    queryKey: ['backups', serverName],
    queryFn: () => listBackups(serverName),
    refetchInterval: 10_000,
    enabled: enabled && !!serverName,
  });
}

/** Create an on-demand backup. Invalidates backup list on success. */
export function useCreateBackup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (serverName: string) => createBackup(serverName),
    onSuccess: (_data, serverName) => {
      queryClient.invalidateQueries({ queryKey: ['backups', serverName] });
      toast.success('Backup started');
    },
    onError: (error: Error) => {
      toast.error(`Failed to start backup: ${error.message}`);
    },
  });
}

/** Delete a backup. Invalidates backup list on success. */
export function useDeleteBackup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ serverName, backupName }: { serverName: string; backupName: string }) =>
      deleteBackup(serverName, backupName),
    onSuccess: (_data, { serverName }) => {
      queryClient.invalidateQueries({ queryKey: ['backups', serverName] });
      toast.success('Backup deleted');
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete backup: ${error.message}`);
    },
  });
}

/** Restore from a backup. Invalidates server cache on success (server restarts). */
export function useRestoreBackup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ serverName, backupName }: { serverName: string; backupName: string }) =>
      restoreBackup(serverName, backupName),
    onSuccess: (_data, { serverName }) => {
      queryClient.invalidateQueries({ queryKey: ['servers', serverName] });
      toast.success('Restore started, server restarting...');
    },
    onError: (error: Error) => {
      toast.error(`Failed to restore: ${error.message}`);
    },
  });
}
