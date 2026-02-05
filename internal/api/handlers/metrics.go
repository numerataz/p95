package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sixtyseven/internal/domain"
	"sixtyseven/internal/service"
)

// MetricsHandler handles metrics endpoints
type MetricsHandler struct {
	metricsService *service.MetricsService
	runService     *service.RunService
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(metricsService *service.MetricsService, runService *service.RunService) *MetricsHandler {
	return &MetricsHandler{
		metricsService: metricsService,
		runService:     runService,
	}
}

// BatchLog handles batch metric logging from SDK
func (h *MetricsHandler) BatchLog(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	// Verify run exists
	run, err := h.runService.GetByID(r.Context(), runID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	// Parse metrics
	var req domain.BatchMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate batch size
	if len(req.Metrics) > 1000 {
		respondError(w, http.StatusBadRequest, "batch size exceeds limit (1000)")
		return
	}

	// Log metrics
	if err := h.metricsService.LogMetrics(r.Context(), runID, req.Metrics); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]int{"accepted": len(req.Metrics)})
}

// List returns all metric names for a run
func (h *MetricsHandler) List(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	names, err := h.metricsService.GetMetricNames(r.Context(), runID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string][]string{"metrics": names})
}

// GetLatest returns the latest value for each metric
func (h *MetricsHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	latest, err := h.metricsService.GetLatest(r.Context(), runID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, latest)
}

// GetSummary returns summary statistics for all metrics
func (h *MetricsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	summary, err := h.metricsService.GetSummary(r.Context(), runID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// GetSeries returns time series data for a specific metric
func (h *MetricsHandler) GetSeries(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	metricNameRaw := chi.URLParam(r, "metricName")
	if metricNameRaw == "" {
		respondError(w, http.StatusBadRequest, "metric name required")
		return
	}
	metricName, err := url.PathUnescape(metricNameRaw)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid metric name encoding")
		return
	}

	// Parse query options
	opts := domain.MetricQueryOptions{
		RunID:      runID,
		MetricName: metricName,
	}

	// Parse time range
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = &t
		}
	}
	if until := r.URL.Query().Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			opts.Until = &t
		}
	}

	// Parse step range
	if minStep := r.URL.Query().Get("min_step"); minStep != "" {
		if s, err := strconv.ParseInt(minStep, 10, 64); err == nil {
			opts.MinStep = &s
		}
	}
	if maxStep := r.URL.Query().Get("max_step"); maxStep != "" {
		if s, err := strconv.ParseInt(maxStep, 10, 64); err == nil {
			opts.MaxStep = &s
		}
	}

	// Parse downsampling
	if maxPoints := r.URL.Query().Get("max_points"); maxPoints != "" {
		if m, err := strconv.Atoi(maxPoints); err == nil {
			opts.MaxPoints = m
		}
	}

	// Parse pagination
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			opts.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			opts.Offset = o
		}
	}

	series, err := h.metricsService.GetSeries(r.Context(), opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"metric": metricName,
		"run_id": runID,
		"points": series,
	})
}
