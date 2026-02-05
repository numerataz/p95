import { apiClient } from "./client";
import type { Run } from "./types";

export interface Project {
  slug: string;
  name: string;
  run_count: number;
  last_updated: string;
}

export async function getProjects(): Promise<Project[]> {
  const response = await apiClient.get<{ projects: Project[] }>("/projects");
  return response.data.projects || [];
}

export async function getProjectRuns(projectSlug: string): Promise<Run[]> {
  const response = await apiClient.get<Run[]>(`/projects/${projectSlug}/runs`);
  return response.data;
}

export async function getLocalRun(runId: string): Promise<Run> {
  const response = await apiClient.get<Run>(`/runs/${runId}`);
  return response.data;
}
