import { apiClient } from "./client";
import type {
  LoginRequest,
  LoginResponse,
  User,
  APIKey,
  APIKeyResponse,
  CreateAPIKeyRequest,
} from "./types";

export async function login(data: LoginRequest): Promise<LoginResponse> {
  const response = await apiClient.post<LoginResponse>("/auth/login", data);
  return response.data;
}

export async function getMe(): Promise<User> {
  const response = await apiClient.get<User>("/auth/me");
  return response.data;
}

export async function logout(): Promise<void> {
  await apiClient.post("/auth/logout");
}

export async function getApiKeys(): Promise<APIKey[]> {
  const response = await apiClient.get<APIKey[]>("/auth/api-keys");
  return response.data;
}

export async function createApiKey(
  data: CreateAPIKeyRequest,
): Promise<APIKeyResponse> {
  const response = await apiClient.post<APIKeyResponse>("/auth/api-keys", data);
  return response.data;
}

export async function deleteApiKey(keyId: string): Promise<void> {
  await apiClient.delete(`/auth/api-keys/${keyId}`);
}
