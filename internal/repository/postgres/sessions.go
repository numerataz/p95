package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sixtyseven/internal/domain"
)

// SessionRepository implements the session repository interface
type SessionRepository struct {
	db *DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, token_hash, device_info, ip_address, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	_, err := r.db.Pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.DeviceInfo,
		session.IPAddress,
		session.ExpiresAt,
		session.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetByID retrieves a session by ID
func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	query := `
		SELECT s.id, s.user_id, s.token_hash, s.device_info, s.ip_address, s.expires_at, s.created_at,
			   u.id, u.email, u.name, u.avatar_url, u.is_active, u.is_admin, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1
	`

	session := &domain.Session{}
	user := &domain.User{}

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.DeviceInfo,
		&session.IPAddress,
		&session.ExpiresAt,
		&session.CreatedAt,
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
		return nil, fmt.Errorf("failed to get session by ID: %w", err)
	}

	session.User = user
	return session, nil
}

// GetByTokenHash retrieves a session by token hash
func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	query := `
		SELECT s.id, s.user_id, s.token_hash, s.device_info, s.ip_address, s.expires_at, s.created_at,
			   u.id, u.email, u.name, u.avatar_url, u.is_active, u.is_admin, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token_hash = $1
	`

	session := &domain.Session{}
	user := &domain.User{}

	err := r.db.Pool.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.DeviceInfo,
		&session.IPAddress,
		&session.ExpiresAt,
		&session.CreatedAt,
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
		return nil, fmt.Errorf("failed to get session by token: %w", err)
	}

	session.User = user
	return session, nil
}

// Delete deletes a session
func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sessions WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// DeleteByUser deletes all sessions for a user
func (r *SessionRepository) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM sessions WHERE user_id = $1`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}

// DeleteExpired deletes all expired sessions
func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`

	_, err := r.db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	return nil
}
