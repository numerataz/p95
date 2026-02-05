package domain

import (
	"time"

	"github.com/google/uuid"
)

// Continuation represents a point where a run was resumed with potential config changes.
// This enables tracking of training checkpoints, hyperparameter adjustments, and resume events.
type Continuation struct {
	ID           uuid.UUID      `json:"id"`
	RunID        uuid.UUID      `json:"run_id"`
	Step         int64          `json:"step"`                     // Metric step at continuation
	Timestamp    time.Time      `json:"timestamp"`                // When the continuation occurred
	ConfigBefore map[string]any `json:"config_before,omitempty"`  // Config snapshot before continuation
	ConfigAfter  map[string]any `json:"config_after,omitempty"`   // New config after continuation
	Note         *string        `json:"note,omitempty"`           // Optional user note
	GitInfo      *GitInfo       `json:"git_info,omitempty"`       // Git info at continuation time
	SystemInfo   *SystemInfo    `json:"system_info,omitempty"`    // System info at continuation time
	CreatedAt    time.Time      `json:"created_at"`
}

// ContinuationCreate contains fields for creating a new continuation
type ContinuationCreate struct {
	Step         int64          `json:"step"`
	ConfigBefore map[string]any `json:"config_before,omitempty"`
	ConfigAfter  map[string]any `json:"config_after,omitempty"`
	Note         *string        `json:"note,omitempty"`
	GitInfo      *GitInfo       `json:"git_info,omitempty"`
	SystemInfo   *SystemInfo    `json:"system_info,omitempty"`
}

// ResumeRunRequest contains fields for resuming a run
type ResumeRunRequest struct {
	Config     map[string]any `json:"config,omitempty"`      // New/updated config (merged with existing)
	Note       *string        `json:"note,omitempty"`        // Optional continuation note
	GitInfo    *GitInfo       `json:"git_info,omitempty"`    // Git info at resume time
	SystemInfo *SystemInfo    `json:"system_info,omitempty"` // System info at resume time
}

// ResumeRunResponse contains the response for a resume operation
type ResumeRunResponse struct {
	Run          *Run          `json:"run"`
	Continuation *Continuation `json:"continuation"`
}

// ConfigDiff returns the keys that changed between config_before and config_after
func (c *Continuation) ConfigDiff() map[string]ConfigChange {
	diff := make(map[string]ConfigChange)

	// Find changed or removed keys
	for k, before := range c.ConfigBefore {
		if after, exists := c.ConfigAfter[k]; exists {
			// Check if value changed
			if !configValuesEqual(before, after) {
				diff[k] = ConfigChange{
					Before: before,
					After:  after,
					Type:   ConfigChangeTypeModified,
				}
			}
		} else {
			diff[k] = ConfigChange{
				Before: before,
				After:  nil,
				Type:   ConfigChangeTypeRemoved,
			}
		}
	}

	// Find added keys
	for k, after := range c.ConfigAfter {
		if _, exists := c.ConfigBefore[k]; !exists {
			diff[k] = ConfigChange{
				Before: nil,
				After:  after,
				Type:   ConfigChangeTypeAdded,
			}
		}
	}

	return diff
}

// ConfigChangeType represents the type of config change
type ConfigChangeType string

const (
	ConfigChangeTypeAdded    ConfigChangeType = "added"
	ConfigChangeTypeModified ConfigChangeType = "modified"
	ConfigChangeTypeRemoved  ConfigChangeType = "removed"
)

// ConfigChange represents a single config value change
type ConfigChange struct {
	Before any              `json:"before,omitempty"`
	After  any              `json:"after,omitempty"`
	Type   ConfigChangeType `json:"type"`
}

// configValuesEqual checks if two config values are equal
func configValuesEqual(a, b any) bool {
	// Simple comparison - handles primitives
	// For complex types, this may need enhancement
	return a == b
}
