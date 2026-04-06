package domain

import "time"

// SweepStatus represents the status of a hyperparameter sweep
type SweepStatus string

const (
	SweepStatusRunning   SweepStatus = "running"
	SweepStatusCompleted SweepStatus = "completed"
	SweepStatusFailed    SweepStatus = "failed"
	SweepStatusStopped   SweepStatus = "stopped"
)

// ParameterSpec defines a hyperparameter search space
type ParameterSpec struct {
	Name   string    `json:"name"`
	Type   string    `json:"type"` // uniform, log_uniform, int, categorical
	Min    *float64  `json:"min,omitempty"`
	Max    *float64  `json:"max,omitempty"`
	Values []any     `json:"values,omitempty"`
}

// SearchSpace contains the hyperparameter search space
type SearchSpace struct {
	Parameters []ParameterSpec `json:"parameters"`
}

// EarlyStoppingConfig configures early stopping for a sweep
type EarlyStoppingConfig struct {
	Method   string `json:"method"`
	MinSteps int    `json:"min_steps"`
	Warmup   int    `json:"warmup"`
}

// Sweep represents a hyperparameter sweep
type Sweep struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Status        SweepStatus          `json:"status"`
	Method        string               `json:"method"` // random, grid
	MetricName    string               `json:"metric_name"`
	MetricGoal    string               `json:"metric_goal"` // minimize, maximize
	SearchSpace   SearchSpace          `json:"search_space"`
	Config        map[string]any       `json:"config,omitempty"`
	MaxRuns       *int                 `json:"max_runs,omitempty"`
	EarlyStopping *EarlyStoppingConfig `json:"early_stopping,omitempty"`
	BestRunID     *string              `json:"best_run_id,omitempty"`
	BestValue     *float64             `json:"best_value,omitempty"`
	RunCount      int                  `json:"run_count"`
	GridIndex     int                  `json:"grid_index"`
	StartedAt     time.Time            `json:"started_at"`
	EndedAt       *time.Time           `json:"ended_at,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
}
