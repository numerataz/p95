import axios, { AxiosError } from "axios";

export const apiClient = axios.create({
  baseURL: "/api/v1",
  headers: {
    "Content-Type": "application/json",
  },
});

export interface ApiError {
  error: string;
  code?: string;
}

export function isApiError(error: unknown): error is AxiosError<ApiError> {
  return axios.isAxiosError(error);
}

export function getErrorMessage(error: unknown): string {
  if (isApiError(error)) {
    return error.response?.data?.error || error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "An unexpected error occurred";
}
