package domain

import (
	"time"

	"github.com/google/uuid"
)

// Artefact represents a file output from a job
type Artefact struct {
	ID          uuid.UUID `json:"id"`
	JobID       uuid.UUID `json:"job_id"`
	Path        string    `json:"path"`       // Relative path in outputs (e.g., "result.txt", "data/metrics.json")
	ObjectKey   string    `json:"object_key"` // Full S3 key
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
}

// ArtefactInfo is a lighter version for listing
type ArtefactInfo struct {
	Path        string `json:"path"`
	ObjectKey   string `json:"object_key"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}
