package domain

import (
	"time"

	"github.com/google/uuid"
)

// TeamRole represents a user's role within a team
type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleMember TeamRole = "member"
	TeamRoleViewer TeamRole = "viewer"
)

// TeamPlan represents the subscription plan for a team
type TeamPlan string

const (
	TeamPlanFree       TeamPlan = "free"
	TeamPlanPro        TeamPlan = "pro"
	TeamPlanEnterprise TeamPlan = "enterprise"
)

// Team represents an organization or workspace
type Team struct {
	ID          uuid.UUID         `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Description *string           `json:"description,omitempty"`
	Plan        TeamPlan          `json:"plan"`
	IsPersonal  bool              `json:"is_personal"`
	Settings    map[string]any    `json:"settings,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TeamMember represents a user's membership in a team
type TeamMember struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	UserID    uuid.UUID `json:"user_id"`
	Role      TeamRole  `json:"role"`
	CreatedAt time.Time `json:"created_at"`

	// Populated by joins
	User *User `json:"user,omitempty"`
	Team *Team `json:"team,omitempty"`
}

// TeamCreate contains fields for creating a new team
type TeamCreate struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Slug        string  `json:"slug" validate:"required,min=1,max=255,slug"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
}

// TeamUpdate contains fields for updating a team
type TeamUpdate struct {
	Name        *string        `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string        `json:"description,omitempty" validate:"omitempty,max=1000"`
	Settings    map[string]any `json:"settings,omitempty"`
}

// TeamInvite contains fields for inviting a user to a team
type TeamInvite struct {
	Email string   `json:"email" validate:"required,email"`
	Role  TeamRole `json:"role" validate:"required,oneof=admin member viewer"`
}

// TeamWithRole represents a team with the user's role in it
type TeamWithRole struct {
	Team
	Role TeamRole `json:"role"`
}
