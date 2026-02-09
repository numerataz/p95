package messages

import (
	"github.com/google/uuid"
	"github.com/ninetyfive/sixtyseven/pkg/client"
)

// Navigation messages
type NavigateToTeamMsg struct {
	TeamSlug string
}

type NavigateToAppMsg struct {
	TeamSlug string
	AppSlug  string
}

type NavigateToRunMsg struct {
	RunID uuid.UUID
}

type NavigateBackMsg struct{}

// Data loading messages
type TeamsLoadedMsg struct {
	Teams []client.Team
}

type AppsLoadedMsg struct {
	Apps []client.App
}

// AppsRefreshedMsg is for silent background refresh of apps/projects list
type AppsRefreshedMsg struct {
	Apps []client.App
}

type RunsLoadedMsg struct {
	Runs []client.Run
}

// RunsRefreshedMsg is for silent background refresh of runs list
type RunsRefreshedMsg struct {
	Runs []client.Run
}

type RunLoadedMsg struct {
	Run         *client.Run
	MetricNames []string
}

type MetricSeriesLoadedMsg struct {
	MetricName string
	Points     []client.Metric
}

// ComparisonSeriesLoadedMsg contains metric data for a compared run
type ComparisonSeriesLoadedMsg struct {
	RunID      uuid.UUID
	RunName    string
	MetricName string
	Points     []client.Metric
}

type LatestMetricsLoadedMsg struct {
	Metrics map[string]float64
}

// ContinuationsLoadedMsg is sent when continuations are fetched
type ContinuationsLoadedMsg struct {
	Continuations []client.Continuation
}

// Tick message for periodic updates (polling-based)
type TickMsg struct{}

// Error message
type ErrorMsg struct {
	Err error
}

func (e ErrorMsg) Error() string {
	return e.Err.Error()
}

// Loading state
type LoadingMsg struct {
	Loading bool
}
