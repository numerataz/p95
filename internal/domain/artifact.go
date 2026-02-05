package domain

import (
	"time"

	"github.com/google/uuid"
)

// Artifact represents a file artifact associated with a run
type Artifact struct {
	ID          uuid.UUID      `json:"id"`
	RunID       uuid.UUID      `json:"run_id"`
	Name        string         `json:"name"`
	Path        string         `json:"path"` // Storage path (S3, local, etc.)
	SizeBytes   *int64         `json:"size_bytes,omitempty"`
	ContentType *string        `json:"content_type,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// ArtifactCreate contains fields for creating a new artifact
type ArtifactCreate struct {
	Name        string         `json:"name" validate:"required,min=1,max=255"`
	Path        string         `json:"path" validate:"required"`
	SizeBytes   *int64         `json:"size_bytes,omitempty"`
	ContentType *string        `json:"content_type,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ArtifactListOptions contains options for listing artifacts
type ArtifactListOptions struct {
	RunID  uuid.UUID
	Limit  int
	Offset int
}
