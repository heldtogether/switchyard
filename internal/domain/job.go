package domain

import (
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the lifecycle state of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "PENDING"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusSucceeded JobStatus = "SUCCEEDED"
	JobStatusFailed    JobStatus = "FAILED"
	JobStatusCancelled JobStatus = "CANCELLED"
	JobStatusTimeout   JobStatus = "TIMEOUT"
)

// IsTerminal returns true if the status is a final state
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusSucceeded || s == JobStatusFailed || s == JobStatusCancelled || s == JobStatusTimeout
}

// ExecutorType represents the execution backend
type ExecutorType string

const (
	ExecutorTypeDocker ExecutorType = "docker"
	ExecutorTypeSwarm  ExecutorType = "swarm"
	ExecutorTypeKube   ExecutorType = "kube"
)

// Job represents a container execution job
type Job struct {
	ID        uuid.UUID `json:"id"`
	RunID     uuid.UUID `json:"run_id"`
	Name      *string   `json:"name,omitempty"` // Optional human-readable name
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"`

	// Status
	Status        JobStatus `json:"status"`
	StatusMessage *string   `json:"status_message,omitempty"`

	// Container specification
	Image       string            `json:"image"`
	ImageDigest *string           `json:"image_digest,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Env         map[string]string `json:"env,omitempty"`

	// Resources
	CPULimit    *string `json:"cpu_limit,omitempty"`
	MemoryLimit *string `json:"memory_limit,omitempty"`
	GPUCount    int     `json:"gpu_count,omitempty"`
	TimeoutSecs int     `json:"timeout_seconds"`

	// Outputs
	Outputs []string `json:"outputs"`

	// Execution tracking
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`

	// Storage references
	ArtefactPrefix *string `json:"artefact_prefix,omitempty"`
	LogObjectKey   *string `json:"log_object_key,omitempty"`

	// Executor details
	Executor         ExecutorType   `json:"executor"`
	ExecutorRef      *string        `json:"executor_ref,omitempty"`
	ExecutorMetadata map[string]any `json:"executor_metadata,omitempty"`

	// Scheduling
	AssignedNodeID *string `json:"assigned_node_id,omitempty"`

	// Registry auth
	RegistrySecretID *uuid.UUID `json:"registry_secret_id,omitempty"`

	// Extensibility
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Duration returns the job execution duration if available
func (j *Job) Duration() *time.Duration {
	if j.StartedAt != nil && j.FinishedAt != nil {
		d := j.FinishedAt.Sub(*j.StartedAt)
		return &d
	}
	return nil
}

// ResourceSpec holds resource configuration for a job
type ResourceSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	GPU    int    `json:"gpu,omitempty"`
}

// RegistryAuth holds authentication for private registries
type RegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
