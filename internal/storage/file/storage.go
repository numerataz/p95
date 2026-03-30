// Package file provides a file-based storage implementation using SQLite.
package file

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/ninetyfive/p95/internal/domain"
	"github.com/ninetyfive/p95/internal/storage"
)

// Storage implements the storage.Storage interface using local files.
type Storage struct {
	logdir string
	mu     sync.RWMutex

	// Cache of open SQLite connections (one per run)
	dbCache   map[string]*sql.DB
	dbCacheMu sync.RWMutex
}

// New creates a new file-based storage instance.
func New(logdir string) (*Storage, error) {
	// Expand ~ and ensure directory exists
	if strings.HasPrefix(logdir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		logdir = filepath.Join(home, logdir[1:])
	}

	if err := os.MkdirAll(logdir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logdir: %w", err)
	}

	return &Storage{
		logdir:  logdir,
		dbCache: make(map[string]*sql.DB),
	}, nil
}

// LogDir returns the log directory path.
func (s *Storage) LogDir() string {
	return s.logdir
}

// ListProjects returns all projects in the log directory.
func (s *Storage) ListProjects(ctx context.Context) ([]storage.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.logdir)
	if err != nil {
		return nil, fmt.Errorf("failed to read logdir: %w", err)
	}

	var projects []storage.Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		projectDir := filepath.Join(s.logdir, entry.Name())
		runCount := 0
		var lastUpdated time.Time

		// Count runs and find latest update
		runEntries, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, runEntry := range runEntries {
			if !runEntry.IsDir() {
				continue
			}
			runCount++

			// Check meta.json for update time
			metaPath := filepath.Join(projectDir, runEntry.Name(), "meta.json")
			if info, err := os.Stat(metaPath); err == nil {
				if info.ModTime().After(lastUpdated) {
					lastUpdated = info.ModTime()
				}
			}
		}

		if runCount > 0 {
			projects = append(projects, storage.Project{
				Slug:        entry.Name(),
				Name:        entry.Name(),
				RunCount:    runCount,
				LastUpdated: lastUpdated,
			})
		}
	}

	// Sort by last updated (newest first)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastUpdated.After(projects[j].LastUpdated)
	})

	return projects, nil
}

// ListRuns returns all runs in a project.
func (s *Storage) ListRuns(ctx context.Context, project string, opts domain.RunListOptions) ([]*domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	projectDir := filepath.Join(s.logdir, project)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Run{}, nil
		}
		return nil, fmt.Errorf("failed to read project directory: %w", err)
	}

	var runs []*domain.Run
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		run, err := s.loadRunMeta(project, entry.Name())
		if err != nil {
			continue // Skip invalid runs
		}

		// Apply status filter
		if opts.Status != nil && run.Status != *opts.Status {
			continue
		}

		// Apply tag filter
		if len(opts.Tags) > 0 {
			hasAllTags := true
			for _, tag := range opts.Tags {
				found := false
				for _, runTag := range run.Tags {
					if runTag == tag {
						found = true
						break
					}
				}
				if !found {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}

		runs = append(runs, run)
	}

	// Sort runs
	orderBy := opts.OrderBy
	if orderBy == "" {
		orderBy = "started_at"
	}
	orderDir := opts.OrderDir
	if orderDir == "" {
		orderDir = "desc"
	}

	sort.Slice(runs, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "name":
			less = runs[i].Name < runs[j].Name
		case "status":
			less = string(runs[i].Status) < string(runs[j].Status)
		default: // started_at
			less = runs[i].StartedAt.Before(runs[j].StartedAt)
		}
		if orderDir == "desc" {
			return !less
		}
		return less
	})

	// Apply pagination
	if opts.Offset > 0 && opts.Offset < len(runs) {
		runs = runs[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(runs) {
		runs = runs[:opts.Limit]
	}

	return runs, nil
}

// GetRun returns a run by ID, searching all projects.
func (s *Storage) GetRun(ctx context.Context, runID string) (*domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	projects, err := os.ReadDir(s.logdir)
	if err != nil {
		return nil, fmt.Errorf("failed to read logdir: %w", err)
	}

	for _, project := range projects {
		if !project.IsDir() {
			continue
		}

		run, err := s.findRunByID(project.Name(), runID)
		if err == nil && run != nil {
			return run, nil
		}
	}

	return nil, fmt.Errorf("run not found: %s", runID)
}

// GetRunByProject returns a run by ID within a specific project.
func (s *Storage) GetRunByProject(ctx context.Context, project, runID string) (*domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.findRunByID(project, runID)
}

func (s *Storage) findRunByID(project, runID string) (*domain.Run, error) {
	projectDir := filepath.Join(s.logdir, project)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	var matches []*domain.Run
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		run, err := s.loadRunMeta(project, entry.Name())
		if err != nil {
			continue
		}

		idStr := run.ID.String()
		if idStr == runID {
			return run, nil
		}
		if strings.HasPrefix(idStr, runID) {
			matches = append(matches, run)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous run ID prefix '%s' matches %d runs, please provide more characters", runID, len(matches))
	}

	return nil, fmt.Errorf("run not found: %s", runID)
}

func (s *Storage) loadRunMeta(project, runDir string) (*domain.Run, error) {
	metaPath := filepath.Join(s.logdir, project, runDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta struct {
		ID             string            `json:"id"`
		Name           string            `json:"name"`
		Project        string            `json:"project"`
		Status         string            `json:"status"`
		Tags           []string          `json:"tags"`
		GitInfo        *domain.GitInfo   `json:"git_info"`
		SystemInfo     *domain.SystemInfo `json:"system_info"`
		StartedAt      string            `json:"started_at"`
		EndedAt        *string           `json:"ended_at"`
		ErrorMessage   *string           `json:"error_message"`
		DurationSeconds *float64         `json:"duration_seconds"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	// Parse UUID
	id, err := parseUUID(meta.ID)
	if err != nil {
		return nil, err
	}

	// Parse timestamps
	startedAt, _ := time.Parse(time.RFC3339, meta.StartedAt)
	var endedAt *time.Time
	if meta.EndedAt != nil {
		t, _ := time.Parse(time.RFC3339, *meta.EndedAt)
		endedAt = &t
	}

	// Load config
	var config map[string]any
	configPath := filepath.Join(s.logdir, project, runDir, "config.json")
	if configData, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(configData, &config)
	}

	run := &domain.Run{
		ID:              id,
		Name:            meta.Name,
		Status:          domain.RunStatus(meta.Status),
		Tags:            meta.Tags,
		GitInfo:         meta.GitInfo,
		SystemInfo:      meta.SystemInfo,
		Config:          config,
		ErrorMessage:    meta.ErrorMessage,
		StartedAt:       startedAt,
		EndedAt:         endedAt,
		DurationSeconds: meta.DurationSeconds,
		CreatedAt:       startedAt,
	}

	// Try to load latest metrics
	run.LatestMetrics, _ = s.GetLatestMetrics(context.Background(), meta.ID)

	// Try to load last metric time (used to detect inactive runs)
	if run.Status == domain.RunStatusRunning {
		if db, err := s.getRunDB(meta.ID); err == nil {
			var lastTime float64
			if err := db.QueryRowContext(context.Background(), "SELECT MAX(time) FROM metrics").Scan(&lastTime); err == nil && lastTime > 0 {
				t := time.Unix(int64(lastTime), 0)
				run.LastMetricTime = &t
			}
		}
	}

	return run, nil
}

// GetMetricNames returns all metric names for a run.
func (s *Storage) GetMetricNames(ctx context.Context, runID string) ([]string, error) {
	db, err := s.getRunDB(runID)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, "SELECT DISTINCT name FROM metrics ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	return names, rows.Err()
}

// GetMetricSeries returns metric data points for a specific metric.
func (s *Storage) GetMetricSeries(ctx context.Context, runID, metricName string, opts storage.MetricQueryOptions) ([]storage.MetricPoint, error) {
	db, err := s.getRunDB(runID)
	if err != nil {
		return nil, err
	}

	query := "SELECT time, step, value FROM metrics WHERE name = ?"
	args := []any{metricName}

	if opts.MinStep != nil {
		query += " AND step >= ?"
		args = append(args, *opts.MinStep)
	}
	if opts.MaxStep != nil {
		query += " AND step <= ?"
		args = append(args, *opts.MaxStep)
	}
	if opts.Since != nil {
		query += " AND time >= ?"
		args = append(args, opts.Since.Unix())
	}
	if opts.Until != nil {
		query += " AND time <= ?"
		args = append(args, opts.Until.Unix())
	}

	query += " ORDER BY step ASC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []storage.MetricPoint
	for rows.Next() {
		var ts float64
		var step int64
		var value float64
		if err := rows.Scan(&ts, &step, &value); err != nil {
			return nil, err
		}
		points = append(points, storage.MetricPoint{
			Time:  time.Unix(int64(ts), int64((ts-float64(int64(ts)))*1e9)),
			Step:  step,
			Value: value,
		})
	}

	// Apply downsampling if needed
	if opts.MaxPoints > 0 && len(points) > opts.MaxPoints {
		points = downsample(points, opts.MaxPoints)
	}

	return points, rows.Err()
}

// GetLatestMetrics returns the latest value for each metric.
func (s *Storage) GetLatestMetrics(ctx context.Context, runID string) (map[string]float64, error) {
	db, err := s.getRunDB(runID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT name, value
		FROM metrics m1
		WHERE step = (SELECT MAX(step) FROM metrics m2 WHERE m2.name = m1.name)
		GROUP BY name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		result[name] = value
	}

	return result, rows.Err()
}

// GetMetricsSummary returns summary statistics for all metrics in a run.
func (s *Storage) GetMetricsSummary(ctx context.Context, runID string) (*storage.MetricsSummary, error) {
	db, err := s.getRunDB(runID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			name,
			COUNT(*) as count,
			MIN(value) as min_value,
			MAX(value) as max_value,
			AVG(value) as avg_value,
			MIN(step) as first_step,
			MAX(step) as last_step,
			MIN(time) as first_time,
			MAX(time) as last_time
		FROM metrics
		GROUP BY name
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []storage.MetricSummary
	var totalPoints int64

	for rows.Next() {
		var m storage.MetricSummary
		var firstTime, lastTime float64
		if err := rows.Scan(&m.Name, &m.Count, &m.MinValue, &m.MaxValue, &m.AvgValue, &m.FirstStep, &m.LastStep, &firstTime, &lastTime); err != nil {
			return nil, err
		}
		m.FirstTime = time.Unix(int64(firstTime), 0)
		m.LastTime = time.Unix(int64(lastTime), 0)

		// Get first and last values
		var firstVal, lastVal float64
		db.QueryRowContext(ctx, "SELECT value FROM metrics WHERE name = ? ORDER BY step ASC LIMIT 1", m.Name).Scan(&firstVal)
		db.QueryRowContext(ctx, "SELECT value FROM metrics WHERE name = ? ORDER BY step DESC LIMIT 1", m.Name).Scan(&lastVal)
		m.FirstValue = firstVal
		m.LastValue = lastVal

		totalPoints += m.Count
		metrics = append(metrics, m)
	}

	return &storage.MetricsSummary{
		RunID:       runID,
		TotalPoints: totalPoints,
		MetricCount: len(metrics),
		Metrics:     metrics,
	}, rows.Err()
}

// Health checks if the storage is accessible.
func (s *Storage) Health(ctx context.Context) error {
	_, err := os.Stat(s.logdir)
	return err
}

// Close closes all open database connections.
func (s *Storage) Close() error {
	s.dbCacheMu.Lock()
	defer s.dbCacheMu.Unlock()

	for _, db := range s.dbCache {
		db.Close()
	}
	s.dbCache = make(map[string]*sql.DB)
	return nil
}

// getRunDB returns the SQLite database for a run, opening it if necessary.
func (s *Storage) getRunDB(runID string) (*sql.DB, error) {
	s.dbCacheMu.RLock()
	if db, ok := s.dbCache[runID]; ok {
		s.dbCacheMu.RUnlock()
		return db, nil
	}
	s.dbCacheMu.RUnlock()

	// Find the database file
	dbPath, err := s.findRunDB(runID)
	if err != nil {
		return nil, err
	}

	// Open the database
	s.dbCacheMu.Lock()
	defer s.dbCacheMu.Unlock()

	// Check again in case another goroutine opened it
	if db, ok := s.dbCache[runID]; ok {
		return db, nil
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}

	s.dbCache[runID] = db
	return db, nil
}

func (s *Storage) findRunDB(runID string) (string, error) {
	projects, err := os.ReadDir(s.logdir)
	if err != nil {
		return "", err
	}

	for _, project := range projects {
		if !project.IsDir() {
			continue
		}

		projectDir := filepath.Join(s.logdir, project.Name())
		runs, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, run := range runs {
			if !run.IsDir() {
				continue
			}

			metaPath := filepath.Join(projectDir, run.Name(), "meta.json")
			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}

			var meta struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(data, &meta); err != nil {
				continue
			}

			if meta.ID == runID {
				return filepath.Join(projectDir, run.Name(), "run.db"), nil
			}
		}
	}

	return "", fmt.Errorf("run database not found: %s", runID)
}

// downsample reduces the number of points using LTTB algorithm.
func downsample(points []storage.MetricPoint, targetCount int) []storage.MetricPoint {
	if len(points) <= targetCount {
		return points
	}

	// Simple downsampling: keep every nth point
	// For better quality, implement LTTB (Largest Triangle Three Buckets)
	step := float64(len(points)) / float64(targetCount)
	result := make([]storage.MetricPoint, 0, targetCount)

	for i := 0; i < targetCount; i++ {
		idx := int(float64(i) * step)
		if idx >= len(points) {
			idx = len(points) - 1
		}
		result = append(result, points[idx])
	}

	return result
}

// parseUUID parses a UUID string using google/uuid.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// GetContinuations returns all continuations for a run.
func (s *Storage) GetContinuations(ctx context.Context, runID string) ([]*domain.Continuation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the run directory
	runDir, err := s.findRunDir(runID)
	if err != nil {
		return nil, err
	}

	// Read continuations.json
	continuationsPath := filepath.Join(runDir, "continuations.json")
	data, err := os.ReadFile(continuationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Continuation{}, nil
		}
		return nil, fmt.Errorf("failed to read continuations: %w", err)
	}

	var rawContinuations []struct {
		ID           string             `json:"id"`
		Step         int64              `json:"step"`
		Timestamp    string             `json:"timestamp"`
		ConfigBefore map[string]any     `json:"config_before"`
		ConfigAfter  map[string]any     `json:"config_after"`
		Note         *string            `json:"note"`
		GitInfo      *domain.GitInfo    `json:"git_info"`
		SystemInfo   *domain.SystemInfo `json:"system_info"`
	}

	if err := json.Unmarshal(data, &rawContinuations); err != nil {
		return nil, fmt.Errorf("failed to parse continuations: %w", err)
	}

	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return nil, fmt.Errorf("invalid run ID: %w", err)
	}

	continuations := make([]*domain.Continuation, len(rawContinuations))
	for i, raw := range rawContinuations {
		id, _ := uuid.Parse(raw.ID)
		timestamp, _ := time.Parse(time.RFC3339, raw.Timestamp)

		continuations[i] = &domain.Continuation{
			ID:           id,
			RunID:        runUUID,
			Step:         raw.Step,
			Timestamp:    timestamp,
			ConfigBefore: raw.ConfigBefore,
			ConfigAfter:  raw.ConfigAfter,
			Note:         raw.Note,
			GitInfo:      raw.GitInfo,
			SystemInfo:   raw.SystemInfo,
			CreatedAt:    timestamp,
		}
	}

	return continuations, nil
}

// ListSweeps returns all sweeps in a project.
func (s *Storage) ListSweeps(ctx context.Context, project string) ([]domain.Sweep, error) {
	sweepsDir := filepath.Join(s.logdir, project, ".sweeps")
	entries, err := os.ReadDir(sweepsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.Sweep{}, nil
		}
		return nil, fmt.Errorf("failed to read sweeps directory: %w", err)
	}

	var sweeps []domain.Sweep
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sweep, err := s.loadSweep(project, entry.Name())
		if err != nil {
			continue
		}
		sweeps = append(sweeps, *sweep)
	}

	sort.Slice(sweeps, func(i, j int) bool {
		return sweeps[i].CreatedAt.After(sweeps[j].CreatedAt)
	})

	return sweeps, nil
}

// GetSweep returns a sweep by ID within a project.
func (s *Storage) GetSweep(ctx context.Context, project, sweepID string) (*domain.Sweep, error) {
	return s.loadSweep(project, sweepID)
}

// GetSweepByID searches all projects for a sweep by ID.
func (s *Storage) GetSweepByID(ctx context.Context, sweepID string) (*domain.Sweep, string, error) {
	projects, err := os.ReadDir(s.logdir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read logdir: %w", err)
	}

	for _, project := range projects {
		if !project.IsDir() {
			continue
		}
		sweep, err := s.loadSweep(project.Name(), sweepID)
		if err == nil {
			return sweep, project.Name(), nil
		}
	}

	return nil, "", fmt.Errorf("sweep not found: %s", sweepID)
}

// GetSweepRuns returns runs belonging to a sweep.
func (s *Storage) GetSweepRuns(ctx context.Context, project, sweepID string) ([]*domain.Run, error) {
	sweepFile := filepath.Join(s.logdir, project, ".sweeps", sweepID, "sweep.json")
	data, err := os.ReadFile(sweepFile)
	if err != nil {
		return nil, fmt.Errorf("sweep not found: %w", err)
	}

	var raw struct {
		Runs []struct {
			RunID string `json:"run_id"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse sweep: %w", err)
	}

	var runs []*domain.Run
	for _, r := range raw.Runs {
		if r.RunID == "" {
			continue
		}
		run, err := s.GetRun(ctx, r.RunID)
		if err != nil {
			continue
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// StopSweep updates a sweep's status to stopped.
func (s *Storage) StopSweep(ctx context.Context, project, sweepID string) error {
	sweepFile := filepath.Join(s.logdir, project, ".sweeps", sweepID, "sweep.json")
	data, err := os.ReadFile(sweepFile)
	if err != nil {
		return fmt.Errorf("sweep not found: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse sweep: %w", err)
	}

	raw["status"] = "stopped"

	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sweepFile, updated, 0644)
}

func (s *Storage) loadSweep(project, sweepID string) (*domain.Sweep, error) {
	sweepFile := filepath.Join(s.logdir, project, ".sweeps", sweepID, "sweep.json")
	data, err := os.ReadFile(sweepFile)
	if err != nil {
		return nil, err
	}

	var raw struct {
		ID            string                      `json:"id"`
		Name          string                      `json:"name"`
		Status        string                      `json:"status"`
		Method        string                      `json:"method"`
		MetricName    string                      `json:"metric_name"`
		MetricGoal    string                      `json:"metric_goal"`
		SearchSpace   domain.SearchSpace          `json:"search_space"`
		Config        map[string]any              `json:"config"`
		MaxRuns       *int                        `json:"max_runs"`
		EarlyStopping *domain.EarlyStoppingConfig `json:"early_stopping"`
		BestRunID     *string                     `json:"best_run_id"`
		BestValue     *float64                    `json:"best_value"`
		RunCount      int                         `json:"run_count"`
		GridIndex     int                         `json:"grid_index"`
		StartedAt     string                      `json:"started_at"`
		CreatedAt     string                      `json:"created_at"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse sweep.json: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, raw.CreatedAt)
	startedAt := createdAt
	if raw.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.StartedAt); err == nil {
			startedAt = t
		}
	}

	// Use directory name as fallback ID
	id := raw.ID
	if id == "" {
		id = sweepID
	}

	return &domain.Sweep{
		ID:            id,
		Name:          raw.Name,
		Status:        domain.SweepStatus(raw.Status),
		Method:        raw.Method,
		MetricName:    raw.MetricName,
		MetricGoal:    raw.MetricGoal,
		SearchSpace:   raw.SearchSpace,
		Config:        raw.Config,
		MaxRuns:       raw.MaxRuns,
		EarlyStopping: raw.EarlyStopping,
		BestRunID:     raw.BestRunID,
		BestValue:     raw.BestValue,
		RunCount:      raw.RunCount,
		GridIndex:     raw.GridIndex,
		StartedAt:     startedAt,
		CreatedAt:     createdAt,
	}, nil
}

// findRunDir finds the directory for a run by ID.
func (s *Storage) findRunDir(runID string) (string, error) {
	projects, err := os.ReadDir(s.logdir)
	if err != nil {
		return "", err
	}

	for _, project := range projects {
		if !project.IsDir() {
			continue
		}

		projectDir := filepath.Join(s.logdir, project.Name())
		runs, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, run := range runs {
			if !run.IsDir() {
				continue
			}

			metaPath := filepath.Join(projectDir, run.Name(), "meta.json")
			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}

			var meta struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(data, &meta); err != nil {
				continue
			}

			if meta.ID == runID {
				return filepath.Join(projectDir, run.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("run not found: %s", runID)
}
