import { useParams, useNavigate, Link } from 'react-router';
import { format } from 'date-fns';
import {
  Activity,
  ArrowLeft,
  Copy,
  Package,
  Play,
  Square,
  RotateCcw,
  Terminal,
  Trash2,
} from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs';
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
import { ServerStatusBadge } from '@/components/servers/server-status-badge';
import { ConsolePanel } from '@/components/console/console-panel';
import { MetricsPanel } from '@/components/servers/metrics-panel';
import { ModUpload } from '@/components/mods/mod-upload';
import { ModList } from '@/components/mods/mod-list';
import {
  useServer,
  useStartServer,
  useStopServer,
  useRestartServer,
  useDeleteServer,
} from '@/hooks/use-servers';

export default function ServerDetailPage() {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const { data: server, isLoading, error } = useServer(name ?? '');
  const startMutation = useStartServer();
  const stopMutation = useStopServer();
  const restartMutation = useRestartServer();
  const deleteMutation = useDeleteServer();

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    );
  }

  if (error || !server) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-destructive">
          {error
            ? `Failed to load server: ${error.message}`
            : 'Server not found'}
        </p>
        <Button variant="outline" asChild>
          <Link to="/servers">
            <ArrowLeft className="mr-1 size-4" />
            Back to servers
          </Link>
        </Button>
      </div>
    );
  }

  const isActive = server.state === 'Ready' || server.state === 'Allocated';
  const isStopped = server.state === 'Shutdown' || server.state === 'Error';
  const parameters = server.parameters ?? {};

  function handleDelete() {
    deleteMutation.mutate(server!.name, {
      onSuccess: () => {
        toast.success(`Server "${server!.name}" deleted`);
        navigate('/servers');
      },
    });
  }

  function copyAddress() {
    if (server?.address) {
      navigator.clipboard.writeText(server.address);
      toast.success('Address copied to clipboard');
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/servers">
            <ArrowLeft className="mr-1 size-4" />
            Back
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">
              {server.name}
            </h1>
            <ServerStatusBadge state={server.state} />
          </div>
          <p className="text-muted-foreground">{server.gameType}</p>
        </div>
      </div>

      {/* Tabbed Content */}
      <Tabs defaultValue="overview" className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          {isActive && (
            <TabsTrigger value="console">
              <Terminal className="mr-1 size-4" />
              Console
            </TabsTrigger>
          )}
          {isActive && (
            <TabsTrigger value="resources">
              <Activity className="mr-1 size-4" />
              Resources
            </TabsTrigger>
          )}
          {isActive && (
            <TabsTrigger value="mods">
              <Package className="mr-1 size-4" />
              Mods
            </TabsTrigger>
          )}
        </TabsList>

        {/* Overview Tab -- existing content */}
        <TabsContent value="overview" className="space-y-6">
          {/* Connection Info (visible when Ready or Allocated) */}
          {isActive && server.address && (
            <Card>
              <CardHeader>
                <CardTitle>Connection Info</CardTitle>
                <CardDescription>
                  Connect to your server using the address below
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center gap-2">
                  <code className="flex-1 rounded-md bg-muted px-4 py-3 text-lg font-mono">
                    {server.address}
                  </code>
                  <Button variant="outline" size="icon" onClick={copyAddress}>
                    <Copy className="size-4" />
                  </Button>
                </div>
                {server.ports && server.ports.length > 0 && (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Name</TableHead>
                        <TableHead>Port</TableHead>
                        <TableHead>Protocol</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {server.ports.map((port) => (
                        <TableRow key={port.name}>
                          <TableCell className="font-medium">{port.name}</TableCell>
                          <TableCell>{port.port}</TableCell>
                          <TableCell>{port.protocol}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          )}

          {/* Server Info */}
          <Card>
            <CardHeader>
              <CardTitle>Server Info</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <p className="text-sm text-muted-foreground">Game Type</p>
                  <p className="font-medium">{server.gameType}</p>
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">State</p>
                  <ServerStatusBadge state={server.state} />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Created</p>
                  <p className="font-medium">
                    {format(new Date(server.createdAt), 'PPpp')}
                  </p>
                </div>
              </div>

              {Object.keys(parameters).length > 0 && (
                <>
                  <Separator />
                  <div>
                    <p className="mb-2 text-sm font-medium">Parameters</p>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Parameter</TableHead>
                          <TableHead>Value</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {Object.entries(parameters).map(([key, value]) => (
                          <TableRow key={key}>
                            <TableCell className="font-mono text-sm">
                              {key}
                            </TableCell>
                            <TableCell>{value}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </>
              )}
            </CardContent>
          </Card>

          {/* Actions */}
          <Card>
            <CardHeader>
              <CardTitle>Actions</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-3">
              {/* Start (when Shutdown or Error) */}
              {isStopped && (
                <Button
                  onClick={() => startMutation.mutate(server.name)}
                  disabled={startMutation.isPending}
                >
                  <Play className="mr-2 size-4" />
                  Start
                </Button>
              )}

              {/* Stop (when Ready or Allocated) */}
              {isActive && (
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button variant="outline" disabled={stopMutation.isPending}>
                      <Square className="mr-2 size-4" />
                      Stop
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Stop server?</AlertDialogTitle>
                      <AlertDialogDescription>
                        This will stop the server. Any connected players will be
                        disconnected. You can start it again later.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction
                        onClick={() => stopMutation.mutate(server.name)}
                      >
                        Stop Server
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}

              {/* Restart (when Ready or Allocated) */}
              {isActive && (
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button variant="outline" disabled={restartMutation.isPending}>
                      <RotateCcw className="mr-2 size-4" />
                      Restart
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Restart server?</AlertDialogTitle>
                      <AlertDialogDescription>
                        This will restart the server. Any connected players will be
                        temporarily disconnected.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction
                        onClick={() => restartMutation.mutate(server.name)}
                      >
                        Restart Server
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}

              {/* Delete (always visible) */}
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button
                    variant="destructive"
                    disabled={deleteMutation.isPending}
                  >
                    <Trash2 className="mr-2 size-4" />
                    Delete
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Delete server?</AlertDialogTitle>
                    <AlertDialogDescription>
                      This will permanently delete the server &quot;{server.name}&quot;.
                      This action cannot be undone.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction onClick={handleDelete}>
                      Delete Server
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Console Tab */}
        <TabsContent value="console">
          <Card>
            <CardHeader>
              <CardTitle>Server Console</CardTitle>
              <CardDescription>View server output and send commands</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-[500px]">
                <ConsolePanel serverName={server.name} enabled={isActive} />
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Resources Tab */}
        <TabsContent value="resources">
          <MetricsPanel serverName={server.name} enabled={isActive} />
        </TabsContent>

        {/* Mods Tab */}
        <TabsContent value="mods" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Upload Mods</CardTitle>
              <CardDescription>Upload mod files to your server. The server will restart after upload.</CardDescription>
            </CardHeader>
            <CardContent>
              <ModUpload serverName={server.name} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Installed Mods</CardTitle>
              <CardDescription>Manage installed mod files</CardDescription>
            </CardHeader>
            <CardContent>
              <ModList serverName={server.name} enabled={isActive} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
