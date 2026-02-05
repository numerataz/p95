package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sixtyseven/internal/domain"
)

// AppRepository implements the app repository interface
type AppRepository struct {
	db *DB
}

// NewAppRepository creates a new app repository
func NewAppRepository(db *DB) *AppRepository {
	return &AppRepository{db: db}
}

// Create creates a new app
func (r *AppRepository) Create(ctx context.Context, app *domain.App) error {
	query := `
		INSERT INTO apps (id, team_id, name, slug, description, visibility, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}

	_, err := r.db.Pool.Exec(ctx, query,
		app.ID,
		app.TeamID,
		app.Name,
		app.Slug,
		app.Description,
		app.Visibility,
		app.Settings,
		app.CreatedAt,
		app.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	return nil
}

// GetByID retrieves an app by ID
func (r *AppRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.App, error) {
	query := `
		SELECT a.id, a.team_id, a.name, a.slug, a.description, a.visibility, a.settings, a.archived_at, a.created_at, a.updated_at,
			   (SELECT COUNT(*) FROM runs WHERE app_id = a.id) as run_count
		FROM apps a
		WHERE a.id = $1
	`

	app := &domain.App{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&app.ID,
		&app.TeamID,
		&app.Name,
		&app.Slug,
		&app.Description,
		&app.Visibility,
		&app.Settings,
		&app.ArchivedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
		&app.RunCount,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get app by ID: %w", err)
	}

	return app, nil
}

// GetBySlug retrieves an app by team ID and slug
func (r *AppRepository) GetBySlug(ctx context.Context, teamID uuid.UUID, slug string) (*domain.App, error) {
	query := `
		SELECT a.id, a.team_id, a.name, a.slug, a.description, a.visibility, a.settings, a.archived_at, a.created_at, a.updated_at,
			   (SELECT COUNT(*) FROM runs WHERE app_id = a.id) as run_count
		FROM apps a
		WHERE a.team_id = $1 AND a.slug = $2
	`

	app := &domain.App{}
	err := r.db.Pool.QueryRow(ctx, query, teamID, slug).Scan(
		&app.ID,
		&app.TeamID,
		&app.Name,
		&app.Slug,
		&app.Description,
		&app.Visibility,
		&app.Settings,
		&app.ArchivedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
		&app.RunCount,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get app by slug: %w", err)
	}

	return app, nil
}

// Update updates an app
func (r *AppRepository) Update(ctx context.Context, app *domain.App) error {
	query := `
		UPDATE apps
		SET name = $2, description = $3, visibility = $4, settings = $5, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		app.ID,
		app.Name,
		app.Description,
		app.Visibility,
		app.Settings,
	)

	if err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("app not found")
	}

	return nil
}

// Delete deletes an app
func (r *AppRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM apps WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("app not found")
	}

	return nil
}

// Archive archives an app
func (r *AppRepository) Archive(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE apps
		SET archived_at = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to archive app: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("app not found")
	}

	return nil
}

// List retrieves apps with filtering options
func (r *AppRepository) List(ctx context.Context, opts domain.AppListOptions) ([]*domain.App, error) {
	query := `
		SELECT a.id, a.team_id, a.name, a.slug, a.description, a.visibility, a.settings, a.archived_at, a.created_at, a.updated_at,
			   (SELECT COUNT(*) FROM runs WHERE app_id = a.id) as run_count
		FROM apps a
		WHERE a.team_id = $1
	`

	args := []any{opts.TeamID}
	argIdx := 2

	if !opts.IncludeArchived {
		query += fmt.Sprintf(" AND a.archived_at IS NULL")
	}

	query += " ORDER BY a.name"

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
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}
	defer rows.Close()

	var apps []*domain.App
	for rows.Next() {
		app := &domain.App{}
		err := rows.Scan(
			&app.ID,
			&app.TeamID,
			&app.Name,
			&app.Slug,
			&app.Description,
			&app.Visibility,
			&app.Settings,
			&app.ArchivedAt,
			&app.CreatedAt,
			&app.UpdatedAt,
			&app.RunCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan app: %w", err)
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// CountByTeam counts apps in a team
func (r *AppRepository) CountByTeam(ctx context.Context, teamID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM apps WHERE team_id = $1 AND archived_at IS NULL`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, teamID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count apps: %w", err)
	}

	return count, nil
}
