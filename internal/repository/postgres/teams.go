package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sixtyseven/internal/domain"
)

// TeamRepository implements the team repository interface
type TeamRepository struct {
	db *DB
}

// NewTeamRepository creates a new team repository
func NewTeamRepository(db *DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create creates a new team
func (r *TeamRepository) Create(ctx context.Context, team *domain.Team) error {
	query := `
		INSERT INTO teams (id, name, slug, description, plan, is_personal, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	if team.ID == uuid.Nil {
		team.ID = uuid.New()
	}

	_, err := r.db.Pool.Exec(ctx, query,
		team.ID,
		team.Name,
		team.Slug,
		team.Description,
		team.Plan,
		team.IsPersonal,
		team.Settings,
		team.CreatedAt,
		team.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	return nil
}

// GetByID retrieves a team by ID
func (r *TeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Team, error) {
	query := `
		SELECT id, name, slug, description, plan, is_personal, settings, created_at, updated_at
		FROM teams
		WHERE id = $1
	`

	team := &domain.Team{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.Slug,
		&team.Description,
		&team.Plan,
		&team.IsPersonal,
		&team.Settings,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team by ID: %w", err)
	}

	return team, nil
}

// GetBySlug retrieves a team by slug
func (r *TeamRepository) GetBySlug(ctx context.Context, slug string) (*domain.Team, error) {
	query := `
		SELECT id, name, slug, description, plan, is_personal, settings, created_at, updated_at
		FROM teams
		WHERE slug = $1
	`

	team := &domain.Team{}
	err := r.db.Pool.QueryRow(ctx, query, slug).Scan(
		&team.ID,
		&team.Name,
		&team.Slug,
		&team.Description,
		&team.Plan,
		&team.IsPersonal,
		&team.Settings,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team by slug: %w", err)
	}

	return team, nil
}

// Update updates a team
func (r *TeamRepository) Update(ctx context.Context, team *domain.Team) error {
	query := `
		UPDATE teams
		SET name = $2, description = $3, plan = $4, settings = $5, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		team.ID,
		team.Name,
		team.Description,
		team.Plan,
		team.Settings,
	)

	if err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("team not found")
	}

	return nil
}

// Delete deletes a team
func (r *TeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM teams WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("team not found")
	}

	return nil
}

// ListByUser retrieves all teams a user is a member of
func (r *TeamRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.TeamWithRole, error) {
	query := `
		SELECT t.id, t.name, t.slug, t.description, t.plan, t.is_personal, t.settings, t.created_at, t.updated_at, tm.role
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.name
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams by user: %w", err)
	}
	defer rows.Close()

	var teams []*domain.TeamWithRole
	for rows.Next() {
		team := &domain.TeamWithRole{}
		err := rows.Scan(
			&team.ID,
			&team.Name,
			&team.Slug,
			&team.Description,
			&team.Plan,
			&team.IsPersonal,
			&team.Settings,
			&team.CreatedAt,
			&team.UpdatedAt,
			&team.Role,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		teams = append(teams, team)
	}

	return teams, nil
}

// AddMember adds a user to a team
func (r *TeamRepository) AddMember(ctx context.Context, member *domain.TeamMember) error {
	query := `
		INSERT INTO team_members (id, team_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	if member.ID == uuid.Nil {
		member.ID = uuid.New()
	}

	_, err := r.db.Pool.Exec(ctx, query,
		member.ID,
		member.TeamID,
		member.UserID,
		member.Role,
		member.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}

	return nil
}

// GetMember retrieves a team member
func (r *TeamRepository) GetMember(ctx context.Context, teamID, userID uuid.UUID) (*domain.TeamMember, error) {
	query := `
		SELECT id, team_id, user_id, role, created_at
		FROM team_members
		WHERE team_id = $1 AND user_id = $2
	`

	member := &domain.TeamMember{}
	err := r.db.Pool.QueryRow(ctx, query, teamID, userID).Scan(
		&member.ID,
		&member.TeamID,
		&member.UserID,
		&member.Role,
		&member.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team member: %w", err)
	}

	return member, nil
}

// UpdateMemberRole updates a team member's role
func (r *TeamRepository) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role domain.TeamRole) error {
	query := `
		UPDATE team_members
		SET role = $3
		WHERE team_id = $1 AND user_id = $2
	`

	result, err := r.db.Pool.Exec(ctx, query, teamID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("team member not found")
	}

	return nil
}

// RemoveMember removes a user from a team
func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`

	result, err := r.db.Pool.Exec(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("team member not found")
	}

	return nil
}

// ListMembers retrieves all members of a team
func (r *TeamRepository) ListMembers(ctx context.Context, teamID uuid.UUID) ([]*domain.TeamMember, error) {
	query := `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.created_at,
			   u.id, u.email, u.name, u.avatar_url, u.is_active, u.is_admin, u.created_at, u.updated_at
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = $1
		ORDER BY tm.created_at
	`

	rows, err := r.db.Pool.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list team members: %w", err)
	}
	defer rows.Close()

	var members []*domain.TeamMember
	for rows.Next() {
		member := &domain.TeamMember{}
		user := &domain.User{}
		err := rows.Scan(
			&member.ID,
			&member.TeamID,
			&member.UserID,
			&member.Role,
			&member.CreatedAt,
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
			return nil, fmt.Errorf("failed to scan team member: %w", err)
		}
		member.User = user
		members = append(members, member)
	}

	return members, nil
}
