import { apiClient } from "./client";
import type {
  Run,
  RunFilters,
  Continuation,
  ResumeRunRequest,
  ResumeRunResponse,
} from "./types";

export async function getRuns(
  teamSlug: string,
  appSlug: string,
  filters?: RunFilters,
): Promise<Run[]> {
  const response = await apiClient.get<Run[]>(
    `/teams/${teamSlug}/apps/${appSlug}/runs`,
    { params: filters },
  );
  return response.data;
}

export async function getRun(
  teamSlug: string,
  appSlug: string,
  runId: string,
  includeMetrics = false,
): Promise<Run> {
  const params = includeMetrics ? { include_metrics: "true" } : {};
  const response = await apiClient.get<Run>(
    `/teams/${teamSlug}/apps/${appSlug}/runs/${runId}`,
    { params },
  );
  return response.data;
}

export async function getRunById(runId: string): Promise<Run> {
  const response = await apiClient.get<Run>(`/runs/${runId}`);
  return response.data;
}

export async function updateRunStatus(
  runId: string,
  status: string,
  errorMessage?: string,
): Promise<Run> {
  const response = await apiClient.put<Run>(`/runs/${runId}/status`, {
    status,
    error_message: errorMessage,
  });
  return response.data;
}

export async function deleteRun(
  teamSlug: string,
  appSlug: string,
  runId: string,
): Promise<void> {
  await apiClient.delete(`/teams/${teamSlug}/apps/${appSlug}/runs/${runId}`);
}

export async function resumeRun(
  runId: string,
  request: ResumeRunRequest,
): Promise<ResumeRunResponse> {
  const response = await apiClient.post<ResumeRunResponse>(
    `/runs/${runId}/resume`,
    request,
  );
  return response.data;
}

export async function getContinuations(runId: string): Promise<Continuation[]> {
  const response = await apiClient.get<{ continuations: Continuation[] }>(
    `/runs/${runId}/continuations`,
  );
  return response.data.continuations;
}
