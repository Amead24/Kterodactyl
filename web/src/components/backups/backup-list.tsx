import { format } from 'date-fns';
import { Trash2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { useBackups, useDeleteBackup } from '@/hooks/use-backups';
import { useAuthStore } from '@/stores/auth-store';
import { RestoreDialog } from './restore-dialog';
import type { BackupResponse } from '@/types/api';

interface BackupListProps {
  serverName: string;
  enabled: boolean;
}

type BackupState = BackupResponse['state'];

const stateStyles: Record<BackupState, string> = {
  Pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
  InProgress: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
  Completed: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
  Failed: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300',
};

function BackupStateBadge({ state }: { state: BackupState }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        stateStyles[state] ?? stateStyles.Failed,
      )}
    >
      {state}
    </span>
  );
}

function formatBytes(bytes: number | undefined): string {
  if (bytes == null || bytes === 0) return '-';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

export function BackupList({ serverName, enabled }: BackupListProps) {
  const { data, isLoading } = useBackups(serverName, enabled);
  const deleteMutation = useDeleteBackup();
  const isAdmin = useAuthStore((s) => s.user?.role === 'admin');

  const backups = data?.data ?? [];

  if (isLoading) {
    return <p className="text-sm text-muted-foreground">Loading backups...</p>;
  }

  if (backups.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No backups yet. Create a backup to get started.
      </p>
    );
  }

  function handleDelete(backupName: string) {
    deleteMutation.mutate({ serverName, backupName });
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>State</TableHead>
          <TableHead>Size</TableHead>
          <TableHead>Started</TableHead>
          <TableHead>Completed</TableHead>
          {isAdmin && <TableHead className="w-32">Actions</TableHead>}
        </TableRow>
      </TableHeader>
      <TableBody>
        {backups.map((backup) => (
          <TableRow key={backup.name}>
            <TableCell className="font-mono text-sm max-w-[200px] truncate">
              {backup.name}
            </TableCell>
            <TableCell>
              <BackupStateBadge state={backup.state} />
            </TableCell>
            <TableCell className="text-muted-foreground">
              {formatBytes(backup.size)}
            </TableCell>
            <TableCell className="text-muted-foreground text-sm">
              {backup.startedAt ? format(new Date(backup.startedAt), 'PPp') : '-'}
            </TableCell>
            <TableCell className="text-muted-foreground text-sm">
              {backup.completedAt ? format(new Date(backup.completedAt), 'PPp') : '-'}
            </TableCell>
            {isAdmin && (
              <TableCell>
                <div className="flex items-center gap-1">
                  {backup.state === 'Completed' && (
                    <RestoreDialog
                      serverName={serverName}
                      backupName={backup.name}
                    />
                  )}
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        disabled={deleteMutation.isPending}
                      >
                        <Trash2 className="size-4 text-destructive" />
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>Delete backup?</AlertDialogTitle>
                        <AlertDialogDescription>
                          This will permanently delete the backup &quot;{backup.name}&quot;
                          from S3 storage. This action cannot be undone.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction onClick={() => handleDelete(backup.name)}>
                          Delete
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              </TableCell>
            )}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
