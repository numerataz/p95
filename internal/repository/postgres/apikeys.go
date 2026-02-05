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

// APIKeyRepository implements the API key repository interface
type APIKeyRepository struct {
	db *DB
}

// NewAPIKeyRepository creates a new API key repository
func NewAPIKeyRepository(db *DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create creates a new API key
func (r *APIKeyRepository) Create(ctx context.Context, key *domain.APIKey) error {
	query := `
		INSERT INTO api_keys (id, user_id, team_id, key_hash, key_prefix, name, scopes, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}

	_, err := r.db.Pool.Exec(ctx, query,
		key.ID,
		key.UserID,
		key.TeamID,
		key.KeyHash,
		key.KeyPrefix,
		key.Name,
		key.Scopes,
		key.ExpiresAt,
		key.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	return nil
}

// GetByID retrieves an API key by ID
func (r *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	query := `
		SELECT k.id, k.user_id, k.team_id, k.key_hash, k.key_prefix, k.name, k.scopes,
			   k.last_used_at, k.expires_at, k.created_at,
			   u.id, u.email, u.name, u.avatar_url, u.is_active, u.is_admin, u.created_at, u.updated_at
		FROM api_keys k
		JOIN users u ON k.user_id = u.id
		WHERE k.id = $1
	`

	key := &domain.APIKey{}
	user := &domain.User{}

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&key.ID,
		&key.UserID,
		&key.TeamID,
		&key.KeyHash,
		&key.KeyPrefix,
		&key.Name,
		&key.Scopes,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.CreatedAt,
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.IsActive,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get API key by ID: %w", err)
	}

	key.User = user
	return key, nil
}

// GetByPrefix retrieves API keys by prefix (for authentication)
func (r *APIKeyRepository) GetByPrefix(ctx context.Context, prefix string) ([]*domain.APIKey, error) {
	query := `
		SELECT k.id, k.user_id, k.team_id, k.key_hash, k.key_prefix, k.name, k.scopes,
			   k.last_used_at, k.expires_at, k.created_at,
			   u.id, u.email, u.name, u.avatar_url, u.is_active, u.is_admin, u.created_at, u.updated_at
		FROM api_keys k
		JOIN users u ON k.user_id = u.id
		WHERE k.key_prefix = $1
	`

	rows, err := r.db.Pool.Query(ctx, query, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to get API keys by prefix: %w", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		key := &domain.APIKey{}
		user := &domain.User{}

		err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.TeamID,
			&key.KeyHash,
			&key.KeyPrefix,
			&key.Name,
			&key.Scopes,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.CreatedAt,
			&user.ID,
			&user.Email,
			&user.Name,
			&user.AvatarURL,
			&user.IsActive,
			&user.IsAdmin,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		key.User = user
		keys = append(keys, key)
	}

	return keys, nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`

	_, err := r.db.Pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update last used: %w", err)
	}

	return nil
}

// Delete deletes an API key
func (r *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// ListByUser retrieves all API keys for a user
func (r *APIKeyRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	query := `
		SELECT id, user_id, team_id, key_hash, key_prefix, name, scopes, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		key := &domain.APIKey{}
		err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.TeamID,
			&key.KeyHash,
			&key.KeyPrefix,
			&key.Name,
			&key.Scopes,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}
