package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sixtyseven/internal/domain"
	"sixtyseven/internal/repository/postgres"
)

// TeamService handles team operations
type TeamService struct {
	teams *postgres.TeamRepository
	users *postgres.UserRepository
}

// NewTeamService creates a new team service
func NewTeamService(teams *postgres.TeamRepository, users *postgres.UserRepository) *TeamService {
	return &TeamService{
		teams: teams,
		users: users,
	}
}

// Create creates a new team
func (s *TeamService) Create(ctx context.Context, ownerID uuid.UUID, req domain.TeamCreate) (*domain.Team, error) {
	// Check if slug already exists
	existing, err := s.teams.GetBySlug(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing team: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("team with slug '%s' already exists", req.Slug)
	}

	team := &domain.Team{
		ID:          uuid.New(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Plan:        domain.TeamPlanFree,
		IsPersonal:  false,
		Settings:    map[string]any{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.teams.Create(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	// Add owner as team member
	member := &domain.TeamMember{
		ID:        uuid.New(),
		TeamID:    team.ID,
		UserID:    ownerID,
		Role:      domain.TeamRoleOwner,
		CreatedAt: time.Now(),
	}

	if err := s.teams.AddMember(ctx, member); err != nil {
		// Cleanup team if member addition fails
		_ = s.teams.Delete(ctx, team.ID)
		return nil, fmt.Errorf("failed to add owner to team: %w", err)
	}

	return team, nil
}

// GetByID retrieves a team by ID
func (s *TeamService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Team, error) {
	team, err := s.teams.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	return team, nil
}

// GetBySlug retrieves a team by slug
func (s *TeamService) GetBySlug(ctx context.Context, slug string) (*domain.Team, error) {
	team, err := s.teams.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	return team, nil
}

// Update updates a team
func (s *TeamService) Update(ctx context.Context, id uuid.UUID, req domain.TeamUpdate) (*domain.Team, error) {
	team, err := s.teams.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found")
	}

	if req.Name != nil {
		team.Name = *req.Name
	}
	if req.Description != nil {
		team.Description = req.Description
	}
	if req.Settings != nil {
		team.Settings = req.Settings
	}

	if err := s.teams.Update(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to update team: %w", err)
	}

	return team, nil
}

// Delete deletes a team
func (s *TeamService) Delete(ctx context.Context, id uuid.UUID) error {
	team, err := s.teams.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return fmt.Errorf("team not found")
	}

	// Prevent deletion of personal teams
	if team.IsPersonal {
		return fmt.Errorf("cannot delete personal team")
	}

	if err := s.teams.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}

// ListByUser retrieves all teams a user is a member of
func (s *TeamService) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.TeamWithRole, error) {
	teams, err := s.teams.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	return teams, nil
}

// AddMember adds a user to a team
func (s *TeamService) AddMember(ctx context.Context, teamID uuid.UUID, email string, role domain.TeamRole) (*domain.TeamMember, error) {
	// Find user by email
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Check if already a member
	existing, err := s.teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user is already a member of this team")
	}

	member := &domain.TeamMember{
		ID:        uuid.New(),
		TeamID:    teamID,
		UserID:    user.ID,
		Role:      role,
		CreatedAt: time.Now(),
	}

	if err := s.teams.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	member.User = user
	return member, nil
}

// UpdateMemberRole updates a team member's role
func (s *TeamService) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role domain.TeamRole) error {
	// Prevent removing last owner
	if role != domain.TeamRoleOwner {
		members, err := s.teams.ListMembers(ctx, teamID)
		if err != nil {
			return fmt.Errorf("failed to list members: %w", err)
		}

		ownerCount := 0
		for _, m := range members {
			if m.Role == domain.TeamRoleOwner && m.UserID != userID {
				ownerCount++
			}
		}

		if ownerCount == 0 {
			// Check if current user is the only owner
			member, _ := s.teams.GetMember(ctx, teamID, userID)
			if member != nil && member.Role == domain.TeamRoleOwner {
				return fmt.Errorf("cannot remove the last owner from the team")
			}
		}
	}

	if err := s.teams.UpdateMemberRole(ctx, teamID, userID, role); err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}
	return nil
}

// RemoveMember removes a user from a team
func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	// Prevent removing last owner
	members, err := s.teams.ListMembers(ctx, teamID)
	if err != nil {
		return fmt.Errorf("failed to list members: %w", err)
	}

	ownerCount := 0
	for _, m := range members {
		if m.Role == domain.TeamRoleOwner {
			ownerCount++
		}
	}

	member, _ := s.teams.GetMember(ctx, teamID, userID)
	if member != nil && member.Role == domain.TeamRoleOwner && ownerCount == 1 {
		return fmt.Errorf("cannot remove the last owner from the team")
	}

	if err := s.teams.RemoveMember(ctx, teamID, userID); err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	return nil
}

// ListMembers retrieves all members of a team
func (s *TeamService) ListMembers(ctx context.Context, teamID uuid.UUID) ([]*domain.TeamMember, error) {
	members, err := s.teams.ListMembers(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	return members, nil
}

// GetMember retrieves a team member
func (s *TeamService) GetMember(ctx context.Context, teamID, userID uuid.UUID) (*domain.TeamMember, error) {
	member, err := s.teams.GetMember(ctx, teamID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get member: %w", err)
	}
	return member, nil
}

// HasPermission checks if a user has at least the specified role in a team
func (s *TeamService) HasPermission(ctx context.Context, teamID, userID uuid.UUID, requiredRole domain.TeamRole) (bool, error) {
	member, err := s.teams.GetMember(ctx, teamID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get member: %w", err)
	}
	if member == nil {
		return false, nil
	}

	roleHierarchy := map[domain.TeamRole]int{
		domain.TeamRoleViewer: 1,
		domain.TeamRoleMember: 2,
		domain.TeamRoleAdmin:  3,
		domain.TeamRoleOwner:  4,
	}

	return roleHierarchy[member.Role] >= roleHierarchy[requiredRole], nil
}
