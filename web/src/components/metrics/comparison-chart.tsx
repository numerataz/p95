import { useQueries } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { getMetricSeries } from "@/api/metrics";
import { Skeleton } from "@/components/ui/skeleton";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  ReferenceLine,
} from "recharts";
import { formatMetricValue, formatNumber } from "@/lib/utils";
import type { XAxisMode, YAxisScale } from "./metric-chart";

// Generate tick values that produce unique formatted labels
function generateUniqueTicks(
  min: number,
  max: number,
  formatter: (v: number) => string,
  maxTicks: number = 10,
): number[] {
  if (min === max) return [min];

  const range = max - min;
  const step = range / maxTicks;
  const ticks: number[] = [];
  const seenLabels = new Set<string>();

  for (let i = 0; i <= maxTicks; i++) {
    const value = min + i * step;
    const label = formatter(value);
    if (!seenLabels.has(label)) {
      seenLabels.add(label);
      ticks.push(value);
    }
  }

  return ticks;
}

// Color palette for comparison series
const SERIES_COLORS = [
  "hsl(25, 95%, 53%)", // Orange (primary)
  "hsl(142, 71%, 45%)", // Green
  "hsl(217, 91%, 60%)", // Blue
  "hsl(292, 84%, 61%)", // Magenta
  "hsl(48, 96%, 53%)", // Yellow
];

interface ComparedRun {
  id: string;
  name: string;
}

interface ComparisonChartProps {
  runs: ComparedRun[];
  metricName: string;
  height?: number;
  xAxisMode?: XAxisMode;
  yAxisScale?: YAxisScale;
}

export function ComparisonChart({
  runs,
  metricName,
  height = 300,
  xAxisMode = "step",
  yAxisScale = "linear",
}: ComparisonChartProps) {
  const [highlightedRunId, setHighlightedRunId] = useState<string | null>(null);
  const [hoverY, setHoverY] = useState<number | null>(null);

  // Fetch metric data for all runs (poll every 5s for live updates)
  const queries = useQueries({
    queries: runs.map((run) => ({
      queryKey: ["metrics", "series", run.id, metricName],
      queryFn: () => getMetricSeries(run.id, metricName, { maxPoints: 1000 }),
      staleTime: 5000,
      refetchInterval: 5000,
    })),
  });

  const isLoading = queries.some((q) => q.isLoading);

  const { chartData, runDataKeys } = useMemo(() => {
    // Build a map of step/time -> values for all runs
    const dataMap = new Map<
      number,
      { xValue: number; step: number; [key: string]: number }
    >();
    const runDataKeys: { runId: string; runName: string; color: string }[] = [];

    // Find the earliest timestamp across all runs for relative time
    let minTimestamp = Infinity;
    queries.forEach((query) => {
      if (query.data?.points) {
        query.data.points.forEach((p) => {
          if (p.time) {
            const ts = new Date(p.time).getTime();
            if (ts < minTimestamp) minTimestamp = ts;
          }
        });
      }
    });

    queries.forEach((query, index) => {
      if (!query.data?.points) return;

      const run = runs[index];
      const dataKey = `run_${run.id}`;
      runDataKeys.push({
        runId: run.id,
        runName: run.name,
        color: SERIES_COLORS[index % SERIES_COLORS.length],
      });

      query.data.points.forEach((p) => {
        let xValue: number;
        if (xAxisMode === "relativeTime" && p.time) {
          xValue = (new Date(p.time).getTime() - minTimestamp) / 1000;
        } else {
          xValue = p.step;
        }

        let value = p.value;
        if (yAxisScale === "log" && value > 0) {
          value = Math.log10(value);
        }

        const key = xAxisMode === "relativeTime" ? Math.round(xValue) : p.step;
        if (!dataMap.has(key)) {
          dataMap.set(key, { xValue, step: p.step });
        }
        const entry = dataMap.get(key)!;
        entry[dataKey] = value;
        // Store original value for tooltip
        entry[`${dataKey}_original`] = p.value;
      });
    });

    // Sort by x value
    const sortedData = Array.from(dataMap.values()).sort(
      (a, b) => a.xValue - b.xValue,
    );

    return { chartData: sortedData, runDataKeys };
  }, [queries, runs, xAxisMode, yAxisScale]);

  // Format function for X axis
  const formatXAxis = (v: number) => {
    if (xAxisMode === "relativeTime") {
      if (v < 60) return `${v.toFixed(0)}s`;
      if (v < 3600) return `${(v / 60).toFixed(1)}m`;
      return `${(v / 3600).toFixed(1)}h`;
    }
    return formatNumber(v);
  };

  // Compute unique ticks to avoid duplicate labels
  const xAxisTicks = useMemo(() => {
    if (chartData.length === 0) return undefined;
    const xValues = chartData.map((d) => d.xValue);
    const min = Math.min(...xValues);
    const max = Math.max(...xValues);
    return generateUniqueTicks(min, max, formatXAxis, 10);
  }, [chartData, xAxisMode]);

  if (isLoading) {
    return <Skeleton style={{ height }} className="w-full" />;
  }

  if (chartData.length === 0) {
    return (
      <div
        style={{ height }}
        className="w-full flex items-center justify-center text-muted-foreground"
      >
        No data points
      </div>
    );
  }

  const formatYAxis = (v: number) => {
    if (yAxisScale === "log") {
      const actualValue = Math.pow(10, v);
      return formatMetricValue(actualValue);
    }
    return formatMetricValue(v);
  };

  // Reorder runs so highlighted run is drawn last (on top)
  const orderedRunDataKeys = highlightedRunId
    ? [
        ...runDataKeys.filter((r) => r.runId !== highlightedRunId),
        ...runDataKeys.filter((r) => r.runId === highlightedRunId),
      ]
    : runDataKeys;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const handleLegendClick = (data: any) => {
    if (!data?.dataKey || typeof data.dataKey !== "string") return;
    const runId = data.dataKey.replace("run_", "");
    setHighlightedRunId((prev) => (prev === runId ? null : runId));
  };

  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart
        data={chartData}
        onMouseMove={(e) => {
          if (e?.activePayload?.[0]?.value !== undefined) {
            setHoverY(e.activePayload[0].value);
          }
        }}
        onMouseLeave={() => setHoverY(null)}
      >
        <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
        <XAxis
          dataKey="xValue"
          tick={{ fontSize: 12 }}
          tickFormatter={formatXAxis}
          className="text-muted-foreground"
          ticks={xAxisTicks}
          type="number"
          domain={["dataMin", "dataMax"]}
        />
        <YAxis
          tick={{ fontSize: 12 }}
          tickFormatter={formatYAxis}
          className="text-muted-foreground"
          domain={["auto", "auto"]}
        />
        <Tooltip
          cursor={{ stroke: "hsl(var(--muted-foreground))", strokeWidth: 1, strokeDasharray: "4 4" }}
          content={({ active, payload, label }) => {
            if (!active || !payload || !payload.length) return null;
            return (
              <div className="bg-popover border rounded-md shadow-md p-2 text-sm">
                <div className="font-medium mb-1">
                  {xAxisMode === "relativeTime"
                    ? formatXAxis(label as number)
                    : `Step ${formatNumber(label as number)}`}
                </div>
                {payload.map((entry) => {
                  const runKey = runDataKeys.find(
                    (r) => `run_${r.runId}` === entry.dataKey,
                  );
                  if (!runKey) return null;
                  const originalValue =
                    entry.payload[`run_${runKey.runId}_original`];
                  return (
                    <div
                      key={entry.dataKey}
                      className="flex items-center gap-2"
                    >
                      <span
                        className="w-2 h-2 rounded-full"
                        style={{ backgroundColor: runKey.color }}
                      />
                      <span className="truncate max-w-[120px]">
                        {runKey.runName}:
                      </span>
                      <span>{formatMetricValue(originalValue)}</span>
                    </div>
                  );
                })}
              </div>
            );
          }}
        />
        <Legend
          onClick={handleLegendClick}
          wrapperStyle={{ cursor: "pointer", fontSize: "9px", lineHeight: "1.2" }}
          iconSize={8}
          formatter={(value, entry) => {
            const runKey = runDataKeys.find(
              (r) => `run_${r.runId}` === entry.dataKey,
            );
            if (!runKey) return value;
            const isHighlighted = highlightedRunId === runKey.runId;
            return (
              <span
                style={{
                  fontWeight: isHighlighted ? "600" : "normal",
                  textDecoration: isHighlighted ? "underline" : "none",
                  fontSize: "9px",
                }}
              >
                {runKey.runName}
              </span>
            );
          }}
        />
        {hoverY !== null && (
          <ReferenceLine
            y={hoverY}
            stroke="hsl(var(--muted-foreground))"
            strokeWidth={1}
            strokeDasharray="4 4"
          />
        )}
        {orderedRunDataKeys.map((runKey) => (
          <Line
            key={runKey.runId}
            type="monotone"
            dataKey={`run_${runKey.runId}`}
            name={runKey.runName}
            stroke={runKey.color}
            strokeWidth={highlightedRunId === runKey.runId ? 3 : 2}
            strokeOpacity={
              highlightedRunId && highlightedRunId !== runKey.runId ? 0.4 : 1
            }
            dot={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}
