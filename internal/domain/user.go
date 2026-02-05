package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user account in the system
type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never expose in JSON
	Name         string     `json:"name"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	IsActive     bool       `json:"is_active"`
	IsAdmin      bool       `json:"is_admin"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// UserCreate contains fields for creating a new user
type UserCreate struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=1,max=255"`
}

// UserUpdate contains fields for updating a user
type UserUpdate struct {
	Name      *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	AvatarURL *string `json:"avatar_url,omitempty" validate:"omitempty,url"`
}

// UserResponse is the public representation of a user
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

// ToResponse converts a User to UserResponse
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
		IsAdmin:   u.IsAdmin,
		CreatedAt: u.CreatedAt,
	}
}
