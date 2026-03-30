import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useState, useMemo } from "react";
import { getProjectRuns } from "@/api/projects";
import { getProjectSweeps } from "@/api/sweeps";
import { getMetricNames } from "@/api/metrics";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { RunStatusBadge } from "@/components/runs/run-status-badge";
import { formatRelativeTime, formatDuration } from "@/lib/utils";
import {
  ChevronLeft,
  Activity,
  X,
  Cloud,
  Shuffle,
  Grid3X3,
  Trophy,
  ArrowRight,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ComparisonChart } from "@/components/metrics/comparison-chart";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import type { XAxisMode, YAxisScale } from "@/components/metrics/metric-chart";
import type { SweepStatus } from "@/api/types";

export const Route = createFileRoute("/projects/$projectSlug")({
  component: ProjectRunsPage,
});

function SweepStatusBadge({ status }: { status: SweepStatus }) {
  const variants: Record<
    SweepStatus,
    "default" | "success" | "destructive" | "secondary"
  > = {
    running: "default",
    completed: "success",
    failed: "destructive",
    stopped: "secondary",
  };
  return <Badge variant={variants[status]}>{status}</Badge>;
}

function ProjectRunsPage() {
  const projectSlug =
    window.location.pathname.split("/projects/")[1]?.split("/")[0] || "";
  const searchParams = new URLSearchParams(window.location.search);
  const source = searchParams.get("source") || "local";
  const team = searchParams.get("team") || "";
  const isRemote = source === "remote";

  const sweepHref = (sweepId: string) => {
    if (isRemote && team) {
      return `/sweeps/${sweepId}?source=remote&team=${encodeURIComponent(team)}&project=${encodeURIComponent(projectSlug)}`;
    }
    return `/sweeps/${sweepId}`;
  };

  const [activeTab, setActiveTab] = useState("runs");
  const [selectedRunIds, setSelectedRunIds] = useState<Set<string>>(new Set());
  const [xAxisMode, setXAxisMode] = useState<XAxisMode>("step");
  const [yAxisScale, setYAxisScale] = useState<YAxisScale>("linear");
  const [selectedMetric, setSelectedMetric] = useState<string | null>(null);

  const { data: runs, isLoading: runsLoading } = useQuery({
    queryKey: ["project-runs", projectSlug, source, team],
    queryFn: () => getProjectRuns(projectSlug, { source, team }),
    refetchInterval: 3000,
  });

  const { data: sweeps, isLoading: sweepsLoading } = useQuery({
    queryKey: ["project-sweeps", projectSlug, source, team],
    queryFn: () =>
      getProjectSweeps(
        projectSlug,
        { source, team },
        { limit: 50, order_by: "started_at", order_dir: "desc" },
      ),
    refetchInterval: 5000,
  });

  const selectedRuns = useMemo(() => {
    if (!runs) return [];
    return runs.filter((r) => selectedRunIds.has(r.id));
  }, [runs, selectedRunIds]);

  const firstSelectedRunId = selectedRuns[0]?.id;
  const { data: metricNames } = useQuery({
    queryKey: ["metrics", "names", firstSelectedRunId],
    queryFn: () => getMetricNames(firstSelectedRunId!),
    enabled: !!firstSelectedRunId,
    refetchInterval: 5000,
  });

  const toggleRunSelection = (runId: string) => {
    setSelectedRunIds((prev) => {
      const next = new Set(prev);
      if (next.has(runId)) {
        next.delete(runId);
      } else {
        next.add(runId);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (!runs) return;
    if (selectedRunIds.size === runs.length) {
      setSelectedRunIds(new Set());
    } else {
      setSelectedRunIds(new Set(runs.map((r) => r.id)));
    }
  };

  const clearComparison = () => {
    setSelectedRunIds(new Set());
    setSelectedMetric(null);
  };

  const allSelected =
    runs && runs.length > 0 && selectedRunIds.size === runs.length;

  const isLoading = activeTab === "runs" ? runsLoading : sweepsLoading;

  if (isLoading && activeTab === "runs" && !runs) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  const activeMetric = selectedMetric || metricNames?.[0];
  const isComparing = selectedRunIds.size > 0;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="space-y-2">
        <a href="/projects">
          <Button variant="ghost" size="sm" className="gap-1 -ml-2">
            <ChevronLeft className="h-4 w-4" />
            Back
          </Button>
        </a>
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <Activity className="h-7 w-7" />
              {projectSlug}
              {isRemote && (
                <Badge variant="outline" className="text-sm font-normal gap-1">
                  <Cloud className="h-3 w-3" />
                  Cloud
                </Badge>
              )}
            </h1>
            <p className="text-muted-foreground">
              {runs?.length || 0} run{(runs?.length || 0) !== 1 ? "s" : ""}
              {(sweeps?.length || 0) > 0 && (
                <span>
                  {" "}
                  · {sweeps?.length} sweep
                  {(sweeps?.length || 0) !== 1 ? "s" : ""}
                </span>
              )}
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="runs">
            Runs {runs !== undefined && `(${runs.length})`}
          </TabsTrigger>
          <TabsTrigger value="sweeps">
            Sweeps {sweeps !== undefined && `(${sweeps.length})`}
          </TabsTrigger>
        </TabsList>

        {/* Runs Tab */}
        <TabsContent value="runs" className="mt-4">
          {!runs || runs.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center text-muted-foreground">
                No runs in this project yet
              </CardContent>
            </Card>
          ) : (
            <div className="flex gap-4">
              {/* Runs table */}
              <div className={isComparing ? "w-1/4 min-w-[280px]" : "w-full"}>
                {isComparing && (
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-sm text-muted-foreground">
                      {selectedRunIds.size} selected
                    </span>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={clearComparison}
                      className="gap-1 h-7"
                    >
                      <X className="h-3 w-3" />
                      Clear
                    </Button>
                  </div>
                )}

                <Card>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-10">
                          <div className="flex items-center justify-center">
                            <Checkbox
                              checked={allSelected || false}
                              onCheckedChange={toggleSelectAll}
                            />
                          </div>
                        </TableHead>
                        <TableHead>Name</TableHead>
                        <TableHead className="w-20">Status</TableHead>
                        {!isComparing && (
                          <>
                            <TableHead>Tags</TableHead>
                            <TableHead>Started</TableHead>
                            <TableHead>Duration</TableHead>
                          </>
                        )}
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {runs.map((run) => (
                        <TableRow
                          key={run.id}
                          className={
                            selectedRunIds.has(run.id) ? "bg-muted/50" : ""
                          }
                        >
                          <TableCell className="py-2">
                            <div className="flex items-center justify-center">
                              <Checkbox
                                checked={selectedRunIds.has(run.id)}
                                onCheckedChange={() =>
                                  toggleRunSelection(run.id)
                                }
                              />
                            </div>
                          </TableCell>
                          <TableCell className="py-2">
                            <a
                              href={`/runs/${run.id}`}
                              className="font-medium hover:underline text-sm"
                            >
                              {run.name}
                            </a>
                            {!isComparing && run.git_info?.branch && (
                              <span className="ml-2 text-xs text-muted-foreground">
                                {run.git_info.branch}
                              </span>
                            )}
                          </TableCell>
                          <TableCell className="py-2">
                            <RunStatusBadge status={run.status} />
                          </TableCell>
                          {!isComparing && (
                            <>
                              <TableCell>
                                <div className="flex gap-1 flex-wrap">
                                  {run.tags?.slice(0, 3).map((tag) => (
                                    <Badge
                                      key={tag}
                                      variant="outline"
                                      className="text-xs"
                                    >
                                      {tag}
                                    </Badge>
                                  ))}
                                  {run.tags && run.tags.length > 3 && (
                                    <Badge
                                      variant="outline"
                                      className="text-xs"
                                    >
                                      +{run.tags.length - 3}
                                    </Badge>
                                  )}
                                </div>
                              </TableCell>
                              <TableCell className="text-muted-foreground">
                                {formatRelativeTime(run.started_at)}
                              </TableCell>
                              <TableCell className="text-muted-foreground">
                                {run.duration_seconds !== undefined
                                  ? formatDuration(run.duration_seconds)
                                  : run.status === "running"
                                    ? "Running..."
                                    : "-"}
                              </TableCell>
                            </>
                          )}
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </Card>
              </div>

              {/* Comparison panel */}
              {isComparing && (
                <div className="flex-1">
                  <Card className="h-full">
                    <CardHeader className="pb-2">
                      <div className="flex items-center justify-between">
                        <CardTitle className="text-lg">
                          Run Comparison
                        </CardTitle>
                        <div className="flex items-center gap-4">
                          <div className="flex items-center gap-1">
                            <span className="text-xs text-muted-foreground">
                              X:
                            </span>
                            <Button
                              variant={
                                xAxisMode === "step" ? "secondary" : "ghost"
                              }
                              size="sm"
                              className="h-6 px-2 text-xs"
                              onClick={() => setXAxisMode("step")}
                            >
                              Step
                            </Button>
                            <Button
                              variant={
                                xAxisMode === "relativeTime"
                                  ? "secondary"
                                  : "ghost"
                              }
                              size="sm"
                              className="h-6 px-2 text-xs"
                              onClick={() => setXAxisMode("relativeTime")}
                            >
                              Time
                            </Button>
                          </div>
                          <div className="flex items-center gap-1">
                            <span className="text-xs text-muted-foreground">
                              Y:
                            </span>
                            <Button
                              variant={
                                yAxisScale === "linear" ? "secondary" : "ghost"
                              }
                              size="sm"
                              className="h-6 px-2 text-xs"
                              onClick={() => setYAxisScale("linear")}
                            >
                              Linear
                            </Button>
                            <Button
                              variant={
                                yAxisScale === "log" ? "secondary" : "ghost"
                              }
                              size="sm"
                              className="h-6 px-2 text-xs"
                              onClick={() => setYAxisScale("log")}
                            >
                              Log
                            </Button>
                          </div>
                        </div>
                      </div>
                    </CardHeader>
                    <CardContent>
                      {selectedRuns.length < 2 ? (
                        <div className="text-muted-foreground text-center py-12">
                          Select at least 2 runs to compare
                        </div>
                      ) : metricNames && metricNames.length > 0 ? (
                        <Tabs
                          value={activeMetric || metricNames[0]}
                          onValueChange={setSelectedMetric}
                        >
                          <TabsList className="flex-wrap h-auto gap-1 mb-4">
                            {metricNames.map((name) => (
                              <TabsTrigger
                                key={name}
                                value={name}
                                className="text-xs"
                              >
                                {name}
                              </TabsTrigger>
                            ))}
                          </TabsList>
                          {metricNames.map((name) => (
                            <TabsContent key={name} value={name}>
                              <ComparisonChart
                                runs={selectedRuns.map((r) => ({
                                  id: r.id,
                                  name: r.name,
                                }))}
                                metricName={name}
                                height={400}
                                xAxisMode={xAxisMode}
                                yAxisScale={yAxisScale}
                              />
                              <p className="text-xs text-muted-foreground mt-2">
                                Click on a run in the legend to highlight it
                              </p>
                            </TabsContent>
                          ))}
                        </Tabs>
                      ) : (
                        <div className="text-muted-foreground text-center py-12">
                          No metrics available for comparison
                        </div>
                      )}
                    </CardContent>
                  </Card>
                </div>
              )}
            </div>
          )}
        </TabsContent>

        {/* Sweeps Tab */}
        <TabsContent value="sweeps" className="mt-4">
          {sweepsLoading ? (
            <Skeleton className="h-64 w-full" />
          ) : !sweeps || sweeps.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <p className="text-muted-foreground mb-2">No sweeps yet</p>
                <p className="text-sm text-muted-foreground mb-4">
                  Start a hyperparameter sweep with the SDK:
                </p>
                <pre className="bg-muted p-4 rounded-md text-sm text-left inline-block overflow-x-auto">
                  <code>{`import p95

sweep_id = p95.sweep(
    project="${projectSlug}",
    config=p95.SweepConfig(
        method="random",
        metric="val_loss",
        goal="minimize",
        parameters=[...],
        max_runs=20,
    ),
)

p95.agent(sweep_id, train_fn)`}</code>
                </pre>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead className="w-24">Method</TableHead>
                    <TableHead className="w-24">Status</TableHead>
                    <TableHead className="w-32">Metric</TableHead>
                    <TableHead className="w-20 text-center">Runs</TableHead>
                    <TableHead className="w-32">Best Value</TableHead>
                    <TableHead className="w-32">Started</TableHead>
                    <TableHead className="w-8"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sweeps.map((sweep) => (
                    <TableRow key={sweep.id}>
                      <TableCell className="py-3">
                        <a
                          href={sweepHref(sweep.id)}
                          className="font-medium hover:underline"
                        >
                          {sweep.name}
                        </a>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1 text-sm text-muted-foreground">
                          {sweep.method === "random" ? (
                            <Shuffle className="h-3 w-3" />
                          ) : (
                            <Grid3X3 className="h-3 w-3" />
                          )}
                          {sweep.method}
                        </div>
                      </TableCell>
                      <TableCell>
                        <SweepStatusBadge status={sweep.status} />
                      </TableCell>
                      <TableCell>
                        <span className="text-sm">
                          {sweep.metric_name}
                          <span className="text-muted-foreground ml-1">
                            ({sweep.metric_goal})
                          </span>
                        </span>
                      </TableCell>
                      <TableCell className="text-center">
                        <span className="text-sm">
                          {sweep.run_count}
                          {sweep.max_runs && (
                            <span className="text-muted-foreground">
                              /{sweep.max_runs}
                            </span>
                          )}
                        </span>
                      </TableCell>
                      <TableCell>
                        {sweep.best_value !== undefined &&
                        sweep.best_value !== null ? (
                          <div className="flex items-center gap-1 text-sm">
                            <Trophy className="h-3 w-3 text-yellow-500" />
                            {sweep.best_value.toFixed(4)}
                          </div>
                        ) : (
                          <span className="text-muted-foreground text-sm">
                            -
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground text-sm">
                        {formatRelativeTime(sweep.started_at)}
                      </TableCell>
                      <TableCell>
                        <a href={sweepHref(sweep.id)}>
                          <ArrowRight className="h-4 w-4 text-muted-foreground hover:text-foreground" />
                        </a>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Card>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
