import { apiFetch } from './client';
import type { BackupResponse, ListResponse } from '@/types/api';

/** POST /gameservers/{name}/backups -- Create an on-demand backup. */
export function createBackup(serverName: string): Promise<BackupResponse> {
  return apiFetch<BackupResponse>(`/gameservers/${serverName}/backups`, {
    method: 'POST',
  });
}

/** GET /gameservers/{name}/backups -- List all backups for a server. */
export function listBackups(serverName: string): Promise<ListResponse<BackupResponse>> {
  return apiFetch<ListResponse<BackupResponse>>(`/gameservers/${serverName}/backups`);
}

/** DELETE /gameservers/{name}/backups/{backupName} -- Delete a backup. */
export function deleteBackup(serverName: string, backupName: string): Promise<void> {
  return apiFetch<void>(`/gameservers/${serverName}/backups/${backupName}`, {
    method: 'DELETE',
  });
}

/** POST /gameservers/{name}/backups/{backupName}/restore -- Restore from a backup. */
export function restoreBackup(serverName: string, backupName: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/gameservers/${serverName}/backups/${backupName}/restore`, {
    method: 'POST',
  });
}
