import { apiClient } from "./client";
import type { Run } from "./types";

export interface Project {
  slug: string;
  name: string;
  run_count: number;
  last_updated: string;
  source: "local" | "remote";
  team_slug?: string;
}

export async function getProjects(): Promise<Project[]> {
  const response = await apiClient.get<{ projects: Project[] }>("/projects");
  return response.data.projects || [];
}

export async function getProjectRuns(
  projectSlug: string,
  opts?: { source?: string; team?: string },
): Promise<Run[]> {
  const params: Record<string, string> = {};
  if (opts?.source === "remote" && opts.team) {
    params.source = "remote";
    params.team = opts.team;
  }
  const response = await apiClient.get<Run[]>(`/projects/${projectSlug}/runs`, {
    params,
  });
  return response.data;
}

export async function getLocalRun(runId: string): Promise<Run> {
  const response = await apiClient.get<Run>(`/runs/${runId}`);
  return response.data;
}
