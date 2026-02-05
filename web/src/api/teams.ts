import { apiClient } from "./client";
import type {
  Team,
  TeamWithRole,
  TeamMember,
  CreateTeamRequest,
} from "./types";

export async function getTeams(): Promise<TeamWithRole[]> {
  const response = await apiClient.get<TeamWithRole[]>("/teams");
  return response.data;
}

export async function getTeam(teamSlug: string): Promise<Team> {
  const response = await apiClient.get<Team>(`/teams/${teamSlug}`);
  return response.data;
}

export async function createTeam(data: CreateTeamRequest): Promise<Team> {
  const response = await apiClient.post<Team>("/teams", data);
  return response.data;
}

export async function updateTeam(
  teamSlug: string,
  data: Partial<CreateTeamRequest>,
): Promise<Team> {
  const response = await apiClient.put<Team>(`/teams/${teamSlug}`, data);
  return response.data;
}

export async function deleteTeam(teamSlug: string): Promise<void> {
  await apiClient.delete(`/teams/${teamSlug}`);
}

export async function getTeamMembers(teamSlug: string): Promise<TeamMember[]> {
  const response = await apiClient.get<TeamMember[]>(
    `/teams/${teamSlug}/members`,
  );
  return response.data;
}

export async function addTeamMember(
  teamSlug: string,
  userId: string,
  role: string,
): Promise<TeamMember> {
  const response = await apiClient.post<TeamMember>(
    `/teams/${teamSlug}/members`,
    { user_id: userId, role },
  );
  return response.data;
}

export async function updateTeamMember(
  teamSlug: string,
  userId: string,
  role: string,
): Promise<TeamMember> {
  const response = await apiClient.put<TeamMember>(
    `/teams/${teamSlug}/members/${userId}`,
    { role },
  );
  return response.data;
}

export async function removeTeamMember(
  teamSlug: string,
  userId: string,
): Promise<void> {
  await apiClient.delete(`/teams/${teamSlug}/members/${userId}`);
}
