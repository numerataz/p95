package client

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	localTeamSlug  = "__local__"
	remoteTeamPref = "remote:"
)

// API is the minimal interface used by the TUI.
type API interface {
	GetTeams() ([]Team, error)
	GetApps(teamSlug string) ([]App, error)
	GetRuns(teamSlug, appSlug string) ([]Run, error)
	GetRun(runID uuid.UUID, includeMetrics bool) (*Run, error)
	GetMetricSeries(runID uuid.UUID, metricName string, maxPoints int) ([]Metric, error)
	GetLatestMetrics(runID uuid.UUID) (map[string]float64, error)
	GetMetricNames(runID uuid.UUID) ([]string, error)
	GetContinuations(runID uuid.UUID) ([]Continuation, error)
}

type runSource int

const (
	runSourceRemote runSource = iota
	runSourceLocal
)

// CompositeClient merges remote service data and local data for the TUI.
type CompositeClient struct {
	local  API
	remote API

	mu        sync.RWMutex
	runOwners map[uuid.UUID]runSource
}

// NewComposite creates a client that can browse both remote and local data.
func NewComposite(local API, remote API) *CompositeClient {
	return &CompositeClient{
		local:     local,
		remote:    remote,
		runOwners: make(map[uuid.UUID]runSource),
	}
}

func isRemoteTeamSlug(slug string) bool {
	return strings.HasPrefix(slug, remoteTeamPref)
}

func encodeRemoteTeamSlug(slug string) string {
	return remoteTeamPref + slug
}

func decodeRemoteTeamSlug(slug string) string {
	return strings.TrimPrefix(slug, remoteTeamPref)
}

func (c *CompositeClient) teamSource(teamSlug string) runSource {
	if teamSlug == localTeamSlug {
		return runSourceLocal
	}
	if isRemoteTeamSlug(teamSlug) {
		return runSourceRemote
	}
	if c.remote != nil {
		return runSourceRemote
	}
	return runSourceLocal
}

func (c *CompositeClient) GetTeams() ([]Team, error) {
	teams := make([]Team, 0, 4)

	if c.remote != nil {
		remoteTeams, err := c.remote.GetTeams()
		if err == nil {
			for _, t := range remoteTeams {
				t.Slug = encodeRemoteTeamSlug(t.Slug)
				teams = append(teams, t)
			}
		}
	}

	teams = append(teams, Team{
		ID:          uuid.Nil,
		Name:        "Local",
		Slug:        localTeamSlug,
		Description: "Local runs",
		Role:        "owner",
	})

	return teams, nil
}

func (c *CompositeClient) GetApps(teamSlug string) ([]App, error) {
	src := c.teamSource(teamSlug)
	switch src {
	case runSourceRemote:
		if c.remote == nil {
			return nil, fmt.Errorf("remote client not configured")
		}
		return c.remote.GetApps(decodeRemoteTeamSlug(teamSlug))
	default:
		if c.local == nil {
			return nil, fmt.Errorf("local client not configured")
		}
		return c.local.GetApps("local")
	}
}

func (c *CompositeClient) GetRuns(teamSlug, appSlug string) ([]Run, error) {
	src := c.teamSource(teamSlug)

	var (
		runs []Run
		err  error
	)

	switch src {
	case runSourceRemote:
		if c.remote == nil {
			return nil, fmt.Errorf("remote client not configured")
		}
		runs, err = c.remote.GetRuns(decodeRemoteTeamSlug(teamSlug), appSlug)
	case runSourceLocal:
		if c.local == nil {
			return nil, fmt.Errorf("local client not configured")
		}
		runs, err = c.local.GetRuns("local", appSlug)
	}
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	for _, r := range runs {
		c.runOwners[r.ID] = src
	}
	c.mu.Unlock()

	return runs, nil
}

func (c *CompositeClient) sourceForRun(runID uuid.UUID) (runSource, bool) {
	c.mu.RLock()
	src, ok := c.runOwners[runID]
	c.mu.RUnlock()
	return src, ok
}

func (c *CompositeClient) GetRun(runID uuid.UUID, includeMetrics bool) (*Run, error) {
	if src, ok := c.sourceForRun(runID); ok {
		if src == runSourceRemote {
			return c.remote.GetRun(runID, includeMetrics)
		}
		return c.local.GetRun(runID, includeMetrics)
	}

	if c.remote != nil {
		run, err := c.remote.GetRun(runID, includeMetrics)
		if err == nil {
			return run, nil
		}
	}
	if c.local != nil {
		return c.local.GetRun(runID, includeMetrics)
	}
	return nil, fmt.Errorf("no client configured")
}

func (c *CompositeClient) GetMetricSeries(runID uuid.UUID, metricName string, maxPoints int) ([]Metric, error) {
	if src, ok := c.sourceForRun(runID); ok {
		if src == runSourceRemote {
			return c.remote.GetMetricSeries(runID, metricName, maxPoints)
		}
		return c.local.GetMetricSeries(runID, metricName, maxPoints)
	}

	if c.remote != nil {
		points, err := c.remote.GetMetricSeries(runID, metricName, maxPoints)
		if err == nil {
			return points, nil
		}
	}
	if c.local != nil {
		return c.local.GetMetricSeries(runID, metricName, maxPoints)
	}
	return nil, fmt.Errorf("no client configured")
}

func (c *CompositeClient) GetLatestMetrics(runID uuid.UUID) (map[string]float64, error) {
	if src, ok := c.sourceForRun(runID); ok {
		if src == runSourceRemote {
			return c.remote.GetLatestMetrics(runID)
		}
		return c.local.GetLatestMetrics(runID)
	}

	if c.remote != nil {
		metrics, err := c.remote.GetLatestMetrics(runID)
		if err == nil {
			return metrics, nil
		}
	}
	if c.local != nil {
		return c.local.GetLatestMetrics(runID)
	}
	return nil, fmt.Errorf("no client configured")
}

func (c *CompositeClient) GetMetricNames(runID uuid.UUID) ([]string, error) {
	if src, ok := c.sourceForRun(runID); ok {
		if src == runSourceRemote {
			return c.remote.GetMetricNames(runID)
		}
		return c.local.GetMetricNames(runID)
	}

	if c.remote != nil {
		names, err := c.remote.GetMetricNames(runID)
		if err == nil {
			return names, nil
		}
	}
	if c.local != nil {
		return c.local.GetMetricNames(runID)
	}
	return nil, fmt.Errorf("no client configured")
}

func (c *CompositeClient) GetContinuations(runID uuid.UUID) ([]Continuation, error) {
	if src, ok := c.sourceForRun(runID); ok {
		if src == runSourceRemote {
			return c.remote.GetContinuations(runID)
		}
		return c.local.GetContinuations(runID)
	}

	if c.remote != nil {
		continuations, err := c.remote.GetContinuations(runID)
		if err == nil {
			return continuations, nil
		}
	}
	if c.local != nil {
		return c.local.GetContinuations(runID)
	}
	return nil, fmt.Errorf("no client configured")
}
