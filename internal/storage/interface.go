// Package storage provides a unified interface for metric storage backends.
package storage

import (
	"context"
	"time"

	"github.com/ninetyfive/p95/internal/domain"
)

// Project represents a project directory in local mode.
type Project struct {
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	RunCount    int       `json:"run_count"`
	LastUpdated time.Time `json:"last_updated"`
}

// Storage defines the interface for accessing runs and metrics.
// This abstraction allows for different backends (PostgreSQL, file-based, etc.)
type Storage interface {
	// Projects (local mode only - in hosted mode, use teams/apps)
	ListProjects(ctx context.Context) ([]Project, error)

	// Runs
	ListRuns(ctx context.Context, project string, opts domain.RunListOptions) ([]*domain.Run, error)
	GetRun(ctx context.Context, runID string) (*domain.Run, error)
	GetRunByProject(ctx context.Context, project, runID string) (*domain.Run, error)

	// Metrics
	GetMetricNames(ctx context.Context, runID string) ([]string, error)
	GetMetricSeries(ctx context.Context, runID, metricName string, opts MetricQueryOptions) ([]MetricPoint, error)
	GetLatestMetrics(ctx context.Context, runID string) (map[string]float64, error)
	GetMetricsSummary(ctx context.Context, runID string) (*MetricsSummary, error)

	// Sweeps
	ListSweeps(ctx context.Context, project string) ([]domain.Sweep, error)
	GetSweep(ctx context.Context, project, sweepID string) (*domain.Sweep, error)
	GetSweepByID(ctx context.Context, sweepID string) (*domain.Sweep, string, error)
	GetSweepRuns(ctx context.Context, project, sweepID string) ([]*domain.Run, error)
	StopSweep(ctx context.Context, project, sweepID string) error

	// Health check
	Health(ctx context.Context) error

	// Cleanup
	Close() error
}

// MetricQueryOptions contains options for querying metric series.
type MetricQueryOptions struct {
	Since     *time.Time
	Until     *time.Time
	MinStep   *int64
	MaxStep   *int64
	MaxPoints int
	Limit     int
	Offset    int
}

// MetricPoint represents a single metric data point.
type MetricPoint struct {
	Time  time.Time `json:"time"`
	Step  int64     `json:"step"`
	Value float64   `json:"value"`
}

// MetricSummary contains summary statistics for a single metric.
type MetricSummary struct {
	Name       string    `json:"name"`
	Count      int64     `json:"count"`
	MinValue   float64   `json:"min_value"`
	MaxValue   float64   `json:"max_value"`
	AvgValue   float64   `json:"avg_value"`
	FirstValue float64   `json:"first_value"`
	LastValue  float64   `json:"last_value"`
	FirstStep  int64     `json:"first_step"`
	LastStep   int64     `json:"last_step"`
	FirstTime  time.Time `json:"first_time"`
	LastTime   time.Time `json:"last_time"`
}

// MetricsSummary contains summary statistics for all metrics in a run.
type MetricsSummary struct {
	RunID       string          `json:"run_id"`
	TotalPoints int64           `json:"total_points"`
	MetricCount int             `json:"metric_count"`
	Metrics     []MetricSummary `json:"metrics"`
}
