package domain

import (
	"time"

	"github.com/google/uuid"
)

// Metric represents a single metric data point
type Metric struct {
	Time   time.Time `json:"time"`
	RunID  uuid.UUID `json:"run_id"`
	Name   string    `json:"name"`
	Step   int64     `json:"step"`
	Value  float64   `json:"value"`
}

// MetricPoint is a simplified metric for batch operations
type MetricPoint struct {
	Name      string    `json:"name" validate:"required,max=255"`
	Step      int64     `json:"step" validate:"gte=0"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// MetricSeries represents a time series of metric values
type MetricSeries struct {
	Name   string        `json:"name"`
	Points []MetricPoint `json:"points"`
}

// SystemMetric represents system-level metrics (GPU, CPU, memory)
type SystemMetric struct {
	Time       time.Time `json:"time"`
	RunID      uuid.UUID `json:"run_id"`
	MetricType string    `json:"metric_type"` // gpu_memory, gpu_utilization, cpu, memory
	DeviceID   int       `json:"device_id"`
	Value      float64   `json:"value"`
}

// BatchMetricsRequest contains a batch of metrics to log
type BatchMetricsRequest struct {
	Metrics []MetricPoint `json:"metrics" validate:"required,min=1,max=1000,dive"`
}

// MetricQueryOptions contains options for querying metrics
type MetricQueryOptions struct {
	RunID      uuid.UUID
	MetricName string
	Since      *time.Time
	Until      *time.Time
	MinStep    *int64
	MaxStep    *int64
	MaxPoints  int // For downsampling
	Limit      int
	Offset     int
}

// MetricSummary contains summary statistics for a metric
type MetricSummary struct {
	Name        string    `json:"name"`
	Count       int64     `json:"count"`
	MinValue    float64   `json:"min_value"`
	MaxValue    float64   `json:"max_value"`
	AvgValue    float64   `json:"avg_value"`
	FirstValue  float64   `json:"first_value"`
	LastValue   float64   `json:"last_value"`
	FirstStep   int64     `json:"first_step"`
	LastStep    int64     `json:"last_step"`
	FirstTime   time.Time `json:"first_time"`
	LastTime    time.Time `json:"last_time"`
}

// RunMetricsSummary contains metric summaries for a run
type RunMetricsSummary struct {
	RunID       uuid.UUID        `json:"run_id"`
	TotalPoints int64            `json:"total_points"`
	MetricCount int              `json:"metric_count"`
	Metrics     []MetricSummary  `json:"metrics"`
}

// MetricUpdate represents a real-time metric update (for WebSocket)
type MetricUpdate struct {
	RunID     uuid.UUID `json:"run_id"`
	Name      string    `json:"name"`
	Step      int64     `json:"step"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}
