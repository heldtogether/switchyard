package domain

import (
	"time"

	"github.com/google/uuid"
)

// Workspace represents a tenant or organization
type Workspace struct {
	ID          uuid.UUID      `json:"id"`
	Slug        string         `json:"slug"` // URL-friendly identifier (e.g., "default", "acme-corp")
	Name        string         `json:"name"` // Human-readable name
	Description *string        `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
