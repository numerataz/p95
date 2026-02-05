package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sixtyseven/internal/api/middleware"
	"sixtyseven/internal/domain"
	"sixtyseven/internal/service"
)

// RunHandler handles run endpoints
type RunHandler struct {
	runService  *service.RunService
	appService  *service.AppService
	teamService *service.TeamService
}

// NewRunHandler creates a new run handler
func NewRunHandler(runService *service.RunService, appService *service.AppService, teamService *service.TeamService) *RunHandler {
	return &RunHandler{
		runService:  runService,
		appService:  appService,
		teamService: teamService,
	}
}

// getAppAndCheckPermission is a helper to get app and check user permission
func (h *RunHandler) getAppAndCheckPermission(w http.ResponseWriter, r *http.Request, requiredRole domain.TeamRole) (*domain.App, bool) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return nil, false
	}

	hasPermission, err := h.teamService.HasPermission(r.Context(), team.ID, user.ID, requiredRole)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if !hasPermission {
		respondError(w, http.StatusForbidden, "insufficient permissions")
		return nil, false
	}

	appSlug := chi.URLParam(r, "appSlug")
	app, err := h.appService.GetBySlug(r.Context(), team.ID, appSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if app == nil {
		respondError(w, http.StatusNotFound, "app not found")
		return nil, false
	}

	return app, true
}

// List returns all runs in an app
func (h *RunHandler) List(w http.ResponseWriter, r *http.Request) {
	app, ok := h.getAppAndCheckPermission(w, r, domain.TeamRoleViewer)
	if !ok {
		return
	}

	// Parse query params
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	opts := domain.RunListOptions{
		AppID:    app.ID,
		Limit:    limit,
		Offset:   offset,
		OrderBy:  r.URL.Query().Get("order_by"),
		OrderDir: r.URL.Query().Get("order_dir"),
	}

	// Parse status filter
	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.RunStatus(status)
		opts.Status = &s
	}

	runs, err := h.runService.List(r.Context(), opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, runs)
}

// Create creates a new run
func (h *RunHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	app, ok := h.getAppAndCheckPermission(w, r, domain.TeamRoleMember)
	if !ok {
		return
	}

	var req domain.RunCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.runService.Create(r.Context(), app.ID, user.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, run)
}

// Get returns a run by ID
func (h *RunHandler) Get(w http.ResponseWriter, r *http.Request) {
	_, ok := h.getAppAndCheckPermission(w, r, domain.TeamRoleViewer)
	if !ok {
		return
	}

	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	// Include metrics if requested
	var run *domain.Run
	if r.URL.Query().Get("include_metrics") == "true" {
		run, err = h.runService.GetWithMetrics(r.Context(), runID)
	} else {
		run, err = h.runService.GetByID(r.Context(), runID)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	respondJSON(w, http.StatusOK, run)
}

// GetByID returns a run by ID directly (for TUI/SDK convenience)
func (h *RunHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	// Include metrics if requested
	var run *domain.Run
	if r.URL.Query().Get("include_metrics") == "true" {
		run, err = h.runService.GetWithMetrics(r.Context(), runID)
	} else {
		run, err = h.runService.GetByID(r.Context(), runID)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	// Verify user has access to this run's app/team
	app, err := h.appService.GetByID(r.Context(), run.AppID)
	if err != nil || app == nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	hasPermission, err := h.teamService.HasPermission(r.Context(), app.TeamID, user.ID, domain.TeamRoleViewer)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasPermission {
		respondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	respondJSON(w, http.StatusOK, run)
}

// Update updates a run
func (h *RunHandler) Update(w http.ResponseWriter, r *http.Request) {
	_, ok := h.getAppAndCheckPermission(w, r, domain.TeamRoleMember)
	if !ok {
		return
	}

	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	var req domain.RunUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updatedRun, err := h.runService.Update(r.Context(), runID, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, updatedRun)
}

// Delete deletes a run
func (h *RunHandler) Delete(w http.ResponseWriter, r *http.Request) {
	_, ok := h.getAppAndCheckPermission(w, r, domain.TeamRoleAdmin)
	if !ok {
		return
	}

	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := h.runService.Delete(r.Context(), runID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "run deleted"})
}

// UpdateStatus updates a run's status (by run ID for SDK convenience)
func (h *RunHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	var req domain.RunStatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.runService.UpdateStatus(r.Context(), runID, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, run)
}

// UpdateConfig merges new config with existing config
func (h *RunHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	var config map[string]any
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.runService.UpdateConfig(r.Context(), runID, config); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "config updated"})
}

// ResumeRun resumes a completed/failed run
func (h *RunHandler) ResumeRun(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	var req domain.ResumeRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.runService.ResumeRun(r.Context(), runID, req)
	if err != nil {
		// Check for specific error types
		if err.Error() == "run not found" {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "run is already running" {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, response)
}

// GetContinuations returns all continuations for a run
func (h *RunHandler) GetContinuations(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	continuations, err := h.runService.GetContinuations(r.Context(), runID)
	if err != nil {
		if err.Error() == "run not found" {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return empty array instead of null if no continuations
	if continuations == nil {
		continuations = []*domain.Continuation{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"continuations": continuations})
}
