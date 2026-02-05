package domain

import (
	"time"

	"github.com/google/uuid"
)

// RunStatus represents the status of a training run
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusAborted   RunStatus = "aborted"
	RunStatusCanceled  RunStatus = "canceled"
)

// GitInfo contains git repository information
type GitInfo struct {
	Commit  string `json:"commit,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Remote  string `json:"remote,omitempty"`
	Dirty   bool   `json:"dirty,omitempty"`
	Message string `json:"message,omitempty"`
}

// SystemInfo contains system/hardware information
type SystemInfo struct {
	Hostname      string   `json:"hostname,omitempty"`
	OS            string   `json:"os,omitempty"`
	PythonVersion string   `json:"python_version,omitempty"`
	GPUInfo       []string `json:"gpu_info,omitempty"`
	CPUCount      int      `json:"cpu_count,omitempty"`
	MemoryGB      float64  `json:"memory_gb,omitempty"`
}

// Run represents a single training run
type Run struct {
	ID              uuid.UUID      `json:"id"`
	AppID           uuid.UUID      `json:"app_id"`
	UserID          uuid.UUID      `json:"user_id"`
	Name            string         `json:"name"`
	Description     *string        `json:"description,omitempty"`
	Status          RunStatus      `json:"status"`
	Tags            []string       `json:"tags,omitempty"`
	GitInfo         *GitInfo       `json:"git_info,omitempty"`
	SystemInfo      *SystemInfo    `json:"system_info,omitempty"`
	Config          map[string]any `json:"config,omitempty"`
	ErrorMessage    *string        `json:"error_message,omitempty"`
	StartedAt       time.Time      `json:"started_at"`
	EndedAt         *time.Time     `json:"ended_at,omitempty"`
	DurationSeconds *float64       `json:"duration_seconds,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`

	// Populated by joins
	App  *App  `json:"app,omitempty"`
	User *User `json:"user,omitempty"`

	// Computed fields
	LatestMetrics map[string]float64 `json:"latest_metrics,omitempty"`
	MetricCount   int                `json:"metric_count,omitempty"`
}

// RunCreate contains fields for creating a new run
type RunCreate struct {
	Name        string         `json:"name,omitempty" validate:"omitempty,max=255"`
	Description *string        `json:"description,omitempty" validate:"omitempty,max=1000"`
	Tags        []string       `json:"tags,omitempty"`
	GitInfo     *GitInfo       `json:"git_info,omitempty"`
	SystemInfo  *SystemInfo    `json:"system_info,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
}

// RunUpdate contains fields for updating a run
type RunUpdate struct {
	Name        *string        `json:"name,omitempty" validate:"omitempty,max=255"`
	Description *string        `json:"description,omitempty" validate:"omitempty,max=1000"`
	Tags        []string       `json:"tags,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
}

// RunStatusUpdate contains fields for updating run status
type RunStatusUpdate struct {
	Status       RunStatus `json:"status" validate:"required,oneof=completed failed aborted"`
	ErrorMessage *string   `json:"error_message,omitempty"`
}

// RunListOptions contains options for listing runs
type RunListOptions struct {
	AppID    uuid.UUID
	UserID   *uuid.UUID
	Status   *RunStatus
	Tags     []string
	Limit    int
	Offset   int
	OrderBy  string // "started_at", "name", "status"
	OrderDir string // "asc", "desc"
}

// IsActive returns true if the run is still active (running)
func (r *Run) IsActive() bool {
	return r.Status == RunStatusRunning
}

// Complete marks the run as completed
func (r *Run) Complete() {
	now := time.Now()
	r.Status = RunStatusCompleted
	r.EndedAt = &now
	duration := now.Sub(r.StartedAt).Seconds()
	r.DurationSeconds = &duration
}

// Fail marks the run as failed with an error message
func (r *Run) Fail(errorMsg string) {
	now := time.Now()
	r.Status = RunStatusFailed
	r.EndedAt = &now
	r.ErrorMessage = &errorMsg
	duration := now.Sub(r.StartedAt).Seconds()
	r.DurationSeconds = &duration
}
