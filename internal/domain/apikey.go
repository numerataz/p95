package domain

import (
	"time"

	"github.com/google/uuid"
)

// APIKeyPrefix is the prefix for all API keys
const APIKeyPrefix = "ss67_"

// APIKeyScope represents a permission scope for an API key
type APIKeyScope string

const (
	APIKeyScopeRead  APIKeyScope = "read"
	APIKeyScopeWrite APIKeyScope = "write"
	APIKeyScopeAdmin APIKeyScope = "admin"
)

// APIKey represents an API key for SDK authentication
type APIKey struct {
	ID         uuid.UUID     `json:"id"`
	UserID     uuid.UUID     `json:"user_id"`
	TeamID     *uuid.UUID    `json:"team_id,omitempty"` // Optional team scope
	KeyHash    string        `json:"-"`                 // Never expose
	KeyPrefix  string        `json:"key_prefix"`        // First 12 chars for identification
	Name       string        `json:"name"`
	Scopes     []APIKeyScope `json:"scopes"`
	LastUsedAt *time.Time    `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time    `json:"expires_at,omitempty"` // NULL = never expires
	CreatedAt  time.Time     `json:"created_at"`

	// Populated by joins
	User *User `json:"user,omitempty"`
	Team *Team `json:"team,omitempty"`
}

// APIKeyCreate contains fields for creating a new API key
type APIKeyCreate struct {
	Name      string        `json:"name" validate:"required,min=1,max=255"`
	TeamID    *uuid.UUID    `json:"team_id,omitempty"`
	Scopes    []APIKeyScope `json:"scopes,omitempty" validate:"omitempty,dive,oneof=read write admin"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
}

// APIKeyResponse is returned when creating a new API key
// This is the only time the raw key is visible
type APIKeyResponse struct {
	APIKey
	RawKey string `json:"key"` // Only returned on creation
}

// APIKeyInfo contains validated API key information
type APIKeyInfo struct {
	Key    *APIKey
	User   *User
	Team   *Team
	Scopes []APIKeyScope
}

// Session represents an authenticated session (for TUI/Dashboard)
type Session struct {
	ID         uuid.UUID      `json:"id"`
	UserID     uuid.UUID      `json:"user_id"`
	TokenHash  string         `json:"-"`
	DeviceInfo map[string]any `json:"device_info,omitempty"`
	IPAddress  string         `json:"ip_address,omitempty"`
	ExpiresAt  time.Time      `json:"expires_at"`
	CreatedAt  time.Time      `json:"created_at"`

	// Populated by joins
	User *User `json:"user,omitempty"`
}

// SessionCreate contains fields for creating a new session
type SessionCreate struct {
	UserID     uuid.UUID
	TokenHash  string
	DeviceInfo map[string]any
	IPAddress  string
	ExpiresAt  time.Time
}

// IsExpired returns true if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// HasScope returns true if the API key has the specified scope
func (k *APIKey) HasScope(scope APIKeyScope) bool {
	for _, s := range k.Scopes {
		if s == scope || s == APIKeyScopeAdmin {
			return true
		}
	}
	return false
}

// IsExpired returns true if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
