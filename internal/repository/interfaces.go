package repository

import (
	"context"

	"github.com/google/uuid"
	"sixtyseven/internal/domain"
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*domain.User, error)
}

// TeamRepository defines the interface for team data access
type TeamRepository interface {
	Create(ctx context.Context, team *domain.Team) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Team, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Team, error)
	Update(ctx context.Context, team *domain.Team) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.TeamWithRole, error)

	// Team membership
	AddMember(ctx context.Context, member *domain.TeamMember) error
	GetMember(ctx context.Context, teamID, userID uuid.UUID) (*domain.TeamMember, error)
	UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role domain.TeamRole) error
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
	ListMembers(ctx context.Context, teamID uuid.UUID) ([]*domain.TeamMember, error)
}

// AppRepository defines the interface for app data access
type AppRepository interface {
	Create(ctx context.Context, app *domain.App) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.App, error)
	GetBySlug(ctx context.Context, teamID uuid.UUID, slug string) (*domain.App, error)
	Update(ctx context.Context, app *domain.App) error
	Delete(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, opts domain.AppListOptions) ([]*domain.App, error)
	CountByTeam(ctx context.Context, teamID uuid.UUID) (int, error)
}

// RunRepository defines the interface for run data access
type RunRepository interface {
	Create(ctx context.Context, run *domain.Run) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Run, error)
	Update(ctx context.Context, run *domain.Run) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.RunStatus, errorMsg *string) error
	UpdateConfig(ctx context.Context, id uuid.UUID, config map[string]any) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, opts domain.RunListOptions) ([]*domain.Run, error)
	CountByApp(ctx context.Context, appID uuid.UUID) (int, error)
	GetActiveByApp(ctx context.Context, appID uuid.UUID) ([]*domain.Run, error)
}

// MetricsRepository defines the interface for metrics data access
type MetricsRepository interface {
	BatchInsert(ctx context.Context, runID uuid.UUID, metrics []domain.MetricPoint) error
	GetSeries(ctx context.Context, opts domain.MetricQueryOptions) ([]domain.Metric, error)
	GetLatest(ctx context.Context, runID uuid.UUID) (map[string]float64, error)
	GetMetricNames(ctx context.Context, runID uuid.UUID) ([]string, error)
	GetSummary(ctx context.Context, runID uuid.UUID) (*domain.RunMetricsSummary, error)
	DeleteByRun(ctx context.Context, runID uuid.UUID) error
}

// APIKeyRepository defines the interface for API key data access
type APIKeyRepository interface {
	Create(ctx context.Context, key *domain.APIKey) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)
	GetByPrefix(ctx context.Context, prefix string) ([]*domain.APIKey, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error)
}

// SessionRepository defines the interface for session data access
type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Session, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// ArtifactRepository defines the interface for artifact data access
type ArtifactRepository interface {
	Create(ctx context.Context, artifact *domain.Artifact) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByRun(ctx context.Context, runID uuid.UUID, opts domain.ArtifactListOptions) ([]*domain.Artifact, error)
}

// ContinuationRepository defines the interface for continuation data access
type ContinuationRepository interface {
	Create(ctx context.Context, continuation *domain.Continuation) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Continuation, error)
	ListByRun(ctx context.Context, runID uuid.UUID) ([]*domain.Continuation, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByRun(ctx context.Context, runID uuid.UUID) error
}
