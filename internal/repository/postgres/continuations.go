package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sixtyseven/internal/domain"
)

// ContinuationRepository implements the continuation repository interface
type ContinuationRepository struct {
	db *DB
}

// NewContinuationRepository creates a new continuation repository
func NewContinuationRepository(db *DB) *ContinuationRepository {
	return &ContinuationRepository{db: db}
}

// Create creates a new continuation
func (r *ContinuationRepository) Create(ctx context.Context, continuation *domain.Continuation) error {
	query := `
		INSERT INTO run_continuations (id, run_id, step, timestamp, config_before, config_after, note, git_info, system_info, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	if continuation.ID == uuid.Nil {
		continuation.ID = uuid.New()
	}

	_, err := r.db.Pool.Exec(ctx, query,
		continuation.ID,
		continuation.RunID,
		continuation.Step,
		continuation.Timestamp,
		continuation.ConfigBefore,
		continuation.ConfigAfter,
		continuation.Note,
		continuation.GitInfo,
		continuation.SystemInfo,
		continuation.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create continuation: %w", err)
	}

	return nil
}

// GetByID retrieves a continuation by ID
func (r *ContinuationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Continuation, error) {
	query := `
		SELECT id, run_id, step, timestamp, config_before, config_after, note, git_info, system_info, created_at
		FROM run_continuations
		WHERE id = $1
	`

	continuation := &domain.Continuation{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&continuation.ID,
		&continuation.RunID,
		&continuation.Step,
		&continuation.Timestamp,
		&continuation.ConfigBefore,
		&continuation.ConfigAfter,
		&continuation.Note,
		&continuation.GitInfo,
		&continuation.SystemInfo,
		&continuation.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get continuation by ID: %w", err)
	}

	return continuation, nil
}

// ListByRun retrieves all continuations for a run ordered by step
func (r *ContinuationRepository) ListByRun(ctx context.Context, runID uuid.UUID) ([]*domain.Continuation, error) {
	query := `
		SELECT id, run_id, step, timestamp, config_before, config_after, note, git_info, system_info, created_at
		FROM run_continuations
		WHERE run_id = $1
		ORDER BY step ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list continuations: %w", err)
	}
	defer rows.Close()

	var continuations []*domain.Continuation
	for rows.Next() {
		continuation := &domain.Continuation{}
		err := rows.Scan(
			&continuation.ID,
			&continuation.RunID,
			&continuation.Step,
			&continuation.Timestamp,
			&continuation.ConfigBefore,
			&continuation.ConfigAfter,
			&continuation.Note,
			&continuation.GitInfo,
			&continuation.SystemInfo,
			&continuation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan continuation: %w", err)
		}
		continuations = append(continuations, continuation)
	}

	return continuations, nil
}

// Delete deletes a continuation by ID
func (r *ContinuationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM run_continuations WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete continuation: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("continuation not found")
	}

	return nil
}

// DeleteByRun deletes all continuations for a run
func (r *ContinuationRepository) DeleteByRun(ctx context.Context, runID uuid.UUID) error {
	query := `DELETE FROM run_continuations WHERE run_id = $1`

	_, err := r.db.Pool.Exec(ctx, query, runID)
	if err != nil {
		return fmt.Errorf("failed to delete continuations by run: %w", err)
	}

	return nil
}
