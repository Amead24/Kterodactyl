import { useServerMetrics } from '@/hooks/use-metrics';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

interface MetricsPanelProps {
  serverName: string;
  enabled: boolean;
}

function ProgressBar({ value, max }: { value: number; max: number }) {
  const percentage = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  return (
    <div className="h-2.5 w-full rounded-full bg-muted">
      <div
        className="h-2.5 rounded-full bg-primary transition-all"
        style={{ width: `${percentage}%` }}
      />
    </div>
  );
}

function formatCpu(millicores: number): string {
  if (millicores >= 1000) {
    return `${(millicores / 1000).toFixed(1)} cores`;
  }
  return `${millicores}m`;
}

function formatMemory(mib: number): string {
  if (mib >= 1024) {
    return `${(mib / 1024).toFixed(1)} GiB`;
  }
  return `${mib} MiB`;
}

/**
 * Displays CPU and memory usage for a game server with progress bars.
 *
 * Handles three states:
 * - Not enabled (server not running): placeholder message
 * - Loading: skeleton cards
 * - Error (metrics-server unavailable): graceful "unavailable" message (not a crash)
 */
export function MetricsPanel({ serverName, enabled }: MetricsPanelProps) {
  const { data: metrics, isLoading, error } = useServerMetrics(serverName, enabled);

  if (!enabled) {
    return (
      <div className="flex h-32 items-center justify-center">
        <p className="text-muted-foreground">Metrics available when server is running</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader><Skeleton className="h-5 w-20" /></CardHeader>
          <CardContent className="space-y-3">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-2.5 w-full" />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><Skeleton className="h-5 w-20" /></CardHeader>
          <CardContent className="space-y-3">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-2.5 w-full" />
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error || !metrics) {
    return (
      <div className="flex h-32 items-center justify-center">
        <p className="text-sm text-muted-foreground">Resource metrics unavailable</p>
      </div>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {/* CPU Card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">CPU Usage</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-baseline justify-between">
            <span className="text-2xl font-bold">{formatCpu(metrics.cpu)}</span>
            <span className="text-sm text-muted-foreground">
              / {formatCpu(metrics.cpuLimit)}
            </span>
          </div>
          <ProgressBar value={metrics.cpu} max={metrics.cpuLimit} />
        </CardContent>
      </Card>

      {/* Memory Card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Memory Usage</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-baseline justify-between">
            <span className="text-2xl font-bold">{formatMemory(metrics.memoryMiB)}</span>
            <span className="text-sm text-muted-foreground">
              / {formatMemory(metrics.memoryLimitMiB)}
            </span>
          </div>
          <ProgressBar value={metrics.memoryMiB} max={metrics.memoryLimitMiB} />
        </CardContent>
      </Card>
    </div>
  );
}
