import { createFileRoute, useParams } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { getRunById, getContinuations } from "@/api/runs";
import { getLatestMetrics } from "@/api/metrics";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RunStatusBadge } from "@/components/runs/run-status-badge";
import { MetricsPanel } from "@/components/metrics/metrics-panel";
import { ContinuationsPanel } from "@/components/runs/continuations-panel";
import {
  formatRelativeTime,
  formatDuration,
  formatMetricValue,
} from "@/lib/utils";
import { GitBranch, Clock, Server, RotateCcw } from "lucide-react";

export const Route = createFileRoute("/_authenticated/runs/$runId")({
  component: RunDetailPage,
});

function RunDetailPage() {
  const { runId } = useParams({ from: "/_authenticated/runs/$runId" });

  const { data: run, isLoading: runLoading } = useQuery({
    queryKey: ["run", runId],
    queryFn: () => getRunById(runId),
    refetchInterval: (query) => {
      // Poll every 3s while running, then stop
      const status = query.state.data?.status;
      return status === "running" ? 3000 : false;
    },
  });

  const { data: latestMetrics } = useQuery({
    queryKey: ["metrics", "latest", runId],
    queryFn: () => getLatestMetrics(runId),
    refetchInterval: run?.status === "running" ? 3000 : false,
  });

  const { data: continuations = [] } = useQuery({
    queryKey: ["continuations", runId],
    queryFn: () => getContinuations(runId),
    refetchInterval: run?.status === "running" ? 5000 : false, // Poll while running
    retry: false,
  });

  const isLoading = runLoading;

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (!run) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        Run not found
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">{run.name}</h1>
            <RunStatusBadge status={run.status} />
          </div>
          {run.description && (
            <p className="text-muted-foreground mt-1">{run.description}</p>
          )}
          <div className="flex items-center gap-4 mt-2 text-sm text-muted-foreground">
            <span>Started {formatRelativeTime(run.started_at)}</span>
            {run.duration_seconds !== undefined && (
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {formatDuration(run.duration_seconds)}
              </span>
            )}
          </div>
        </div>
        {run.tags && run.tags.length > 0 && (
          <div className="flex gap-1">
            {run.tags.map((tag) => (
              <Badge key={tag} variant="outline">
                {tag}
              </Badge>
            ))}
          </div>
        )}
      </div>

      {/* Latest Metrics Summary */}
      {latestMetrics && Object.keys(latestMetrics).length > 0 && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {Object.entries(latestMetrics)
            .slice(0, 4)
            .map(([name, value]) => (
              <Card key={name}>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    {name}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {formatMetricValue(value)}
                  </div>
                </CardContent>
              </Card>
            ))}
        </div>
      )}

      {/* Tabs */}
      <Tabs defaultValue="metrics" className="space-y-4">
        <TabsList>
          <TabsTrigger value="metrics">Metrics</TabsTrigger>
          <TabsTrigger value="config">Config</TabsTrigger>
          {continuations.length > 0 && (
            <TabsTrigger value="continuations" className="gap-1">
              <RotateCcw className="h-3 w-3" />
              Continuations ({continuations.length})
            </TabsTrigger>
          )}
          <TabsTrigger value="system">System</TabsTrigger>
        </TabsList>

        <TabsContent value="metrics" className="space-y-4">
          <MetricsPanel
            runId={runId}
            isRunning={run.status === "running"}
            continuations={continuations}
          />
        </TabsContent>

        <TabsContent value="continuations" className="space-y-4">
          <ContinuationsPanel continuations={continuations} />
        </TabsContent>

        <TabsContent value="config" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Hyperparameters</CardTitle>
            </CardHeader>
            <CardContent>
              {run.config && Object.keys(run.config).length > 0 ? (
                <pre className="bg-muted p-4 rounded-md overflow-auto text-sm">
                  {JSON.stringify(run.config, null, 2)}
                </pre>
              ) : (
                <p className="text-muted-foreground">No configuration logged</p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="system" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            {/* Git Info */}
            {run.git_info && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <GitBranch className="h-5 w-5" />
                    Git Info
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2 text-sm">
                  {run.git_info.branch && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Branch</span>
                      <span className="font-mono">{run.git_info.branch}</span>
                    </div>
                  )}
                  {run.git_info.commit && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Commit</span>
                      <span className="font-mono">
                        {run.git_info.commit.slice(0, 7)}
                      </span>
                    </div>
                  )}
                  {run.git_info.dirty !== undefined && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Dirty</span>
                      <Badge
                        variant={run.git_info.dirty ? "warning" : "success"}
                      >
                        {run.git_info.dirty ? "Yes" : "No"}
                      </Badge>
                    </div>
                  )}
                  {run.git_info.message && (
                    <div className="pt-2 border-t">
                      <span className="text-muted-foreground">Message</span>
                      <p className="mt-1">{run.git_info.message}</p>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {/* System Info */}
            {run.system_info && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Server className="h-5 w-5" />
                    System Info
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2 text-sm">
                  {run.system_info.hostname && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Host</span>
                      <span>{run.system_info.hostname}</span>
                    </div>
                  )}
                  {run.system_info.os && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">OS</span>
                      <span>{run.system_info.os}</span>
                    </div>
                  )}
                  {run.system_info.python_version && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Python</span>
                      <span>{run.system_info.python_version}</span>
                    </div>
                  )}
                  {run.system_info.cpu_count && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">CPUs</span>
                      <span>{run.system_info.cpu_count}</span>
                    </div>
                  )}
                  {run.system_info.memory_gb && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Memory</span>
                      <span>{run.system_info.memory_gb.toFixed(1)} GB</span>
                    </div>
                  )}
                  {run.system_info.gpu_info &&
                    run.system_info.gpu_info.length > 0 && (
                      <div className="pt-2 border-t">
                        <span className="text-muted-foreground">GPUs</span>
                        <ul className="mt-1 space-y-1">
                          {run.system_info.gpu_info.map((gpu, i) => (
                            <li key={i} className="text-sm">
                              {gpu}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                </CardContent>
              </Card>
            )}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
