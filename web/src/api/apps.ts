import { apiClient } from "./client";
import type { App, CreateAppRequest } from "./types";

export async function getApps(
  teamSlug: string,
  includeArchived = false,
): Promise<App[]> {
  const params = includeArchived ? { include_archived: "true" } : {};
  const response = await apiClient.get<App[]>(`/teams/${teamSlug}/apps`, {
    params,
  });
  return response.data;
}

export async function getApp(teamSlug: string, appSlug: string): Promise<App> {
  const response = await apiClient.get<App>(
    `/teams/${teamSlug}/apps/${appSlug}`,
  );
  return response.data;
}

export async function createApp(
  teamSlug: string,
  data: CreateAppRequest,
): Promise<App> {
  const response = await apiClient.post<App>(`/teams/${teamSlug}/apps`, data);
  return response.data;
}

export async function updateApp(
  teamSlug: string,
  appSlug: string,
  data: Partial<CreateAppRequest>,
): Promise<App> {
  const response = await apiClient.put<App>(
    `/teams/${teamSlug}/apps/${appSlug}`,
    data,
  );
  return response.data;
}

export async function deleteApp(
  teamSlug: string,
  appSlug: string,
): Promise<void> {
  await apiClient.delete(`/teams/${teamSlug}/apps/${appSlug}`);
}
