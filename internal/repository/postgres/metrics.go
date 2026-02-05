package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sixtyseven/internal/domain"
)

// MetricsRepository implements the metrics repository interface for TimescaleDB
type MetricsRepository struct {
	db *DB
}

// NewMetricsRepository creates a new metrics repository
func NewMetricsRepository(db *DB) *MetricsRepository {
	return &MetricsRepository{db: db}
}

// BatchInsert inserts a batch of metrics efficiently
func (r *MetricsRepository) BatchInsert(ctx context.Context, runID uuid.UUID, metrics []domain.MetricPoint) error {
	if len(metrics) == 0 {
		return nil
	}

	// Use COPY for efficient bulk insert
	columns := []string{"time", "run_id", "name", "step", "value"}

	rows := make([][]any, len(metrics))
	for i, m := range metrics {
		ts := m.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		rows[i] = []any{ts, runID, m.Name, m.Step, m.Value}
	}

	_, err := r.db.Pool.CopyFrom(
		ctx,
		pgx.Identifier{"metrics"},
		columns,
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		return fmt.Errorf("failed to batch insert metrics: %w", err)
	}

	return nil
}

// GetSeries retrieves a metric time series
func (r *MetricsRepository) GetSeries(ctx context.Context, opts domain.MetricQueryOptions) ([]domain.Metric, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("run_id = $%d", argIdx))
	args = append(args, opts.RunID)
	argIdx++

	if opts.MetricName != "" {
		conditions = append(conditions, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, opts.MetricName)
		argIdx++
	}

	if opts.Since != nil {
		conditions = append(conditions, fmt.Sprintf("time >= $%d", argIdx))
		args = append(args, *opts.Since)
		argIdx++
	}

	if opts.Until != nil {
		conditions = append(conditions, fmt.Sprintf("time <= $%d", argIdx))
		args = append(args, *opts.Until)
		argIdx++
	}

	if opts.MinStep != nil {
		conditions = append(conditions, fmt.Sprintf("step >= $%d", argIdx))
		args = append(args, *opts.MinStep)
		argIdx++
	}

	if opts.MaxStep != nil {
		conditions = append(conditions, fmt.Sprintf("step <= $%d", argIdx))
		args = append(args, *opts.MaxStep)
		argIdx++
	}

	// Build query with optional downsampling
	var query string
	if opts.MaxPoints > 0 {
		// Use LTTB-like downsampling via window function
		query = fmt.Sprintf(`
			WITH numbered AS (
				SELECT time, run_id, name, step, value,
					   ROW_NUMBER() OVER (PARTITION BY name ORDER BY step) as rn,
					   COUNT(*) OVER (PARTITION BY name) as total
				FROM metrics
				WHERE %s
			)
			SELECT time, run_id, name, step, value
			FROM numbered
			WHERE rn %% GREATEST(1, total / $%d) = 0
			ORDER BY name, step
		`, joinConditions(conditions), argIdx)
		args = append(args, opts.MaxPoints)
	} else {
		query = fmt.Sprintf(`
			SELECT time, run_id, name, step, value
			FROM metrics
			WHERE %s
			ORDER BY step
		`, joinConditions(conditions))

		if opts.Limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIdx)
			args = append(args, opts.Limit)
			argIdx++
		}
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIdx)
			args = append(args, opts.Offset)
		}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric series: %w", err)
	}
	defer rows.Close()

	var metrics []domain.Metric
	for rows.Next() {
		var m domain.Metric
		err := rows.Scan(&m.Time, &m.RunID, &m.Name, &m.Step, &m.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// GetLatest retrieves the latest value for each metric in a run
func (r *MetricsRepository) GetLatest(ctx context.Context, runID uuid.UUID) (map[string]float64, error) {
	query := `
		SELECT DISTINCT ON (name) name, value
		FROM metrics
		WHERE run_id = $1
		ORDER BY name, time DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest metrics: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("failed to scan latest metric: %w", err)
		}
		result[name] = value
	}

	return result, nil
}

// GetMetricNames retrieves all unique metric names for a run
func (r *MetricsRepository) GetMetricNames(ctx context.Context, runID uuid.UUID) ([]string, error) {
	query := `
		SELECT DISTINCT name
		FROM metrics
		WHERE run_id = $1
		ORDER BY name
	`

	rows, err := r.db.Pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan metric name: %w", err)
		}
		names = append(names, name)
	}

	return names, nil
}

// GetSummary retrieves summary statistics for all metrics in a run
func (r *MetricsRepository) GetSummary(ctx context.Context, runID uuid.UUID) (*domain.RunMetricsSummary, error) {
	query := `
		SELECT
			name,
			COUNT(*) as count,
			MIN(value) as min_value,
			MAX(value) as max_value,
			AVG(value) as avg_value,
			(SELECT value FROM metrics m2 WHERE m2.run_id = $1 AND m2.name = m.name ORDER BY step ASC LIMIT 1) as first_value,
			(SELECT value FROM metrics m2 WHERE m2.run_id = $1 AND m2.name = m.name ORDER BY step DESC LIMIT 1) as last_value,
			MIN(step) as first_step,
			MAX(step) as last_step,
			MIN(time) as first_time,
			MAX(time) as last_time
		FROM metrics m
		WHERE run_id = $1
		GROUP BY name
		ORDER BY name
	`

	rows, err := r.db.Pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics summary: %w", err)
	}
	defer rows.Close()

	summary := &domain.RunMetricsSummary{
		RunID:   runID,
		Metrics: []domain.MetricSummary{},
	}

	for rows.Next() {
		var m domain.MetricSummary
		err := rows.Scan(
			&m.Name,
			&m.Count,
			&m.MinValue,
			&m.MaxValue,
			&m.AvgValue,
			&m.FirstValue,
			&m.LastValue,
			&m.FirstStep,
			&m.LastStep,
			&m.FirstTime,
			&m.LastTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric summary: %w", err)
		}
		summary.TotalPoints += m.Count
		summary.Metrics = append(summary.Metrics, m)
	}

	summary.MetricCount = len(summary.Metrics)
	return summary, nil
}

// DeleteByRun deletes all metrics for a run
func (r *MetricsRepository) DeleteByRun(ctx context.Context, runID uuid.UUID) error {
	query := `DELETE FROM metrics WHERE run_id = $1`

	_, err := r.db.Pool.Exec(ctx, query, runID)
	if err != nil {
		return fmt.Errorf("failed to delete metrics: %w", err)
	}

	return nil
}

// Helper function to join conditions
func joinConditions(conditions []string) string {
	result := ""
	for i, c := range conditions {
		if i > 0 {
			result += " AND "
		}
		result += c
	}
	return result
}
