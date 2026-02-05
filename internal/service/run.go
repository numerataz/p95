package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sixtyseven/internal/domain"
	"sixtyseven/internal/repository/postgres"
)

// RunService handles run operations
type RunService struct {
	runs          *postgres.RunRepository
	apps          *postgres.AppRepository
	metrics       *postgres.MetricsRepository
	continuations *postgres.ContinuationRepository
}

// NewRunService creates a new run service
func NewRunService(runs *postgres.RunRepository, apps *postgres.AppRepository, metrics *postgres.MetricsRepository, continuations *postgres.ContinuationRepository) *RunService {
	return &RunService{
		runs:          runs,
		apps:          apps,
		metrics:       metrics,
		continuations: continuations,
	}
}

// Create creates a new run
func (s *RunService) Create(ctx context.Context, appID, userID uuid.UUID, req domain.RunCreate) (*domain.Run, error) {
	// Verify app exists
	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	if app == nil {
		return nil, fmt.Errorf("app not found")
	}

	// Generate name if not provided
	name := req.Name
	if name == "" {
		name = generateRunName()
	}

	run := &domain.Run{
		ID:         uuid.New(),
		AppID:      appID,
		UserID:     userID,
		Name:       name,
		Description: req.Description,
		Status:     domain.RunStatusRunning,
		Tags:       req.Tags,
		GitInfo:    req.GitInfo,
		SystemInfo: req.SystemInfo,
		Config:     req.Config,
		StartedAt:  time.Now(),
		CreatedAt:  time.Now(),
	}

	if err := s.runs.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}

	return run, nil
}

// GetByID retrieves a run by ID
func (s *RunService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	run, err := s.runs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	return run, nil
}

// Update updates a run
func (s *RunService) Update(ctx context.Context, id uuid.UUID, req domain.RunUpdate) (*domain.Run, error) {
	run, err := s.runs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}

	if req.Name != nil {
		run.Name = *req.Name
	}
	if req.Description != nil {
		run.Description = req.Description
	}
	if req.Tags != nil {
		run.Tags = req.Tags
	}
	if req.Config != nil {
		// Merge configs
		if run.Config == nil {
			run.Config = make(map[string]any)
		}
		for k, v := range req.Config {
			run.Config[k] = v
		}
	}

	if err := s.runs.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update run: %w", err)
	}

	return run, nil
}

// UpdateStatus updates a run's status
func (s *RunService) UpdateStatus(ctx context.Context, id uuid.UUID, req domain.RunStatusUpdate) (*domain.Run, error) {
	run, err := s.runs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}

	if err := s.runs.UpdateStatus(ctx, id, req.Status, req.ErrorMessage); err != nil {
		return nil, fmt.Errorf("failed to update run status: %w", err)
	}

	// Refetch to get updated fields
	return s.runs.GetByID(ctx, id)
}

// UpdateConfig merges new config with existing config
func (s *RunService) UpdateConfig(ctx context.Context, id uuid.UUID, config map[string]any) error {
	if err := s.runs.UpdateConfig(ctx, id, config); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}
	return nil
}

// Delete deletes a run and its metrics
func (s *RunService) Delete(ctx context.Context, id uuid.UUID) error {
	// Delete metrics first
	if err := s.metrics.DeleteByRun(ctx, id); err != nil {
		return fmt.Errorf("failed to delete run metrics: %w", err)
	}

	// Delete run
	if err := s.runs.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}

	return nil
}

// List retrieves runs with filtering options
func (s *RunService) List(ctx context.Context, opts domain.RunListOptions) ([]*domain.Run, error) {
	runs, err := s.runs.List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	return runs, nil
}

// GetWithMetrics retrieves a run with its latest metrics
func (s *RunService) GetWithMetrics(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	run, err := s.runs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	if run == nil {
		return nil, nil
	}

	// Get latest metrics
	latestMetrics, err := s.metrics.GetLatest(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest metrics: %w", err)
	}
	run.LatestMetrics = latestMetrics

	return run, nil
}

// generateRunName generates a unique run name
func generateRunName() string {
	adjectives := []string{"swift", "bright", "calm", "bold", "keen", "wise", "pure", "warm"}
	nouns := []string{"falcon", "river", "forest", "peak", "star", "wave", "cloud", "dawn"}

	adj := adjectives[time.Now().UnixNano()%int64(len(adjectives))]
	noun := nouns[time.Now().UnixNano()/7%int64(len(nouns))]

	return fmt.Sprintf("%s-%s-%s", adj, noun, time.Now().Format("20060102-150405"))
}

// ResumeRun resumes a completed/failed run by setting it back to running status
// and recording a continuation event with config changes
func (s *RunService) ResumeRun(ctx context.Context, id uuid.UUID, req domain.ResumeRunRequest) (*domain.ResumeRunResponse, error) {
	// Get existing run
	run, err := s.runs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}

	// Verify run is not currently running
	if run.Status == domain.RunStatusRunning {
		return nil, fmt.Errorf("run is already running")
	}

	// Get current step from the last metric
	summary, err := s.metrics.GetSummary(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics summary: %w", err)
	}

	var currentStep int64 = 0
	if summary != nil && len(summary.Metrics) > 0 {
		// Find the max step across all metrics
		for _, m := range summary.Metrics {
			if m.LastStep > currentStep {
				currentStep = m.LastStep
			}
		}
	}

	// Capture config before
	configBefore := make(map[string]any)
	for k, v := range run.Config {
		configBefore[k] = v
	}

	// Merge new config with existing
	configAfter := make(map[string]any)
	for k, v := range run.Config {
		configAfter[k] = v
	}
	if req.Config != nil {
		for k, v := range req.Config {
			configAfter[k] = v
		}
	}

	// Create continuation record
	continuation := &domain.Continuation{
		ID:           uuid.New(),
		RunID:        id,
		Step:         currentStep,
		Timestamp:    time.Now(),
		ConfigBefore: configBefore,
		ConfigAfter:  configAfter,
		Note:         req.Note,
		GitInfo:      req.GitInfo,
		SystemInfo:   req.SystemInfo,
		CreatedAt:    time.Now(),
	}

	if err := s.continuations.Create(ctx, continuation); err != nil {
		return nil, fmt.Errorf("failed to create continuation: %w", err)
	}

	// Update run status back to running
	if err := s.runs.UpdateStatus(ctx, id, domain.RunStatusRunning, nil); err != nil {
		return nil, fmt.Errorf("failed to update run status: %w", err)
	}

	// Update run config if changed
	if req.Config != nil {
		if err := s.runs.UpdateConfig(ctx, id, req.Config); err != nil {
			return nil, fmt.Errorf("failed to update run config: %w", err)
		}
	}

	// Refetch the updated run
	updatedRun, err := s.runs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated run: %w", err)
	}

	return &domain.ResumeRunResponse{
		Run:          updatedRun,
		Continuation: continuation,
	}, nil
}

// GetContinuations retrieves all continuations for a run
func (s *RunService) GetContinuations(ctx context.Context, runID uuid.UUID) ([]*domain.Continuation, error) {
	// Verify run exists
	run, err := s.runs.GetByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}

	continuations, err := s.continuations.ListByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get continuations: %w", err)
	}

	return continuations, nil
}
