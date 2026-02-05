package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sixtyseven/internal/domain"
	"sixtyseven/internal/repository/postgres"
)

// AppService handles app operations
type AppService struct {
	apps  *postgres.AppRepository
	teams *postgres.TeamRepository
}

// NewAppService creates a new app service
func NewAppService(apps *postgres.AppRepository, teams *postgres.TeamRepository) *AppService {
	return &AppService{
		apps:  apps,
		teams: teams,
	}
}

// Create creates a new app
func (s *AppService) Create(ctx context.Context, teamID uuid.UUID, req domain.AppCreate) (*domain.App, error) {
	// Verify team exists
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found")
	}

	// Check if slug already exists in team
	existing, err := s.apps.GetBySlug(ctx, teamID, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing app: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("app with slug '%s' already exists in this team", req.Slug)
	}

	// Set default visibility
	visibility := req.Visibility
	if visibility == "" {
		visibility = domain.AppVisibilityPrivate
	}

	app := &domain.App{
		ID:          uuid.New(),
		TeamID:      teamID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Visibility:  visibility,
		Settings:    map[string]any{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.apps.Create(ctx, app); err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	return app, nil
}

// GetByID retrieves an app by ID
func (s *AppService) GetByID(ctx context.Context, id uuid.UUID) (*domain.App, error) {
	app, err := s.apps.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	return app, nil
}

// GetBySlug retrieves an app by team ID and slug
func (s *AppService) GetBySlug(ctx context.Context, teamID uuid.UUID, slug string) (*domain.App, error) {
	app, err := s.apps.GetBySlug(ctx, teamID, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	return app, nil
}

// Update updates an app
func (s *AppService) Update(ctx context.Context, id uuid.UUID, req domain.AppUpdate) (*domain.App, error) {
	app, err := s.apps.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	if app == nil {
		return nil, fmt.Errorf("app not found")
	}

	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.Description != nil {
		app.Description = req.Description
	}
	if req.Visibility != nil {
		app.Visibility = *req.Visibility
	}
	if req.Settings != nil {
		app.Settings = req.Settings
	}

	if err := s.apps.Update(ctx, app); err != nil {
		return nil, fmt.Errorf("failed to update app: %w", err)
	}

	return app, nil
}

// Archive archives an app
func (s *AppService) Archive(ctx context.Context, id uuid.UUID) error {
	if err := s.apps.Archive(ctx, id); err != nil {
		return fmt.Errorf("failed to archive app: %w", err)
	}
	return nil
}

// Delete deletes an app
func (s *AppService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.apps.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}
	return nil
}

// List retrieves apps with filtering options
func (s *AppService) List(ctx context.Context, opts domain.AppListOptions) ([]*domain.App, error) {
	apps, err := s.apps.List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}
	return apps, nil
}
