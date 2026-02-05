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
	"github.com/gorilla/websocket"
)

// Client is the API client for the TUI
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Option is a client option
type Option func(*Client)

// WithToken sets the authentication token
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// New creates a new API client
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

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

// Team represents a team
type Team struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Role        string    `json:"role"`
}

// App represents an app
type App struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	RunCount    int       `json:"run_count"`
}

// Run represents a run
type Run struct {
	ID              uuid.UUID         `json:"id"`
	Name            string            `json:"name"`
	Status          string            `json:"status"`
	Tags            []string          `json:"tags"`
	Config          map[string]any    `json:"config"`
	StartedAt       time.Time         `json:"started_at"`
	EndedAt         *time.Time        `json:"ended_at"`
	DurationSeconds *float64          `json:"duration_seconds"`
	LatestMetrics   map[string]float64 `json:"latest_metrics"`
}

// Metric represents a metric data point
type Metric struct {
	Time  time.Time `json:"time"`
	Name  string    `json:"name"`
	Step  int64     `json:"step"`
	Value float64   `json:"value"`
}

// MetricUpdate represents a real-time metric update
type MetricUpdate struct {
	RunID     uuid.UUID `json:"run_id"`
	Name      string    `json:"name"`
	Step      int64     `json:"step"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
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

// GetTeams returns all teams for the authenticated user
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

// GetApps returns all apps for a team
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

// SubscribeMetrics connects to the WebSocket for real-time metric updates
func (c *Client) SubscribeMetrics(runID uuid.UUID) (<-chan MetricUpdate, error) {
	// Convert HTTP URL to WebSocket URL
	wsURL := c.baseURL
	if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}
	wsURL += fmt.Sprintf("/api/v1/ws/runs/%s/metrics", runID)

	// Add auth header
	header := http.Header{}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	ch := make(chan MetricUpdate, 100)

	go func() {
		defer close(ch)
		defer conn.Close()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var update MetricUpdate
			if err := json.Unmarshal(message, &update); err != nil {
				continue
			}

			select {
			case ch <- update:
			default:
				// Channel full, skip
			}
		}
	}()

	return ch, nil
}

// Login authenticates and returns tokens
func (c *Client) Login(email, password string) (string, error) {
	data, err := c.request("POST", "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	if err != nil {
		return "", err
	}

	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal login response: %w", err)
	}

	c.token = resp.AccessToken
	return resp.AccessToken, nil
}
