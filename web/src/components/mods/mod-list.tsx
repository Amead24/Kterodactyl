import { Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { useMods, useDeleteMod } from '@/hooks/use-mods';

interface ModListProps {
  serverName: string;
  enabled: boolean;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function ModList({ serverName, enabled }: ModListProps) {
  const { data, isLoading } = useMods(serverName, enabled);
  const deleteMutation = useDeleteMod();

  const mods = data?.data ?? [];

  if (isLoading) {
    return <p className="text-sm text-muted-foreground">Loading mods...</p>;
  }

  if (mods.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No mods installed. Upload mod files above.
      </p>
    );
  }

  function handleDelete(filename: string) {
    deleteMutation.mutate(
      { name: serverName, filename },
      {
        onSuccess: () => toast.success(`Deleted ${filename}`),
        onError: (err) => toast.error(`Failed to delete: ${err.message}`),
      },
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Filename</TableHead>
          <TableHead>Size</TableHead>
          <TableHead className="w-16"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {mods.map((mod) => (
          <TableRow key={mod.name}>
            <TableCell className="font-mono text-sm">{mod.name}</TableCell>
            <TableCell className="text-muted-foreground">{formatSize(mod.size)}</TableCell>
            <TableCell>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="ghost" size="icon" disabled={deleteMutation.isPending}>
                    <Trash2 className="size-4 text-destructive" />
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Delete mod?</AlertDialogTitle>
                    <AlertDialogDescription>
                      Remove &quot;{mod.name}&quot; from this server. The server will NOT restart automatically.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction onClick={() => handleDelete(mod.name)}>
                      Delete
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
