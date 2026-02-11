import { Link } from 'react-router';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ServerCard } from '@/components/servers/server-card';
import { useServers } from '@/hooks/use-servers';

/** Responsive grid of server cards with loading and empty states. */
export function ServerList() {
  const { data, isLoading, error } = useServers();

  if (isLoading) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-52 rounded-xl" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <p className="text-sm text-destructive">
        Failed to load servers: {error.message}
      </p>
    );
  }

  const servers = data?.data ?? [];

  if (servers.length === 0) {
    return (
      <div className="flex flex-col items-center gap-4 py-16">
        <p className="text-muted-foreground">
          No servers yet. Create your first server!
        </p>
        <Button asChild>
          <Link to="/servers/create">
            <Plus className="mr-2 size-4" />
            Create Server
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {servers.map((server) => (
        <ServerCard key={server.name} server={server} />
      ))}
    </div>
  );
}
