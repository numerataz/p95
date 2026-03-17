import { apiClient } from "./client";
import type { MetricSeries, MetricsSummaryResponse } from "./types";

export async function getMetricNames(runId: string): Promise<string[]> {
  const response = await apiClient.get<{ metrics: string[] }>(
    `/runs/${runId}/metrics`,
  );
  return response.data.metrics;
}

export async function getLatestMetrics(
  runId: string,
): Promise<Record<string, number>> {
  const response = await apiClient.get<Record<string, number>>(
    `/runs/${runId}/metrics/latest`,
  );
  return response.data;
}

export async function getMetricsSummary(
  runId: string,
): Promise<MetricsSummaryResponse> {
  const response = await apiClient.get<MetricsSummaryResponse>(
    `/runs/${runId}/metrics/summary`,
  );
  return response.data;
}

interface SeriesOptions {
  since?: string;
  until?: string;
  minStep?: number;
  maxStep?: number;
  maxPoints?: number;
  limit?: number;
  offset?: number;
}

export async function getMetricSeries(
  runId: string,
  metricName: string,
  options?: SeriesOptions,
): Promise<MetricSeries> {
  const params: Record<string, string | number> = {};
  if (options?.since) params.since = options.since;
  if (options?.until) params.until = options.until;
  if (options?.minStep !== undefined) params.min_step = options.minStep;
  if (options?.maxStep !== undefined) params.max_step = options.maxStep;
  if (options?.maxPoints !== undefined) params.max_points = options.maxPoints;
  if (options?.limit !== undefined) params.limit = options.limit;
  if (options?.offset !== undefined) params.offset = options.offset;

  const response = await apiClient.get<MetricSeries>(
    `/runs/${runId}/metrics/${encodeURIComponent(metricName)}`,
    { params },
  );
  return response.data;
}
