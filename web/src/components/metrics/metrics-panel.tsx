import { useQuery } from "@tanstack/react-query";
import { getMetricNames } from "@/api/metrics";
import { getContinuations } from "@/api/runs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Slider } from "@/components/ui/slider";
import { MetricChart, XAxisMode, YAxisScale } from "./metric-chart";
import { useState } from "react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import type { Continuation } from "@/api/types";

interface MetricsPanelProps {
  runId: string;
  isRunning: boolean;
  continuations?: Continuation[];
}

export function MetricsPanel({
  runId,
  isRunning,
  continuations,
}: MetricsPanelProps) {
  const { data: metricNames, isLoading } = useQuery({
    queryKey: ["metrics", "names", runId],
    queryFn: () => getMetricNames(runId),
    refetchInterval: isRunning ? 2000 : false,
  });

  // Fetch continuations - always fetch and use provided continuations if available
  const { data: fetchedContinuations = [] } = useQuery({
    queryKey: ["continuations", runId],
    queryFn: () => getContinuations(runId),
    refetchInterval: isRunning ? 5000 : false, // Poll while running
    retry: false,
  });

  // Use provided continuations if they have data, otherwise use fetched
  const activeContinuations =
    continuations && continuations.length > 0
      ? continuations
      : fetchedContinuations;

  const [selectedMetric, setSelectedMetric] = useState<string | null>(null);
  const [smoothing, setSmoothing] = useState(0);
  const [xAxisMode, setXAxisMode] = useState<XAxisMode>("step");
  const [yAxisScale, setYAxisScale] = useState<YAxisScale>("linear");

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (!metricNames || metricNames.length === 0) {
    return (
      <Card>
        <CardContent className="py-8 text-center text-muted-foreground">
          No metrics logged yet
        </CardContent>
      </Card>
    );
  }

  const activeMetric = selectedMetric || metricNames[0];

  return (
    <div className="space-y-4">
      <Tabs value={activeMetric} onValueChange={setSelectedMetric}>
        <TabsList className="flex-wrap h-auto gap-1">
          {metricNames.map((name) => (
            <TabsTrigger key={name} value={name} className="text-xs">
              {name}
            </TabsTrigger>
          ))}
        </TabsList>

        {metricNames.map((name) => (
          <TabsContent key={name} value={name}>
            <Card>
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-lg">{name}</CardTitle>
                  <div className="flex items-center gap-4">
                    {isRunning && (
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
                        </span>
                        Live
                      </div>
                    )}
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
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground">
                        Smoothing
                      </span>
                      <Slider
                        value={[smoothing]}
                        onValueChange={([value]) => setSmoothing(value)}
                        min={0}
                        max={0.99}
                        step={0.01}
                        className="w-24"
                      />
                      <span className="text-xs text-muted-foreground w-7">
                        {smoothing.toFixed(2)}
                      </span>
                    </div>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <MetricChart
                  runId={runId}
                  metricName={name}
                  isRunning={isRunning}
                  smoothing={smoothing}
                  xAxisMode={xAxisMode}
                  yAxisScale={yAxisScale}
                  continuations={activeContinuations}
                />
              </CardContent>
            </Card>
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}
