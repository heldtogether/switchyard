package domain

import (
	"time"

	"github.com/google/uuid"
)

// Project represents a product line, study, or pipeline area
type Project struct {
	ID          uuid.UUID      `json:"id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	Slug        string         `json:"slug"` // URL-friendly identifier unique within workspace
	Name        string         `json:"name"` // Human-readable name
	Description *string        `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   string         `json:"created_by"`
	Archived    bool           `json:"archived"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// IsActive returns true if the project is not archived
func (p *Project) IsActive() bool {
	return !p.Archived
}
