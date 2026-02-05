package middleware

import (
	"context"
	"net/http"
	"strings"

	"sixtyseven/internal/domain"
	"sixtyseven/internal/service"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// UserContextKey is the context key for the authenticated user
	UserContextKey ContextKey = "user"
	// APIKeyContextKey is the context key for the API key info
	APIKeyContextKey ContextKey = "apikey"
)

// AuthProvider interface for authentication
type AuthProvider interface {
	ValidateAccessToken(ctx context.Context, token string) (*domain.User, error)
	ValidateAPIKey(ctx context.Context, rawKey string) (*domain.APIKeyInfo, error)
}

// Auth middleware authenticates requests using JWT or API key
func Auth(authService AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Extract token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				// No "Bearer " prefix, check if it's just the token
				token = authHeader
			}

			var user *domain.User
			var apiKeyInfo *domain.APIKeyInfo

			// Check if it's an API key (starts with ss67_)
			if strings.HasPrefix(token, domain.APIKeyPrefix) {
				info, err := authService.ValidateAPIKey(r.Context(), token)
				if err != nil {
					http.Error(w, "invalid API key", http.StatusUnauthorized)
					return
				}
				user = info.User
				apiKeyInfo = info
			} else {
				// Assume it's a JWT
				var err error
				user, err = authService.ValidateAccessToken(r.Context(), token)
				if err != nil {
					if err == service.ErrTokenExpired {
						http.Error(w, "token expired", http.StatusUnauthorized)
						return
					}
					http.Error(w, "invalid token", http.StatusUnauthorized)
					return
				}
			}

			// Add user to context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			if apiKeyInfo != nil {
				ctx = context.WithValue(ctx, APIKeyContextKey, apiKeyInfo)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUser retrieves the user from the request context
func GetUser(ctx context.Context) *domain.User {
	user, ok := ctx.Value(UserContextKey).(*domain.User)
	if !ok {
		return nil
	}
	return user
}

// GetAPIKeyInfo retrieves the API key info from the request context
func GetAPIKeyInfo(ctx context.Context) *domain.APIKeyInfo {
	info, ok := ctx.Value(APIKeyContextKey).(*domain.APIKeyInfo)
	if !ok {
		return nil
	}
	return info
}

// RequireScope middleware checks if the API key has the required scope
func RequireScope(scope domain.APIKeyScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := GetAPIKeyInfo(r.Context())

			// If no API key info, assume JWT auth with full access
			if info == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check scope
			if !info.Key.HasScope(scope) {
				http.Error(w, "insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
