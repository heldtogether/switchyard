package domain

import (
	"time"

	"github.com/google/uuid"
)

// RunStatus represents the lifecycle state of a run
type RunStatus string

const (
	RunStatusPending    RunStatus = "PENDING"    // Run created, jobs not yet started
	RunStatusCancelling RunStatus = "CANCELLING" // Cancellation requested, jobs are stopping
	RunStatusRunning    RunStatus = "RUNNING"    // At least one job running
	RunStatusSucceeded  RunStatus = "SUCCEEDED"  // All jobs succeeded
	RunStatusFailed     RunStatus = "FAILED"     // At least one job failed
	RunStatusCancelled  RunStatus = "CANCELLED"  // Run was cancelled
	RunStatusPartial    RunStatus = "PARTIAL"    // Some jobs succeeded, some failed
)

// IsTerminal returns true if the status is a final state
func (s RunStatus) IsTerminal() bool {
	return s == RunStatusSucceeded || s == RunStatusFailed || s == RunStatusCancelled || s == RunStatusPartial
}

// Run represents a single execution intent within a project
type Run struct {
	ID          uuid.UUID      `json:"id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	Slug        string         `json:"slug"` // URL-friendly identifier unique within project
	Name        string         `json:"name"` // Human-readable name
	Description *string        `json:"description,omitempty"`
	Status      RunStatus      `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	CreatedBy   string         `json:"created_by"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Duration returns the run execution duration if available
func (r *Run) Duration() *time.Duration {
	if r.StartedAt != nil && r.FinishedAt != nil {
		d := r.FinishedAt.Sub(*r.StartedAt)
		return &d
	}
	return nil
}
