package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sixtyseven/internal/domain"
)

// RunRepository implements the run repository interface
type RunRepository struct {
	db *DB
}

// NewRunRepository creates a new run repository
func NewRunRepository(db *DB) *RunRepository {
	return &RunRepository{db: db}
}

// Create creates a new run
func (r *RunRepository) Create(ctx context.Context, run *domain.Run) error {
	query := `
		INSERT INTO runs (id, app_id, user_id, name, description, status, tags, git_info, system_info, config, started_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}

	_, err := r.db.Pool.Exec(ctx, query,
		run.ID,
		run.AppID,
		run.UserID,
		run.Name,
		run.Description,
		run.Status,
		run.Tags,
		run.GitInfo,
		run.SystemInfo,
		run.Config,
		run.StartedAt,
		run.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}

	return nil
}

// GetByID retrieves a run by ID
func (r *RunRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	query := `
		SELECT r.id, r.app_id, r.user_id, r.name, r.description, r.status, r.tags,
			   r.git_info, r.system_info, r.config, r.error_message,
			   r.started_at, r.ended_at, r.duration_seconds, r.created_at
		FROM runs r
		WHERE r.id = $1
	`

	run := &domain.Run{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&run.ID,
		&run.AppID,
		&run.UserID,
		&run.Name,
		&run.Description,
		&run.Status,
		&run.Tags,
		&run.GitInfo,
		&run.SystemInfo,
		&run.Config,
		&run.ErrorMessage,
		&run.StartedAt,
		&run.EndedAt,
		&run.DurationSeconds,
		&run.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get run by ID: %w", err)
	}

	return run, nil
}

// Update updates a run
func (r *RunRepository) Update(ctx context.Context, run *domain.Run) error {
	query := `
		UPDATE runs
		SET name = $2, description = $3, tags = $4, config = $5
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		run.ID,
		run.Name,
		run.Description,
		run.Tags,
		run.Config,
	)

	if err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("run not found")
	}

	return nil
}

// UpdateStatus updates a run's status
func (r *RunRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.RunStatus, errorMsg *string) error {
	now := time.Now()

	query := `
		UPDATE runs
		SET status = $2, error_message = $3, ended_at = $4,
			duration_seconds = EXTRACT(EPOCH FROM ($4 - started_at))
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query, id, status, errorMsg, now)
	if err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("run not found")
	}

	return nil
}

// UpdateConfig merges new config with existing config
func (r *RunRepository) UpdateConfig(ctx context.Context, id uuid.UUID, config map[string]any) error {
	query := `
		UPDATE runs
		SET config = config || $2
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query, id, config)
	if err != nil {
		return fmt.Errorf("failed to update run config: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("run not found")
	}

	return nil
}

// Delete deletes a run
func (r *RunRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM runs WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("run not found")
	}

	return nil
}

// List retrieves runs with filtering options
func (r *RunRepository) List(ctx context.Context, opts domain.RunListOptions) ([]*domain.Run, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("r.app_id = $%d", argIdx))
	args = append(args, opts.AppID)
	argIdx++

	if opts.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("r.user_id = $%d", argIdx))
		args = append(args, *opts.UserID)
		argIdx++
	}

	if opts.Status != nil {
		conditions = append(conditions, fmt.Sprintf("r.status = $%d", argIdx))
		args = append(args, *opts.Status)
		argIdx++
	}

	if len(opts.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("r.tags && $%d", argIdx))
		args = append(args, opts.Tags)
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.app_id, r.user_id, r.name, r.description, r.status, r.tags,
			   r.git_info, r.system_info, r.config, r.error_message,
			   r.started_at, r.ended_at, r.duration_seconds, r.created_at
		FROM runs r
		WHERE %s
	`, strings.Join(conditions, " AND "))

	// Order by
	orderBy := "r.started_at"
	if opts.OrderBy != "" {
		switch opts.OrderBy {
		case "name":
			orderBy = "r.name"
		case "status":
			orderBy = "r.status"
		}
	}
	orderDir := "DESC"
	if opts.OrderDir == "asc" {
		orderDir = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir)

	// Pagination
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, opts.Limit)
		argIdx++
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, opts.Offset)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	defer rows.Close()

	var runs []*domain.Run
	for rows.Next() {
		run := &domain.Run{}
		err := rows.Scan(
			&run.ID,
			&run.AppID,
			&run.UserID,
			&run.Name,
			&run.Description,
			&run.Status,
			&run.Tags,
			&run.GitInfo,
			&run.SystemInfo,
			&run.Config,
			&run.ErrorMessage,
			&run.StartedAt,
			&run.EndedAt,
			&run.DurationSeconds,
			&run.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// CountByApp counts runs in an app
func (r *RunRepository) CountByApp(ctx context.Context, appID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM runs WHERE app_id = $1`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, appID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count runs: %w", err)
	}

	return count, nil
}

// GetActiveByApp retrieves all active (running) runs for an app
func (r *RunRepository) GetActiveByApp(ctx context.Context, appID uuid.UUID) ([]*domain.Run, error) {
	query := `
		SELECT r.id, r.app_id, r.user_id, r.name, r.description, r.status, r.tags,
			   r.git_info, r.system_info, r.config, r.error_message,
			   r.started_at, r.ended_at, r.duration_seconds, r.created_at
		FROM runs r
		WHERE r.app_id = $1 AND r.status = 'running'
		ORDER BY r.started_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active runs: %w", err)
	}
	defer rows.Close()

	var runs []*domain.Run
	for rows.Next() {
		run := &domain.Run{}
		err := rows.Scan(
			&run.ID,
			&run.AppID,
			&run.UserID,
			&run.Name,
			&run.Description,
			&run.Status,
			&run.Tags,
			&run.GitInfo,
			&run.SystemInfo,
			&run.Config,
			&run.ErrorMessage,
			&run.StartedAt,
			&run.EndedAt,
			&run.DurationSeconds,
			&run.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}
