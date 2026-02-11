import { Link } from 'react-router';
import { formatDistanceToNow } from 'date-fns';
import { Play, Square, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { ServerStatusBadge } from '@/components/servers/server-status-badge';
import {
  useStartServer,
  useStopServer,
  useDeleteServer,
} from '@/hooks/use-servers';
import type { GameServerResponse } from '@/types/api';

interface ServerCardProps {
  server: GameServerResponse;
}

/** Card displaying a game server with status, connection info, and quick actions. */
export function ServerCard({ server }: ServerCardProps) {
  const startMutation = useStartServer();
  const stopMutation = useStopServer();
  const deleteMutation = useDeleteServer();

  const isActive = server.state === 'Ready' || server.state === 'Allocated';
  const isStopped = server.state === 'Shutdown' || server.state === 'Error';

  return (
    <Card className="flex flex-col">
      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <Link
            to={`/servers/${server.name}`}
            className="hover:underline"
          >
            <CardTitle className="text-lg">{server.name}</CardTitle>
          </Link>
          <ServerStatusBadge state={server.state} />
        </div>
        <p className="text-sm text-muted-foreground">{server.gameType}</p>
      </CardHeader>
      <CardContent className="flex-1 space-y-1 text-sm text-muted-foreground">
        {isActive && server.address && (
          <p className="font-mono text-foreground">{server.address}</p>
        )}
        <p>
          Created{' '}
          {formatDistanceToNow(new Date(server.createdAt), {
            addSuffix: true,
          })}
        </p>
      </CardContent>
      <CardFooter className="gap-2">
        {isStopped && (
          <Button
            variant="outline"
            size="sm"
            onClick={(e) => {
              e.preventDefault();
              startMutation.mutate(server.name);
            }}
            disabled={startMutation.isPending}
          >
            <Play className="mr-1 size-3" />
            Start
          </Button>
        )}
        {isActive && (
          <Button
            variant="outline"
            size="sm"
            onClick={(e) => {
              e.preventDefault();
              stopMutation.mutate(server.name);
            }}
            disabled={stopMutation.isPending}
          >
            <Square className="mr-1 size-3" />
            Stop
          </Button>
        )}
        <Button
          variant="ghost"
          size="sm"
          className="ml-auto text-destructive hover:text-destructive"
          onClick={(e) => {
            e.preventDefault();
            deleteMutation.mutate(server.name);
          }}
          disabled={deleteMutation.isPending}
        >
          <Trash2 className="size-3" />
        </Button>
      </CardFooter>
    </Card>
  );
}
