import { HardDrive } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useCreateBackup } from '@/hooks/use-backups';

interface BackupTriggerProps {
  serverName: string;
}

export function BackupTrigger({ serverName }: BackupTriggerProps) {
  const createMutation = useCreateBackup();

  return (
    <Button
      onClick={() => createMutation.mutate(serverName)}
      disabled={createMutation.isPending}
    >
      <HardDrive className="mr-2 size-4" />
      {createMutation.isPending ? 'Starting Backup...' : 'Create Backup'}
    </Button>
  );
}
