// User types
export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  is_admin: boolean;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  expires_at: string;
  user: User;
}

// Team types
export type TeamRole = "owner" | "admin" | "member" | "viewer";
export type TeamPlan = "free" | "pro" | "enterprise";

export interface Team {
  id: string;
  name: string;
  slug: string;
  description?: string;
  plan: TeamPlan;
  is_personal: boolean;
  settings?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface TeamWithRole extends Team {
  role: TeamRole;
}

export interface TeamMember {
  id: string;
  team_id: string;
  user_id: string;
  role: TeamRole;
  created_at: string;
  user?: User;
}

export interface CreateTeamRequest {
  name: string;
  slug: string;
  description?: string;
}

// App types
export type AppVisibility = "private" | "team" | "public";

export interface App {
  id: string;
  team_id: string;
  name: string;
  slug: string;
  description?: string;
  visibility: AppVisibility;
  settings?: Record<string, unknown>;
  archived_at?: string;
  created_at: string;
  updated_at: string;
  team?: Team;
  run_count?: number;
}

export interface CreateAppRequest {
  name: string;
  slug: string;
  description?: string;
  visibility?: AppVisibility;
}

// Run types
export type RunStatus =
  | "running"
  | "completed"
  | "failed"
  | "aborted"
  | "canceled";

export interface GitInfo {
  commit?: string;
  branch?: string;
  remote?: string;
  dirty?: boolean;
  message?: string;
}

export interface SystemInfo {
  hostname?: string;
  os?: string;
  python_version?: string;
  gpu_info?: string[];
  cpu_count?: number;
  memory_gb?: number;
}

export interface Run {
  id: string;
  app_id: string;
  user_id: string;
  name: string;
  description?: string;
  status: RunStatus;
  tags?: string[];
  git_info?: GitInfo;
  system_info?: SystemInfo;
  config?: Record<string, unknown>;
  error_message?: string;
  started_at: string;
  ended_at?: string;
  duration_seconds?: number;
  created_at: string;
  app?: App;
  user?: User;
  latest_metrics?: Record<string, number>;
  metric_count?: number;
}

export interface RunFilters {
  status?: RunStatus;
  tags?: string[];
  limit?: number;
  offset?: number;
  order_by?: "started_at" | "name" | "status";
  order_dir?: "asc" | "desc";
}

// Metric types
export interface MetricPoint {
  name: string;
  step: number;
  value: number;
  timestamp?: string;
}

export interface MetricSeries {
  name: string;
  points: Array<{
    step: number;
    value: number;
    time?: string;
  }>;
}

export interface MetricSummary {
  name: string;
  count: number;
  min_value: number;
  max_value: number;
  avg_value: number;
  first_value: number;
  last_value: number;
  first_step: number;
  last_step: number;
  first_time: string;
  last_time: string;
}

export interface MetricsSummaryResponse {
  run_id: string;
  total_points: number;
  metric_count: number;
  metrics: MetricSummary[];
}

// Sweep types
export type SweepStatus = "running" | "completed" | "failed" | "stopped";
export type SearchMethod = "random" | "grid";
export type MetricGoal = "minimize" | "maximize";

export interface ParameterSpec {
  name: string;
  type: "uniform" | "log_uniform" | "int" | "categorical";
  min?: number;
  max?: number;
  values?: unknown[];
}

export interface SearchSpace {
  parameters: ParameterSpec[];
}

export interface EarlyStoppingConfig {
  method: string;
  min_steps: number;
  warmup: number;
}

export interface Sweep {
  id: string;
  name: string;
  status: SweepStatus;
  method: SearchMethod;
  metric_name: string;
  metric_goal: MetricGoal;
  search_space: SearchSpace;
  config?: Record<string, unknown>;
  max_runs?: number;
  early_stopping?: EarlyStoppingConfig;
  best_run_id?: string;
  best_value?: number;
  run_count: number;
  grid_index: number;
  started_at: string;
  ended_at?: string;
  created_at: string;
}

export interface SweepFilters {
  status?: SweepStatus;
  limit?: number;
  offset?: number;
  order_by?: "started_at" | "name" | "status";
  order_dir?: "asc" | "desc";
}

// API Key types
export type APIKeyScope = "read" | "write" | "admin";

export interface APIKey {
  id: string;
  user_id: string;
  team_id?: string;
  key_prefix: string;
  name: string;
  scopes: APIKeyScope[];
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
}

export interface APIKeyResponse extends APIKey {
  key: string;
}

export interface CreateAPIKeyRequest {
  name: string;
  scopes: APIKeyScope[];
  team_id?: string;
  expires_at?: string;
}

// Pagination
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
}

// Continuation types
export interface Continuation {
  id: string;
  run_id: string;
  step: number;
  timestamp: string;
  config_before?: Record<string, unknown>;
  config_after?: Record<string, unknown>;
  note?: string;
  git_info?: GitInfo;
  system_info?: SystemInfo;
  created_at: string;
}

export interface ConfigChange {
  before?: unknown;
  after?: unknown;
  type: "added" | "modified" | "removed";
}

export interface ResumeRunRequest {
  config?: Record<string, unknown>;
  note?: string;
  git_info?: GitInfo;
  system_info?: SystemInfo;
}

export interface ResumeRunResponse {
  run: Run;
  continuation: Continuation;
}
