package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Client is the API client for the TUI
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new API client
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// request makes an HTTP request
func (c *Client) request(method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %s (status %d)", string(respBody), resp.StatusCode)
	}

	return respBody, nil
}

// Team represents a team (local mode: single "Local" team)
type Team struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Role        string    `json:"role"`
}

// App represents an app (local mode: maps to project)
type App struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	RunCount    int       `json:"run_count"`
}

// inactiveThreshold is how long a "running" run must go without receiving
// metric data before it is considered inactive (likely crashed).
const inactiveThreshold = 5 * time.Minute

// Run represents a run
type Run struct {
	ID              uuid.UUID          `json:"id"`
	Name            string             `json:"name"`
	Status          string             `json:"status"`
	Tags            []string           `json:"tags"`
	Config          map[string]any     `json:"config"`
	StartedAt       time.Time          `json:"started_at"`
	EndedAt         *time.Time         `json:"ended_at"`
	DurationSeconds *float64           `json:"duration_seconds"`
	LatestMetrics   map[string]float64 `json:"latest_metrics"`
	LastMetricTime  *time.Time         `json:"last_metric_time,omitempty"`
}

// IsInactive returns true if the run appears to have crashed: it is still
// marked "running" but has not received any metric data for inactiveThreshold.
func (r *Run) IsInactive() bool {
	if r.Status != "running" {
		return false
	}
	if time.Since(r.StartedAt) < inactiveThreshold {
		return false
	}
	if r.LastMetricTime == nil {
		// No metrics ever logged; treat as inactive after the threshold
		return true
	}
	return time.Since(*r.LastMetricTime) > inactiveThreshold
}

// Metric represents a metric data point
type Metric struct {
	Time  time.Time `json:"time"`
	Name  string    `json:"name"`
	Step  int64     `json:"step"`
	Value float64   `json:"value"`
}

// Continuation represents a point where the run was resumed
type Continuation struct {
	ID           uuid.UUID      `json:"id"`
	RunID        uuid.UUID      `json:"run_id"`
	Step         int64          `json:"step"`
	Timestamp    time.Time      `json:"timestamp"`
	ConfigBefore map[string]any `json:"config_before,omitempty"`
	ConfigAfter  map[string]any `json:"config_after,omitempty"`
	Note         *string        `json:"note,omitempty"`
}

// GetTeams returns all teams (local mode returns single "Local" team)
func (c *Client) GetTeams() ([]Team, error) {
	data, err := c.request("GET", "/api/v1/teams", nil)
	if err != nil {
		return nil, err
	}

	var teams []Team
	if err := json.Unmarshal(data, &teams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal teams: %w", err)
	}

	return teams, nil
}

// GetApps returns all apps for a team (local mode: returns projects)
func (c *Client) GetApps(teamSlug string) ([]App, error) {
	data, err := c.request("GET", fmt.Sprintf("/api/v1/teams/%s/apps", teamSlug), nil)
	if err != nil {
		return nil, err
	}

	var apps []App
	if err := json.Unmarshal(data, &apps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal apps: %w", err)
	}

	return apps, nil
}

// GetRuns returns all runs for an app
func (c *Client) GetRuns(teamSlug, appSlug string) ([]Run, error) {
	data, err := c.request("GET", fmt.Sprintf("/api/v1/teams/%s/apps/%s/runs", teamSlug, appSlug), nil)
	if err != nil {
		return nil, err
	}

	var runs []Run
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal runs: %w", err)
	}

	return runs, nil
}

// GetRun returns a single run with optional metrics
func (c *Client) GetRun(runID uuid.UUID, includeMetrics bool) (*Run, error) {
	path := fmt.Sprintf("/api/v1/runs/%s", runID)
	if includeMetrics {
		path += "?include_metrics=true"
	}

	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("failed to unmarshal run: %w", err)
	}

	return &run, nil
}

// GetMetricSeries returns the time series for a metric
func (c *Client) GetMetricSeries(runID uuid.UUID, metricName string, maxPoints int) ([]Metric, error) {
	path := fmt.Sprintf("/api/v1/runs/%s/metrics/%s", runID, url.PathEscape(metricName))
	if maxPoints > 0 {
		path += fmt.Sprintf("?max_points=%d", maxPoints)
	}

	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Metric string   `json:"metric"`
		Points []Metric `json:"points"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return resp.Points, nil
}

// GetLatestMetrics returns the latest value for each metric
func (c *Client) GetLatestMetrics(runID uuid.UUID) (map[string]float64, error) {
	data, err := c.request("GET", fmt.Sprintf("/api/v1/runs/%s/metrics/latest", runID), nil)
	if err != nil {
		return nil, err
	}

	var metrics map[string]float64
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return metrics, nil
}

// GetMetricNames returns all metric names for a run
func (c *Client) GetMetricNames(runID uuid.UUID) ([]string, error) {
	data, err := c.request("GET", fmt.Sprintf("/api/v1/runs/%s/metrics", runID), nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Metrics []string `json:"metrics"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metric names: %w", err)
	}

	return resp.Metrics, nil
}

// GetContinuations returns all continuations for a run
func (c *Client) GetContinuations(runID uuid.UUID) ([]Continuation, error) {
	data, err := c.request("GET", fmt.Sprintf("/api/v1/runs/%s/continuations", runID), nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Continuations []Continuation `json:"continuations"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal continuations: %w", err)
	}

	return resp.Continuations, nil
}
