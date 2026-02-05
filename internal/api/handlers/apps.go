package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sixtyseven/internal/api/middleware"
	"sixtyseven/internal/domain"
	"sixtyseven/internal/service"
)

// AppHandler handles app endpoints
type AppHandler struct {
	appService  *service.AppService
	teamService *service.TeamService
}

// NewAppHandler creates a new app handler
func NewAppHandler(appService *service.AppService, teamService *service.TeamService) *AppHandler {
	return &AppHandler{
		appService:  appService,
		teamService: teamService,
	}
}

// getTeamAndCheckPermission is a helper to get team and check user permission
func (h *AppHandler) getTeamAndCheckPermission(w http.ResponseWriter, r *http.Request, requiredRole domain.TeamRole) (*domain.Team, bool) {
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

	return team, true
}

// List returns all apps in a team
func (h *AppHandler) List(w http.ResponseWriter, r *http.Request) {
	team, ok := h.getTeamAndCheckPermission(w, r, domain.TeamRoleViewer)
	if !ok {
		return
	}

	opts := domain.AppListOptions{
		TeamID:          team.ID,
		IncludeArchived: r.URL.Query().Get("include_archived") == "true",
	}

	apps, err := h.appService.List(r.Context(), opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, apps)
}

// Create creates a new app
func (h *AppHandler) Create(w http.ResponseWriter, r *http.Request) {
	team, ok := h.getTeamAndCheckPermission(w, r, domain.TeamRoleMember)
	if !ok {
		return
	}

	var req domain.AppCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	app, err := h.appService.Create(r.Context(), team.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, app)
}

// Get returns an app by slug
func (h *AppHandler) Get(w http.ResponseWriter, r *http.Request) {
	team, ok := h.getTeamAndCheckPermission(w, r, domain.TeamRoleViewer)
	if !ok {
		return
	}

	appSlug := chi.URLParam(r, "appSlug")
	app, err := h.appService.GetBySlug(r.Context(), team.ID, appSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if app == nil {
		respondError(w, http.StatusNotFound, "app not found")
		return
	}

	respondJSON(w, http.StatusOK, app)
}

// Update updates an app
func (h *AppHandler) Update(w http.ResponseWriter, r *http.Request) {
	team, ok := h.getTeamAndCheckPermission(w, r, domain.TeamRoleMember)
	if !ok {
		return
	}

	appSlug := chi.URLParam(r, "appSlug")
	app, err := h.appService.GetBySlug(r.Context(), team.ID, appSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if app == nil {
		respondError(w, http.StatusNotFound, "app not found")
		return
	}

	var req domain.AppUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updatedApp, err := h.appService.Update(r.Context(), app.ID, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, updatedApp)
}

// Delete archives or deletes an app
func (h *AppHandler) Delete(w http.ResponseWriter, r *http.Request) {
	team, ok := h.getTeamAndCheckPermission(w, r, domain.TeamRoleAdmin)
	if !ok {
		return
	}

	appSlug := chi.URLParam(r, "appSlug")
	app, err := h.appService.GetBySlug(r.Context(), team.ID, appSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if app == nil {
		respondError(w, http.StatusNotFound, "app not found")
		return
	}

	// Archive by default, delete if ?permanent=true
	if r.URL.Query().Get("permanent") == "true" {
		if err := h.appService.Delete(r.Context(), app.ID); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "app deleted"})
	} else {
		if err := h.appService.Archive(r.Context(), app.ID); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "app archived"})
	}
}
