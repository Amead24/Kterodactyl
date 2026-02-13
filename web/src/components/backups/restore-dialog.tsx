import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { RotateCcw } from 'lucide-react';
import { useRestoreBackup } from '@/hooks/use-backups';

interface RestoreDialogProps {
  serverName: string;
  backupName: string;
}

export function RestoreDialog({ serverName, backupName }: RestoreDialogProps) {
  const restoreMutation = useRestoreBackup();

  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          disabled={restoreMutation.isPending}
        >
          <RotateCcw className="mr-1 size-3.5" />
          Restore
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Restore from backup?</AlertDialogTitle>
          <AlertDialogDescription>
            This will restore the server from backup &quot;{backupName}&quot;.
            Current game data will be overwritten and the server will restart.
            This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => restoreMutation.mutate({ serverName, backupName })}
          >
            Restore
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
