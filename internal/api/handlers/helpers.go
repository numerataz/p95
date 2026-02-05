package handlers

import (
	"encoding/json"
	"net/http"
)

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       any `json:"data"`
	Total      int `json:"total"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
	HasMore    bool `json:"has_more"`
}
