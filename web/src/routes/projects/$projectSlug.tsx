import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useState, useMemo } from "react";
import { getProjectRuns } from "@/api/projects";
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
import { ChevronLeft, Activity, GitCompare, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ComparisonChart } from "@/components/metrics/comparison-chart";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import type { XAxisMode, YAxisScale } from "@/components/metrics/metric-chart";

export const Route = createFileRoute("/projects/$projectSlug")({
  component: ProjectRunsPage,
});

function ProjectRunsPage() {
  // Get projectSlug from URL
  const projectSlug =
    window.location.pathname.split("/projects/")[1]?.split("/")[0] || "";

  const [selectedRunIds, setSelectedRunIds] = useState<Set<string>>(new Set());
  const [showComparison, setShowComparison] = useState(false);
  const [xAxisMode, setXAxisMode] = useState<XAxisMode>("step");
  const [yAxisScale, setYAxisScale] = useState<YAxisScale>("linear");
  const [selectedMetric, setSelectedMetric] = useState<string | null>(null);

  const { data: runs, isLoading } = useQuery({
    queryKey: ["project-runs", projectSlug],
    queryFn: () => getProjectRuns(projectSlug),
    refetchInterval: 3000, // Poll for updates
  });

  // Get selected runs for comparison
  const selectedRuns = useMemo(() => {
    if (!runs) return [];
    return runs.filter((r) => selectedRunIds.has(r.id));
  }, [runs, selectedRunIds]);

  // Get metric names from the first selected run
  const firstSelectedRunId = selectedRuns[0]?.id;
  const { data: metricNames } = useQuery({
    queryKey: ["metrics", "names", firstSelectedRunId],
    queryFn: () => getMetricNames(firstSelectedRunId!),
    enabled: !!firstSelectedRunId && showComparison,
  });

  const toggleRunSelection = (runId: string) => {
    setSelectedRunIds((prev) => {
      const next = new Set(prev);
      if (next.has(runId)) {
        next.delete(runId);
      } else if (next.size < 5) {
        // Max 5 runs
        next.add(runId);
      }
      return next;
    });
  };

  const clearComparison = () => {
    setSelectedRunIds(new Set());
    setShowComparison(false);
    setSelectedMetric(null);
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  const activeMetric = selectedMetric || metricNames?.[0];

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
            </h1>
            <p className="text-muted-foreground">
              {runs?.length || 0} run{(runs?.length || 0) !== 1 ? "s" : ""}
            </p>
          </div>
          {/* Comparison controls */}
          <div className="flex items-center gap-2">
            {selectedRunIds.size > 0 && (
              <>
                <span className="text-sm text-muted-foreground">
                  {selectedRunIds.size} selected
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowComparison(!showComparison)}
                  className="gap-2"
                >
                  <GitCompare className="h-4 w-4" />
                  {showComparison ? "Hide" : "Compare"}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={clearComparison}
                  className="gap-1"
                >
                  <X className="h-4 w-4" />
                  Clear
                </Button>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Comparison chart */}
      {showComparison && selectedRuns.length >= 2 && (
        <Card>
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between">
              <CardTitle className="text-lg">Run Comparison</CardTitle>
              <div className="flex items-center gap-4">
                {/* X-axis toggle */}
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
                {/* Y-axis toggle */}
                <div className="flex items-center gap-1">
                  <span className="text-xs text-muted-foreground">Y:</span>
                  <Button
                    variant={yAxisScale === "linear" ? "secondary" : "ghost"}
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
            {metricNames && metricNames.length > 0 ? (
              <Tabs
                value={activeMetric || metricNames[0]}
                onValueChange={setSelectedMetric}
              >
                <TabsList className="flex-wrap h-auto gap-1 mb-4">
                  {metricNames.map((name) => (
                    <TabsTrigger key={name} value={name} className="text-xs">
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
                      height={350}
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
              <div className="text-muted-foreground text-center py-8">
                No metrics available for comparison
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {showComparison && selectedRuns.length < 2 && (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            Select at least 2 runs to compare
          </CardContent>
        </Card>
      )}

      {/* Runs table */}
      {!runs || runs.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            No runs in this project yet
          </CardContent>
        </Card>
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-10">
                  <span className="sr-only">Compare</span>
                </TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Tags</TableHead>
                <TableHead>Started</TableHead>
                <TableHead>Duration</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {runs.map((run) => (
                <TableRow
                  key={run.id}
                  className={
                    selectedRunIds.has(run.id)
                      ? "bg-muted/50"
                      : "cursor-pointer"
                  }
                >
                  <TableCell>
                    <Checkbox
                      checked={selectedRunIds.has(run.id)}
                      onCheckedChange={() => toggleRunSelection(run.id)}
                      disabled={
                        !selectedRunIds.has(run.id) && selectedRunIds.size >= 5
                      }
                    />
                  </TableCell>
                  <TableCell>
                    <a
                      href={`/runs/${run.id}`}
                      className="font-medium hover:underline"
                    >
                      {run.name}
                    </a>
                    {run.git_info?.branch && (
                      <span className="ml-2 text-xs text-muted-foreground">
                        {run.git_info.branch}
                      </span>
                    )}
                  </TableCell>
                  <TableCell>
                    <RunStatusBadge status={run.status} />
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1 flex-wrap">
                      {run.tags?.slice(0, 3).map((tag) => (
                        <Badge key={tag} variant="outline" className="text-xs">
                          {tag}
                        </Badge>
                      ))}
                      {run.tags && run.tags.length > 3 && (
                        <Badge variant="outline" className="text-xs">
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
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
