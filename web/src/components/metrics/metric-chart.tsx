import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
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
  ReferenceLine,
} from "recharts";
import { formatMetricValue, formatNumber } from "@/lib/utils";
import type { Continuation } from "@/api/types";

export type XAxisMode = "step" | "relativeTime";
export type YAxisScale = "linear" | "log";

interface MetricChartProps {
  runId: string;
  metricName: string;
  isRunning: boolean;
  height?: number;
  smoothing?: number;
  xAxisMode?: XAxisMode;
  yAxisScale?: YAxisScale;
  continuations?: Continuation[];
}

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

// Exponential Moving Average smoothing
function smoothData(
  data: Array<{ step: number; value: number }>,
  smoothingFactor: number,
): Array<{ step: number; value: number; smoothed: number }> {
  if (smoothingFactor === 0 || data.length === 0) {
    return data.map((d) => ({ ...d, smoothed: d.value }));
  }

  const weight = 1 - smoothingFactor;
  const result: Array<{ step: number; value: number; smoothed: number }> = [];
  let smoothedValue = data[0].value;

  for (const point of data) {
    smoothedValue = weight * point.value + (1 - weight) * smoothedValue;
    result.push({
      step: point.step,
      value: point.value,
      smoothed: smoothedValue,
    });
  }

  return result;
}

export function MetricChart({
  runId,
  metricName,
  isRunning,
  height = 300,
  smoothing = 0,
  xAxisMode = "step",
  yAxisScale = "linear",
  continuations = [],
}: MetricChartProps) {
  const { data: series, isLoading } = useQuery({
    queryKey: ["metrics", "series", runId, metricName],
    queryFn: () => getMetricSeries(runId, metricName, { maxPoints: 1000 }),
    refetchInterval: isRunning ? 5000 : false,
  });

  const chartData = useMemo(() => {
    if (!series || !series.points || series.points.length === 0) {
      return [];
    }

    // Find first timestamp for relative time calculation
    const firstTimestamp = series.points[0].time
      ? new Date(series.points[0].time).getTime()
      : 0;

    const rawData = series.points.map((p) => {
      let xValue: number;
      if (xAxisMode === "relativeTime" && p.time) {
        // Convert to seconds since first data point
        xValue = (new Date(p.time).getTime() - firstTimestamp) / 1000;
      } else {
        xValue = p.step;
      }

      // Apply log scale to value if needed
      let yValue = p.value;
      if (yAxisScale === "log" && yValue > 0) {
        yValue = Math.log10(yValue);
      }

      return {
        step: p.step,
        xValue,
        value: p.value,
        displayValue: yValue,
      };
    });

    return smoothData(
      rawData.map((d) => ({ step: d.step, value: d.value })),
      smoothing,
    ).map((d, i) => ({
      ...d,
      xValue: rawData[i].xValue,
      displayValue:
        yAxisScale === "log" && d.smoothed > 0
          ? Math.log10(d.smoothed)
          : d.smoothed,
      originalDisplayValue:
        yAxisScale === "log" && d.value > 0 ? Math.log10(d.value) : d.value,
    }));
  }, [series, smoothing, xAxisMode, yAxisScale]);

  // Format functions based on axis settings
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

  if (!series || !series.points || series.points.length === 0) {
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
      // Convert from log scale back to actual value for display
      const actualValue = Math.pow(10, v);
      return formatMetricValue(actualValue);
    }
    return formatMetricValue(v);
  };

  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={chartData}>
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
          scale={yAxisScale === "log" ? "linear" : "linear"}
        />
        <Tooltip
          content={({ active, payload }) => {
            if (!active || !payload || !payload.length) return null;
            const data = payload[0].payload;
            return (
              <div className="bg-popover border rounded-md shadow-md p-2 text-sm">
                <div className="font-medium">
                  Step {formatNumber(data.step)}
                  {xAxisMode === "relativeTime" && (
                    <span className="text-muted-foreground ml-2">
                      ({formatXAxis(data.xValue)})
                    </span>
                  )}
                </div>
                <div className="text-muted-foreground">
                  Value: {formatMetricValue(data.value)}
                </div>
                {smoothing > 0 && (
                  <div className="text-primary">
                    Smoothed: {formatMetricValue(data.smoothed)}
                  </div>
                )}
              </div>
            );
          }}
        />
        {/* Show original data as faded line when smoothing is applied */}
        {smoothing > 0 && (
          <Line
            type="monotone"
            dataKey="originalDisplayValue"
            stroke="hsl(var(--muted-foreground))"
            strokeWidth={1}
            strokeOpacity={0.3}
            dot={false}
            isAnimationActive={false}
          />
        )}
        <Line
          type="monotone"
          dataKey={smoothing > 0 ? "displayValue" : "originalDisplayValue"}
          stroke="hsl(var(--primary))"
          strokeWidth={2}
          dot={false}
          isAnimationActive={false}
        />
        {/* Continuation markers (Grafana-style deploy markers) */}
        {continuations.map((cont) => {
          // Find the x value for this continuation step
          // If xAxisMode is "step", use the step directly
          // If xAxisMode is "relativeTime", find the closest data point's xValue
          let xValue = cont.step;
          if (xAxisMode === "relativeTime" && chartData.length > 0) {
            // Find the data point with this step or closest to it
            const dataPoint = chartData.find((d) => d.step === cont.step);
            if (dataPoint) {
              xValue = dataPoint.xValue;
            } else {
              // Interpolate if exact step not found
              const before = chartData.filter((d) => d.step < cont.step).pop();
              const after = chartData.find((d) => d.step > cont.step);
              if (before && after) {
                const ratio =
                  (cont.step - before.step) / (after.step - before.step);
                xValue = before.xValue + ratio * (after.xValue - before.xValue);
              } else if (before) {
                xValue = before.xValue;
              } else if (after) {
                xValue = after.xValue;
              }
            }
          }
          return (
            <ReferenceLine
              key={cont.id}
              x={xValue}
              stroke="#888"
              strokeDasharray="5 5"
              strokeWidth={2}
            />
          );
        })}
      </LineChart>
    </ResponsiveContainer>
  );
}
