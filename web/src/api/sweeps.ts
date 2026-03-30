import { apiClient } from "./client";
import type { Sweep, SweepFilters, Run } from "./types";

export async function getProjectSweeps(
  projectSlug: string,
  opts?: { source?: string; team?: string },
  filters?: SweepFilters,
): Promise<Sweep[]> {
  const params: Record<string, string | number> = {};
  if (opts?.source === "remote" && opts.team) {
    params.source = "remote";
    params.team = opts.team;
  }
  if (filters?.limit) params.limit = filters.limit;
  if (filters?.order_by) params.order_by = filters.order_by;
  if (filters?.order_dir) params.order_dir = filters.order_dir;

  const response = await apiClient.get<Sweep[]>(
    `/projects/${projectSlug}/sweeps`,
    { params },
  );
  return response.data;
}

export interface SweepContext {
  source?: string;
  team?: string;
  project?: string;
}

export async function getSweep(sweepId: string, ctx?: SweepContext): Promise<Sweep> {
  const params: Record<string, string> = {};
  if (ctx?.source === "remote" && ctx.team && ctx.project) {
    params.source = "remote";
    params.team = ctx.team;
    params.project = ctx.project;
  }
  const response = await apiClient.get<Sweep>(`/sweeps/${sweepId}`, { params });
  return response.data;
}

export async function getSweepRuns(sweepId: string, ctx?: SweepContext): Promise<Run[]> {
  const params: Record<string, string> = {};
  if (ctx?.source === "remote" && ctx.team && ctx.project) {
    params.source = "remote";
    params.team = ctx.team;
    params.project = ctx.project;
  }
  const response = await apiClient.get<{ runs: Run[] }>(
    `/sweeps/${sweepId}/runs`,
    { params },
  );
  return response.data.runs;
}

export async function stopSweep(sweepId: string, ctx?: SweepContext): Promise<Sweep> {
  const params: Record<string, string> = {};
  if (ctx?.source === "remote" && ctx.team && ctx.project) {
    params.source = "remote";
    params.team = ctx.team;
    params.project = ctx.project;
  }
  const response = await apiClient.post<Sweep>(`/sweeps/${sweepId}/stop`, undefined, { params });
  return response.data;
}
