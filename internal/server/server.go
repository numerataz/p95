// Package server provides a lightweight HTTP server for local mode.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/ninetyfive/p95/internal/domain"
	"github.com/ninetyfive/p95/internal/storage"
	"github.com/ninetyfive/p95/internal/storage/file"
)

// Server represents the local HTTP server.
type Server struct {
	storage   *file.Storage
	router    *chi.Mux
	server    *http.Server
	webFS     fs.FS
	activeRun string // Currently active run ID for UI navigation
}

// New creates a new local server instance.
func New(logdir string, webFS fs.FS) (*Server, error) {
	store, err := file.New(logdir)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	s := &Server{
		storage: store,
		webFS:   webFS,
	}

	s.setupRouter()
	return s, nil
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	// Note: Logger middleware removed - it breaks TUI mode
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// CORS - allow all origins for local development
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", s.healthCheck)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Config endpoint - tells frontend we're in local mode
		r.Get("/config", s.getConfig)

		// Projects (local mode concept)
		r.Get("/projects", s.listProjects)
		r.Get("/projects/{projectSlug}/runs", s.listRuns)

		// TUI compatibility: fake teams/apps endpoints that map to projects
		r.Get("/teams", s.listTeams)
		r.Get("/teams/{teamSlug}/apps", s.listApps)
		r.Get("/teams/{teamSlug}/apps/{appSlug}/runs", s.listAppRuns)

		// Active run (for UI auto-navigation)
		r.Get("/active-run", s.getActiveRun)
		r.Post("/active-run", s.setActiveRun)

		// Runs (compatible with hosted mode API)
		r.Get("/runs/{runID}", s.getRun)

		// Metrics
		r.Get("/runs/{runID}/metrics", s.getMetricNames)
		r.Get("/runs/{runID}/metrics/latest", s.getLatestMetrics)
		r.Get("/runs/{runID}/metrics/summary", s.getMetricsSummary)
		r.Get("/runs/{runID}/metrics/{metricName}", s.getMetricSeries)

		// Continuations
		r.Get("/runs/{runID}/continuations", s.getContinuations)
	})

	// Serve embedded web UI if available
	if s.webFS != nil {
		r.Get("/*", s.serveWebUI)
	} else {
		// Serve a simple status page
		r.Get("/", s.statusPage)
	}

	s.router = r
}

// Start starts the HTTP server.
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.storage != nil {
		s.storage.Close()
	}

	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Handler functions

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.Health(r.Context()); err != nil {
		http.Error(w, "storage unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]any{
		"mode":    "local",
		"version": "0.1.0",
		"logdir":  s.storage.LogDir(),
		"features": map[string]bool{
			"auth":     false,
			"teams":    false,
			"apiKeys":  false,
			"realtime": false, // Polling only for now
		},
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) getActiveRun(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"run_id": s.activeRun,
	})
}

func (s *Server) setActiveRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	s.activeRun = req.RunID
	writeJSON(w, http.StatusOK, map[string]any{
		"run_id": s.activeRun,
	})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.storage.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"projects": projects,
	})
}

// TUI compatibility: fake teams endpoint returns a single "local" team
func (s *Server) listTeams(w http.ResponseWriter, r *http.Request) {
	teams := []map[string]any{
		{
			"id":          "00000000-0000-0000-0000-000000000000",
			"name":        "Local",
			"slug":        "local",
			"description": "Local experiments",
			"role":        "owner",
		},
	}
	writeJSON(w, http.StatusOK, teams)
}

// TUI compatibility: apps endpoint returns projects as apps
func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	projects, err := s.storage.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list projects")
		return
	}

	// Convert projects to apps format
	apps := make([]map[string]any, len(projects))
	for i, p := range projects {
		apps[i] = map[string]any{
			"id":          fmt.Sprintf("00000000-0000-0000-0000-%012d", i),
			"name":        p.Name,
			"slug":        p.Slug,
			"description": "",
			"run_count":   p.RunCount,
		}
	}
	writeJSON(w, http.StatusOK, apps)
}

// TUI compatibility: app runs endpoint
func (s *Server) listAppRuns(w http.ResponseWriter, r *http.Request) {
	appSlug := chi.URLParam(r, "appSlug")

	opts := domain.RunListOptions{
		Limit:    100,
		OrderBy:  r.URL.Query().Get("order_by"),
		OrderDir: r.URL.Query().Get("order_dir"),
	}

	runs, err := s.storage.ListRuns(r.Context(), appSlug, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list runs")
		return
	}

	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "projectSlug")

	opts := domain.RunListOptions{
		Limit:    100,
		OrderBy:  r.URL.Query().Get("order_by"),
		OrderDir: r.URL.Query().Get("order_dir"),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.RunStatus(status)
		opts.Status = &s
	}

	runs, err := s.storage.ListRuns(r.Context(), projectSlug, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list runs")
		return
	}

	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	run, err := s.storage.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Run not found")
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func (s *Server) getMetricNames(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	names, err := s.storage.GetMetricNames(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get metric names")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metrics": names,
	})
}

func (s *Server) getLatestMetrics(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	latest, err := s.storage.GetLatestMetrics(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get latest metrics")
		return
	}

	writeJSON(w, http.StatusOK, latest)
}

func (s *Server) getMetricsSummary(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	summary, err := s.storage.GetMetricsSummary(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get metrics summary")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) getMetricSeries(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")
	metricNameRaw := chi.URLParam(r, "metricName")

	// URL-decode the metric name since chi doesn't do this automatically
	// This allows metric names like "train/loss" to be passed as "train%2Floss"
	metricName, _ := url.PathUnescape(metricNameRaw)

	opts := storage.MetricQueryOptions{
		MaxPoints: 1000, // Default max points
		Limit:     10000,
	}

	if v := r.URL.Query().Get("max_points"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.MaxPoints = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.Limit = n
		}
	}
	if v := r.URL.Query().Get("min_step"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.MinStep = &n
		}
	}
	if v := r.URL.Query().Get("max_step"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			opts.MaxStep = &n
		}
	}

	points, err := s.storage.GetMetricSeries(r.Context(), runID, metricName, opts)
	if err != nil {
		log.Printf("Error getting metric series: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get metric series")
		return
	}

	// Convert to API format
	type apiPoint struct {
		Step  int64   `json:"step"`
		Value float64 `json:"value"`
		Time  string  `json:"time"`
	}

	apiPoints := make([]apiPoint, len(points))
	for i, p := range points {
		apiPoints[i] = apiPoint{
			Step:  p.Step,
			Value: p.Value,
			Time:  p.Time.Format(time.RFC3339),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metric": metricName,
		"name":   metricName,
		"points": apiPoints,
	})
}

func (s *Server) getContinuations(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	continuations, err := s.storage.GetContinuations(r.Context(), runID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "Run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get continuations")
		return
	}

	// Return empty array instead of null
	if continuations == nil {
		continuations = []*domain.Continuation{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"continuations": continuations})
}

func (s *Server) serveWebUI(w http.ResponseWriter, r *http.Request) {
	// SPA serving: try the requested file, fall back to index.html for client-side routing
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else {
		path = path[1:] // Remove leading slash
	}

	// Try to open the requested file
	f, err := s.webFS.Open(path)
	if err != nil {
		// File not found - serve index.html for SPA routing
		path = "index.html"
	} else {
		// Check if it's a directory
		stat, _ := f.Stat()
		f.Close()
		if stat != nil && stat.IsDir() {
			path = "index.html"
		}
	}

	// Read and serve the file
	content, err := fs.ReadFile(s.webFS, path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type
	contentType := "application/octet-stream"
	if strings.HasSuffix(path, ".html") {
		contentType = "text/html; charset=utf-8"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".svg") {
		contentType = "image/svg+xml"
	} else if strings.HasSuffix(path, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(path, ".json") {
		contentType = "application/json"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

func (s *Server) statusPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	html := `<!DOCTYPE html>
<html>
<head>
    <title>p95 Local Viewer</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
        pre { background: #f4f4f4; padding: 15px; border-radius: 5px; overflow-x: auto; }
        .status { color: #28a745; }
    </style>
</head>
<body>
    <h1>p95 Local Viewer</h1>
    <p class="status">Server is running.</p>
    <p>The web UI is not embedded in this build. You can:</p>
    <ol>
        <li>Run the web UI separately with <code>cd web && npm run dev</code></li>
        <li>Or use the API directly at <code>/api/v1</code></li>
    </ol>
    <h2>API Endpoints</h2>
    <pre>
GET /api/v1/config         - Server configuration
GET /api/v1/projects       - List all projects
GET /api/v1/projects/:slug/runs - List runs in a project
GET /api/v1/runs/:id       - Get run details
GET /api/v1/runs/:id/metrics - List metric names
GET /api/v1/runs/:id/metrics/latest - Get latest values
GET /api/v1/runs/:id/metrics/:name - Get metric series
    </pre>
    <h2>Log Directory</h2>
    <p><code>` + s.storage.LogDir() + `</code></p>
</body>
</html>`

	w.Write([]byte(html))
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
