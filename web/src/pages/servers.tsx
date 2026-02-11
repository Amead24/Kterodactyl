import { Link } from 'react-router';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { ServerList } from '@/components/servers/server-list';
import { useServers } from '@/hooks/use-servers';

export default function ServersPage() {
  const { data } = useServers();
  const serverCount = data?.count ?? 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">My Servers</h1>
          <p className="text-muted-foreground">
            {serverCount > 0
              ? `${serverCount} server${serverCount !== 1 ? 's' : ''}`
              : 'Manage your game servers'}
          </p>
        </div>
        <Button asChild>
          <Link to="/servers/create">
            <Plus className="mr-2 size-4" />
            Create Server
          </Link>
        </Button>
      </div>

      <ServerList />
    </div>
  );
}
