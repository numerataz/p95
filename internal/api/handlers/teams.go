package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sixtyseven/internal/api/middleware"
	"sixtyseven/internal/domain"
	"sixtyseven/internal/service"
)

// TeamHandler handles team endpoints
type TeamHandler struct {
	teamService *service.TeamService
}

// NewTeamHandler creates a new team handler
func NewTeamHandler(teamService *service.TeamService) *TeamHandler {
	return &TeamHandler{teamService: teamService}
}

// List returns all teams for the authenticated user
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teams, err := h.teamService.ListByUser(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, teams)
}

// Create creates a new team
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req domain.TeamCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	team, err := h.teamService.Create(r.Context(), user.ID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, team)
}

// Get returns a team by slug
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return
	}

	// Check membership
	member, err := h.teamService.GetMember(r.Context(), team.ID, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if member == nil {
		respondError(w, http.StatusForbidden, "not a member of this team")
		return
	}

	respondJSON(w, http.StatusOK, team)
}

// Update updates a team
func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return
	}

	// Check admin permission
	hasPermission, err := h.teamService.HasPermission(r.Context(), team.ID, user.ID, domain.TeamRoleAdmin)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasPermission {
		respondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var req domain.TeamUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updatedTeam, err := h.teamService.Update(r.Context(), team.ID, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, updatedTeam)
}

// Delete deletes a team
func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return
	}

	// Check owner permission
	hasPermission, err := h.teamService.HasPermission(r.Context(), team.ID, user.ID, domain.TeamRoleOwner)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasPermission {
		respondError(w, http.StatusForbidden, "only owners can delete teams")
		return
	}

	if err := h.teamService.Delete(r.Context(), team.ID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "team deleted"})
}

// ListMembers returns all members of a team
func (h *TeamHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return
	}

	// Check membership
	member, err := h.teamService.GetMember(r.Context(), team.ID, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if member == nil {
		respondError(w, http.StatusForbidden, "not a member of this team")
		return
	}

	members, err := h.teamService.ListMembers(r.Context(), team.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, members)
}

// AddMember adds a member to a team
func (h *TeamHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return
	}

	// Check admin permission
	hasPermission, err := h.teamService.HasPermission(r.Context(), team.ID, user.ID, domain.TeamRoleAdmin)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasPermission {
		respondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var req domain.TeamInvite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	member, err := h.teamService.AddMember(r.Context(), team.ID, req.Email, req.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, member)
}

// UpdateMember updates a team member's role
func (h *TeamHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return
	}

	// Check admin permission
	hasPermission, err := h.teamService.HasPermission(r.Context(), team.ID, user.ID, domain.TeamRoleAdmin)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasPermission {
		respondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	userIDStr := chi.URLParam(r, "userID")
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		Role domain.TeamRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.teamService.UpdateMemberRole(r.Context(), team.ID, targetUserID, req.Role); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "member updated"})
}

// RemoveMember removes a member from a team
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamSlug := chi.URLParam(r, "teamSlug")
	team, err := h.teamService.GetBySlug(r.Context(), teamSlug)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		respondError(w, http.StatusNotFound, "team not found")
		return
	}

	// Check admin permission
	hasPermission, err := h.teamService.HasPermission(r.Context(), team.ID, user.ID, domain.TeamRoleAdmin)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasPermission {
		respondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	userIDStr := chi.URLParam(r, "userID")
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := h.teamService.RemoveMember(r.Context(), team.ID, targetUserID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "member removed"})
}
