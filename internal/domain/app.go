package domain

import (
	"time"

	"github.com/google/uuid"
)

// AppVisibility represents the visibility level of an app
type AppVisibility string

const (
	AppVisibilityPrivate AppVisibility = "private"
	AppVisibilityTeam    AppVisibility = "team"
	AppVisibilityPublic  AppVisibility = "public"
)

// App represents a project/application within a team
type App struct {
	ID          uuid.UUID         `json:"id"`
	TeamID      uuid.UUID         `json:"team_id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Description *string           `json:"description,omitempty"`
	Visibility  AppVisibility     `json:"visibility"`
	Settings    map[string]any    `json:"settings,omitempty"`
	ArchivedAt  *time.Time        `json:"archived_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`

	// Populated by joins
	Team *Team `json:"team,omitempty"`

	// Computed fields
	RunCount int `json:"run_count,omitempty"`
}

// AppCreate contains fields for creating a new app
type AppCreate struct {
	Name        string        `json:"name" validate:"required,min=1,max=255"`
	Slug        string        `json:"slug" validate:"required,min=1,max=255,slug"`
	Description *string       `json:"description,omitempty" validate:"omitempty,max=1000"`
	Visibility  AppVisibility `json:"visibility,omitempty" validate:"omitempty,oneof=private team public"`
}

// AppUpdate contains fields for updating an app
type AppUpdate struct {
	Name        *string        `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string        `json:"description,omitempty" validate:"omitempty,max=1000"`
	Visibility  *AppVisibility `json:"visibility,omitempty" validate:"omitempty,oneof=private team public"`
	Settings    map[string]any `json:"settings,omitempty"`
}

// AppListOptions contains options for listing apps
type AppListOptions struct {
	TeamID      uuid.UUID
	IncludeArchived bool
	Limit       int
	Offset      int
}

// IsArchived returns true if the app is archived
func (a *App) IsArchived() bool {
	return a.ArchivedAt != nil
}
