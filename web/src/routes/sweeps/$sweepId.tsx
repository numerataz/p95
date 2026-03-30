import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { getSweep, getSweepRuns, stopSweep, type SweepContext } from "@/api/sweeps";
import { getMetricNames } from "@/api/metrics";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatRelativeTime, formatDuration } from "@/lib/utils";
import { RunStatusBadge } from "@/components/runs/run-status-badge";
import { ComparisonChart } from "@/components/metrics/comparison-chart";
import type { XAxisMode, YAxisScale } from "@/components/metrics/metric-chart";
import {
  ChevronLeft,
  ChevronRight,
  Shuffle,
  Grid3X3,
  Square,
  Clock,
  TrendingUp,
  TrendingDown,
  Trophy,
} from "lucide-react";
import type { SweepStatus } from "@/api/types";
import {
  ScatterChart,
  Scatter,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

export const Route = createFileRoute("/sweeps/$sweepId")({
  component: SweepDetailPage,
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

const RUNS_PER_PAGE = 10;

function SweepDetailPage() {
  const { sweepId } = Route.useParams();
  const queryClient = useQueryClient();

  // Read source context from URL search params (set when linking from project page)
  const searchParams = new URLSearchParams(window.location.search);
  const sweepCtx: SweepContext = {
    source: searchParams.get("source") || undefined,
    team: searchParams.get("team") || undefined,
    project: searchParams.get("project") || undefined,
  };

  const [runsPage, setRunsPage] = useState(0);
  const [selectedParam, setSelectedParam] = useState<string | null>(null);
  const [xAxisMode, setXAxisMode] = useState<XAxisMode>("step");
  const [yAxisScale, setYAxisScale] = useState<YAxisScale>("linear");
  const [selectedMetric, setSelectedMetric] = useState<string | null>(null);

  const { data: sweep, isLoading: sweepLoading } = useQuery({
    queryKey: ["sweep", sweepId, sweepCtx.source, sweepCtx.team],
    queryFn: () => getSweep(sweepId, sweepCtx),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "running" ? 3000 : false;
    },
  });

  const { data: runs = [] } = useQuery({
    queryKey: ["sweep", sweepId, "runs", sweepCtx.source, sweepCtx.team],
    queryFn: () => getSweepRuns(sweepId, sweepCtx),
    refetchInterval: sweep?.status === "running" ? 3000 : false,
  });

  const firstRunId = runs[0]?.id;
  const { data: metricNames = [] } = useQuery({
    queryKey: ["metrics", "names", firstRunId],
    queryFn: () => getMetricNames(firstRunId!),
    enabled: !!firstRunId,
    refetchInterval: sweep?.status === "running" ? 5000 : false,
  });

  const stopMutation = useMutation({
    mutationFn: () => stopSweep(sweepId, sweepCtx),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sweep", sweepId] });
    },
  });

  const parameterNames = useMemo(() => {
    return sweep?.search_space.parameters.map((p) => p.name) || [];
  }, [sweep?.search_space]);

  // Build scatter data: param value vs final metric value from run config
  const parameterScatterData = useMemo(() => {
    if (!selectedParam || !sweep) return [];
    return runs
      .filter((run) => run.config && run.config[selectedParam] !== undefined)
      .map((run) => {
        const paramValue = run.config![selectedParam];
        const metricValue = run.latest_metrics?.[sweep.metric_name];
        return {
          name: run.name,
          param:
            typeof paramValue === "number"
              ? paramValue
              : String(paramValue),
          metric: metricValue,
          isBest: run.id === sweep.best_run_id,
        };
      })
      .filter((d) => d.metric !== undefined);
  }, [selectedParam, runs, sweep]);

  if (sweepLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (!sweep) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        Sweep not found
      </div>
    );
  }

  const backHref = window.history.length > 1 ? undefined : "/projects";

  return (
    <div className="space-y-6">
      {/* Back button */}
      <a href={backHref} onClick={backHref ? undefined : (e) => { e.preventDefault(); window.history.back(); }}>
        <Button variant="ghost" size="sm" className="gap-1 -ml-2">
          <ChevronLeft className="h-4 w-4" />
          Back
        </Button>
      </a>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">{sweep.name}</h1>
            <SweepStatusBadge status={sweep.status} />
          </div>
          <div className="flex items-center gap-4 mt-2 text-sm text-muted-foreground">
            <span className="flex items-center gap-1">
              {sweep.method === "random" ? (
                <Shuffle className="h-3 w-3" />
              ) : (
                <Grid3X3 className="h-3 w-3" />
              )}
              {sweep.method} search
            </span>
            <span className="flex items-center gap-1">
              {sweep.metric_goal === "minimize" ? (
                <TrendingDown className="h-3 w-3" />
              ) : (
                <TrendingUp className="h-3 w-3" />
              )}
              {sweep.metric_goal} {sweep.metric_name}
            </span>
            <span>
              {sweep.run_count}
              {sweep.max_runs && `/${sweep.max_runs}`} runs
            </span>
            <span>Started {formatRelativeTime(sweep.started_at)}</span>
          </div>
        </div>
        {sweep.status === "running" && (
          <Button
            variant="outline"
            onClick={() => stopMutation.mutate()}
            disabled={stopMutation.isPending}
            className="gap-1"
          >
            <Square className="h-3 w-3" />
            Stop Sweep
          </Button>
        )}
      </div>

      {/* Best Run Card */}
      {sweep.best_run_id && sweep.best_value !== undefined && sweep.best_value !== null && (
        <Card className="border-yellow-500/50 bg-yellow-500/5">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center gap-1">
              <Trophy className="h-4 w-4 text-yellow-500" />
              Best Run
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between">
              <div>
                <a
                  href={`/runs/${sweep.best_run_id}`}
                  className="font-medium hover:underline"
                >
                  {runs.find((r) => r.id === sweep.best_run_id)?.name ||
                    sweep.best_run_id}
                </a>
                <p className="text-sm text-muted-foreground mt-1">
                  {sweep.metric_name}: {sweep.best_value.toFixed(6)}
                </p>
              </div>
              {runs.find((r) => r.id === sweep.best_run_id)?.config && (
                <div className="text-sm text-right">
                  {Object.entries(
                    runs.find((r) => r.id === sweep.best_run_id)!.config!,
                  )
                    .slice(0, 3)
                    .map(([key, value]) => (
                      <div key={key} className="text-muted-foreground">
                        {key}:{" "}
                        <span className="font-mono">
                          {typeof value === "number"
                            ? (value as number).toPrecision(4)
                            : String(value)}
                        </span>
                      </div>
                    ))}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Summary Stats */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Runs
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {sweep.run_count}
              {sweep.max_runs && (
                <span className="text-lg text-muted-foreground font-normal">
                  /{sweep.max_runs}
                </span>
              )}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Best {sweep.metric_name}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {sweep.best_value !== undefined && sweep.best_value !== null
                ? sweep.best_value.toFixed(4)
                : "-"}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Parameters
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {sweep.search_space.parameters.length}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Duration
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold flex items-center gap-1">
              <Clock className="h-5 w-5 text-muted-foreground" />
              {sweep.ended_at
                ? formatDuration(
                    (new Date(sweep.ended_at).getTime() -
                      new Date(sweep.started_at).getTime()) /
                      1000,
                  )
                : "Running..."}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="runs" className="space-y-4">
        <TabsList>
          <TabsTrigger value="runs">Runs</TabsTrigger>
          <TabsTrigger value="metrics">Metrics</TabsTrigger>
          <TabsTrigger value="parameters">Parameters</TabsTrigger>
          <TabsTrigger value="config">Config</TabsTrigger>
        </TabsList>

        {/* Runs Tab */}
        <TabsContent value="runs">
          <Card>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead className="w-20">Status</TableHead>
                  <TableHead>Parameters</TableHead>
                  <TableHead className="w-32">{sweep.metric_name}</TableHead>
                  <TableHead className="w-24">Duration</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {runs
                  .slice(
                    runsPage * RUNS_PER_PAGE,
                    (runsPage + 1) * RUNS_PER_PAGE,
                  )
                  .map((run) => {
                    const metricValue =
                      run.latest_metrics?.[sweep.metric_name];
                    const isBest = run.id === sweep.best_run_id;

                    return (
                      <TableRow
                        key={run.id}
                        className={isBest ? "bg-yellow-500/5" : ""}
                      >
                        <TableCell>
                          <a
                            href={`/runs/${run.id}`}
                            className="font-medium hover:underline"
                          >
                            {run.name}
                          </a>
                        </TableCell>
                        <TableCell>
                          <RunStatusBadge status={run.status} />
                        </TableCell>
                        <TableCell>
                          {run.config && (
                            <div className="flex flex-wrap gap-1">
                              {Object.entries(run.config)
                                .slice(0, 4)
                                .map(([key, value]) => (
                                  <Badge
                                    key={key}
                                    variant="outline"
                                    className="text-xs"
                                  >
                                    {key}=
                                    {typeof value === "number"
                                      ? (value as number).toPrecision(3)
                                      : String(value)}
                                  </Badge>
                                ))}
                            </div>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-sm">
                          {metricValue !== undefined
                            ? metricValue.toFixed(4)
                            : "-"}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {run.duration_seconds !== undefined
                            ? formatDuration(run.duration_seconds)
                            : run.status === "running"
                              ? "Running..."
                              : "-"}
                        </TableCell>
                      </TableRow>
                    );
                  })}
              </TableBody>
            </Table>
            {runs.length > RUNS_PER_PAGE && (
              <div className="flex items-center justify-between px-4 py-3 border-t">
                <span className="text-sm text-muted-foreground">
                  Showing {runsPage * RUNS_PER_PAGE + 1}-
                  {Math.min((runsPage + 1) * RUNS_PER_PAGE, runs.length)} of{" "}
                  {runs.length} runs
                </span>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setRunsPage((p) => Math.max(0, p - 1))}
                    disabled={runsPage === 0}
                  >
                    <ChevronLeft className="h-4 w-4" />
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setRunsPage((p) => p + 1)}
                    disabled={(runsPage + 1) * RUNS_PER_PAGE >= runs.length}
                  >
                    Next
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            )}
          </Card>
        </TabsContent>

        {/* Metrics Tab */}
        <TabsContent value="metrics">
          <Card className="h-full">
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-lg">Run Comparison</CardTitle>
                <div className="flex items-center gap-4">
                  <div className="flex items-center gap-1">
                    <span className="text-xs text-muted-foreground">X:</span>
                    <Button
                      variant={xAxisMode === "step" ? "secondary" : "ghost"}
                      size="sm"
                      className="h-6 px-2 text-xs"
                      onClick={() => setXAxisMode("step")}
                    >
                      Step
                    </Button>
                    <Button
                      variant={
                        xAxisMode === "relativeTime" ? "secondary" : "ghost"
                      }
                      size="sm"
                      className="h-6 px-2 text-xs"
                      onClick={() => setXAxisMode("relativeTime")}
                    >
                      Time
                    </Button>
                  </div>
                  <div className="flex items-center gap-1">
                    <span className="text-xs text-muted-foreground">Y:</span>
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
                      variant={yAxisScale === "log" ? "secondary" : "ghost"}
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
              {runs.length === 0 ? (
                <div className="text-muted-foreground text-center py-12">
                  No runs in this sweep yet
                </div>
              ) : metricNames.length > 0 ? (
                <Tabs
                  value={selectedMetric || metricNames[0]}
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
                        runs={runs.map((r) => ({ id: r.id, name: r.name }))}
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
                  No metrics available yet
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Parameters Tab */}
        <TabsContent value="parameters">
          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <span>Parameter vs {sweep.metric_name}</span>
                  <div className="flex gap-1">
                    {parameterNames.map((name) => (
                      <Button
                        key={name}
                        variant={selectedParam === name ? "secondary" : "ghost"}
                        size="sm"
                        onClick={() => setSelectedParam(name)}
                      >
                        {name}
                      </Button>
                    ))}
                  </div>
                </CardTitle>
              </CardHeader>
              <CardContent>
                {selectedParam && parameterScatterData.length > 0 ? (
                  <ResponsiveContainer width="100%" height={400}>
                    <ScatterChart>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis
                        dataKey="param"
                        name={selectedParam}
                        type={
                          typeof parameterScatterData[0]?.param === "number"
                            ? "number"
                            : "category"
                        }
                      />
                      <YAxis dataKey="metric" name={sweep.metric_name} />
                      <Tooltip cursor={{ strokeDasharray: "3 3" }} />
                      <Scatter
                        data={parameterScatterData}
                        fill="#2563eb"
                        shape={(props: {
                          cx: number;
                          cy: number;
                          payload: { isBest: boolean };
                        }) => {
                          const { cx, cy, payload } = props;
                          if (payload.isBest) {
                            return (
                              <g>
                                <circle cx={cx} cy={cy} r={8} fill="#eab308" />
                                <circle cx={cx} cy={cy} r={4} fill="#ca8a04" />
                              </g>
                            );
                          }
                          return <circle cx={cx} cy={cy} r={6} fill="#2563eb" />;
                        }}
                      />
                    </ScatterChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="text-center py-12 text-muted-foreground">
                    {selectedParam
                      ? "No data available for this parameter"
                      : "Select a parameter to visualize"}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Search Space */}
            <Card>
              <CardHeader>
                <CardTitle>Search Space</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Parameter</TableHead>
                      <TableHead>Type</TableHead>
                      <TableHead>Range / Values</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sweep.search_space.parameters.map((param) => (
                      <TableRow key={param.name}>
                        <TableCell className="font-medium">
                          {param.name}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline">{param.type}</Badge>
                        </TableCell>
                        <TableCell className="font-mono text-sm">
                          {param.type === "categorical"
                            ? param.values
                                ?.map((v) => String(v))
                                .join(", ")
                            : `[${param.min}, ${param.max}]`}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* Config Tab */}
        <TabsContent value="config">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Sweep Configuration</CardTitle>
              </CardHeader>
              <CardContent>
                <dl className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <dt className="text-muted-foreground">Method</dt>
                    <dd className="font-medium">{sweep.method}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-muted-foreground">Metric</dt>
                    <dd className="font-medium">{sweep.metric_name}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-muted-foreground">Goal</dt>
                    <dd className="font-medium">{sweep.metric_goal}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-muted-foreground">Max Runs</dt>
                    <dd className="font-medium">
                      {sweep.max_runs || "Unlimited"}
                    </dd>
                  </div>
                </dl>
              </CardContent>
            </Card>

            {sweep.early_stopping && (
              <Card>
                <CardHeader>
                  <CardTitle>Early Stopping</CardTitle>
                </CardHeader>
                <CardContent>
                  <dl className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <dt className="text-muted-foreground">Method</dt>
                      <dd className="font-medium">
                        {sweep.early_stopping.method}
                      </dd>
                    </div>
                    <div className="flex justify-between">
                      <dt className="text-muted-foreground">Min Steps</dt>
                      <dd className="font-medium">
                        {sweep.early_stopping.min_steps}
                      </dd>
                    </div>
                    <div className="flex justify-between">
                      <dt className="text-muted-foreground">Warmup Runs</dt>
                      <dd className="font-medium">
                        {sweep.early_stopping.warmup}
                      </dd>
                    </div>
                  </dl>
                </CardContent>
              </Card>
            )}

            {sweep.config && Object.keys(sweep.config).length > 0 && (
              <Card className="md:col-span-2">
                <CardHeader>
                  <CardTitle>Static Config</CardTitle>
                </CardHeader>
                <CardContent>
                  <pre className="bg-muted p-4 rounded-md overflow-auto text-sm">
                    {JSON.stringify(sweep.config, null, 2)}
                  </pre>
                </CardContent>
              </Card>
            )}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
