import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listMods, uploadMod, deleteMod } from '@/api/servers';

/** Fetch installed mods for a server. Polls every 30 seconds. */
export function useMods(name: string, enabled = true) {
  return useQuery({
    queryKey: ['mods', name],
    queryFn: () => listMods(name),
    enabled,
    refetchInterval: 30_000,
  });
}

/** Upload a mod file. Invalidates mod list on success. */
export function useUploadMod() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, file, onProgress }: { name: string; file: File; onProgress?: (p: number) => void }) =>
      uploadMod(name, file, onProgress),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['mods', vars.name] });
      // Also invalidate server query since server restarts after upload
      qc.invalidateQueries({ queryKey: ['servers', vars.name] });
    },
  });
}

/** Delete a mod file. Invalidates mod list on success. */
export function useDeleteMod() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, filename }: { name: string; filename: string }) =>
      deleteMod(name, filename),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['mods', vars.name] });
    },
  });
}
