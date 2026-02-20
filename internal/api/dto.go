package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// CreateJobRequest is the request to create a new job
type CreateJobRequest struct {
	Image        string               `json:"image"`
	Command      []string             `json:"command,omitempty"`
	Env          map[string]string    `json:"env,omitempty"`
	Outputs      []string             `json:"outputs"`
	Resources    *ResourcesRequest    `json:"resources,omitempty"`
	TimeoutSecs  *int                 `json:"timeout_seconds,omitempty"`
	RegistryAuth *RegistryAuthRequest `json:"registry_auth,omitempty"`
	Metadata     map[string]any       `json:"metadata,omitempty"`
}

// ResourcesRequest specifies resource limits
type ResourcesRequest struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// RegistryAuthRequest contains registry credentials
type RegistryAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// JobResponse is the response for a job
type JobResponse struct {
	ID            uuid.UUID         `json:"id"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CreatedBy     string            `json:"created_by"`
	Status        string            `json:"status"`
	StatusMessage *string           `json:"status_message,omitempty"`
	Image         string            `json:"image"`
	ImageDigest   *string           `json:"image_digest,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	CPULimit      *string           `json:"cpu_limit,omitempty"`
	MemoryLimit   *string           `json:"memory_limit,omitempty"`
	TimeoutSecs   int               `json:"timeout_seconds"`
	Outputs       []string          `json:"outputs"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty"`
	ExitCode      *int              `json:"exit_code,omitempty"`
	LogURL        *string           `json:"log_url,omitempty"`
	Executor      string            `json:"executor"`
	ExecutorRef   *string           `json:"executor_ref,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// ListJobsResponse is the response for listing jobs
type ListJobsResponse struct {
	Jobs   []JobResponse `json:"jobs"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// ArtefactResponse represents a single artefact
type ArtefactResponse struct {
	Path        string `json:"path"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
	DownloadURL string `json:"download_url"`
}

// ListArtefactsResponse is the response for listing artefacts
type ListArtefactsResponse struct {
	JobID     uuid.UUID          `json:"job_id"`
	Artefacts []ArtefactResponse `json:"artefacts"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// toJobResponse converts a domain.Job to JobResponse
func toJobResponse(job *domain.Job, baseURL string) JobResponse {
	resp := JobResponse{
		ID:            job.ID,
		CreatedAt:     job.CreatedAt,
		UpdatedAt:     job.UpdatedAt,
		CreatedBy:     job.CreatedBy,
		Status:        string(job.Status),
		StatusMessage: job.StatusMessage,
		Image:         job.Image,
		ImageDigest:   job.ImageDigest,
		Command:       job.Command,
		Env:           job.Env,
		CPULimit:      job.CPULimit,
		MemoryLimit:   job.MemoryLimit,
		TimeoutSecs:   job.TimeoutSecs,
		Outputs:       job.Outputs,
		StartedAt:     job.StartedAt,
		FinishedAt:    job.FinishedAt,
		ExitCode:      job.ExitCode,
		Executor:      string(job.Executor),
		ExecutorRef:   job.ExecutorRef,
		Metadata:      job.Metadata,
	}

	// Add log URL if available
	if job.LogObjectKey != nil {
		logURL := baseURL + "/v1/jobs/" + job.ID.String() + "/logs"
		resp.LogURL = &logURL
	}

	return resp
}
