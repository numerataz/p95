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

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.UserCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.authService.Register(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, user.ToResponse())
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req service.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.Login(r.Context(), req)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if err == service.ErrUserInactive {
			respondError(w, http.StatusForbidden, "account is inactive")
			return
		}
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// Me returns the current authenticated user
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	respondJSON(w, http.StatusOK, user.ToResponse())
}

// Logout handles user logout (invalidates session if using sessions)
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// For JWT-based auth, logout is typically client-side
	// For session-based auth, we would invalidate the session here
	respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// CreateAPIKey creates a new API key
func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req domain.APIKeyCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authService.CreateAPIKey(r.Context(), user.ID, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the raw key - this is the only time it will be visible
	respondJSON(w, http.StatusCreated, resp)
}

// ListAPIKeys lists all API keys for the authenticated user
func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keys, err := h.authService.ListAPIKeysByUser(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format (without key hash)
	response := make([]map[string]any, len(keys))
	for i, key := range keys {
		response[i] = map[string]any{
			"id":           key.ID,
			"user_id":      key.UserID,
			"team_id":      key.TeamID,
			"key_prefix":   key.KeyPrefix,
			"name":         key.Name,
			"scopes":       key.Scopes,
			"last_used_at": key.LastUsedAt,
			"expires_at":   key.ExpiresAt,
			"created_at":   key.CreatedAt,
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// DeleteAPIKey deletes an API key
func (h *AuthHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keyIDStr := chi.URLParam(r, "keyID")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	if err := h.authService.DeleteAPIKey(r.Context(), user.ID, keyID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "API key deleted"})
}
