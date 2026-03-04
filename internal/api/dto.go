package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// ========== Workspace DTOs ==========

// CreateWorkspaceRequest is the request to create a new workspace
type CreateWorkspaceRequest struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// WorkspaceResponse is the response for a workspace
type WorkspaceResponse struct {
	ID          uuid.UUID      `json:"id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ========== Project DTOs ==========

// CreateProjectRequest is the request to create a new project
type CreateProjectRequest struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// UpdateProjectRequest is the request to update a project
type UpdateProjectRequest struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ProjectResponse is the response for a project
type ProjectResponse struct {
	ID          uuid.UUID      `json:"id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   string         `json:"created_by"`
	Archived    bool           `json:"archived"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ========== Run DTOs ==========

// CreateRunRequest is the request to create a new run
type CreateRunRequest struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// RerunRunRequest is the request to clone an existing run
type RerunRunRequest struct {
	Mode string `json:"mode"` // "all" | "failed_only"
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
}

// RunResponse is the response for a run
type RunResponse struct {
	ID          uuid.UUID      `json:"id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	CreatedBy   string         `json:"created_by"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// RerunRunResponse is the response for a rerun action
type RerunRunResponse struct {
	Run         RunResponse `json:"run"`
	JobsCreated int         `json:"jobs_created"`
	SourceRunID uuid.UUID   `json:"source_run_id"`
	Mode        string      `json:"mode"`
}

// ========== Job DTOs ==========

// CreateJobRequest is the request to create a new job
type CreateJobRequest struct {
	Name             *string              `json:"name,omitempty"`
	Image            string               `json:"image"`
	Command          []string             `json:"command,omitempty"`
	Env              map[string]string    `json:"env,omitempty"`
	Outputs          []string             `json:"outputs"`
	Resources        *ResourcesRequest    `json:"resources,omitempty"`
	TimeoutSecs      *int                 `json:"timeout_seconds,omitempty"`
	RegistryAuth     *RegistryAuthRequest `json:"registry_auth,omitempty"`
	RegistrySecretID *uuid.UUID           `json:"registry_secret_id,omitempty"`
	Metadata         map[string]any       `json:"metadata,omitempty"`
}

// ResourcesRequest specifies resource limits
type ResourcesRequest struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	GPU    int    `json:"gpu,omitempty"`
}

// RegistryAuthRequest contains registry credentials
type RegistryAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateRegistrySecretRequest contains registry secret details
type CreateRegistrySecretRequest struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegistrySecretResponse represents a registry secret (without password)
type RegistrySecretResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	Host      string    `json:"host"`
	Username  string    `json:"username"`
	Active    bool      `json:"active"`
}

// JobResponse is the response for a job
type JobResponse struct {
	ID             uuid.UUID         `json:"id"`
	Name           *string           `json:"name,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	CreatedBy      string            `json:"created_by"`
	Status         string            `json:"status"`
	StatusMessage  *string           `json:"status_message,omitempty"`
	Image          string            `json:"image"`
	ImageDigest    *string           `json:"image_digest,omitempty"`
	Command        []string          `json:"command,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	CPULimit       *string           `json:"cpu_limit,omitempty"`
	MemoryLimit    *string           `json:"memory_limit,omitempty"`
	GPUCount       int               `json:"gpu_count,omitempty"`
	TimeoutSecs    int               `json:"timeout_seconds"`
	Outputs        []string          `json:"outputs"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	FinishedAt     *time.Time        `json:"finished_at,omitempty"`
	ExitCode       *int              `json:"exit_code,omitempty"`
	LogURL         *string           `json:"log_url,omitempty"`
	Executor       string            `json:"executor"`
	ExecutorRef    *string           `json:"executor_ref,omitempty"`
	AssignedNodeID *string           `json:"assigned_node_id,omitempty"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
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

// ========== Worker & Allocation DTOs ==========

// RegisterWorkerRequest registers a worker node
type RegisterWorkerRequest struct {
	NodeID   string            `json:"node_id"`
	Hostname string            `json:"hostname"`
	Executor string            `json:"executor"`
	GPUTotal int               `json:"gpu_total"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// RegisterWorkerResponse is returned on successful registration
type RegisterWorkerResponse struct {
	NodeID       string    `json:"node_id"`
	RegisteredAt time.Time `json:"registered_at"`
}

// WorkerHeartbeatRequest updates worker heartbeat
type WorkerHeartbeatRequest struct {
	NodeID   string `json:"node_id"`
	GPUTotal int    `json:"gpu_total"`
}

// AllocationClaimRequest claims GPU allocation for a job on a node
type AllocationClaimRequest struct {
	JobID  uuid.UUID `json:"job_id"`
	NodeID string    `json:"node_id"`
}

// AllocationClaimResponse represents an allocation claim
type AllocationClaimResponse struct {
	AllocationID uuid.UUID `json:"allocation_id"`
	NodeID       string    `json:"node_id"`
	GPUCount     int       `json:"gpu_count"`
}

// AllocationReleaseRequest releases an allocation
type AllocationReleaseRequest struct {
	JobID  uuid.UUID `json:"job_id"`
	NodeID string    `json:"node_id"`
}

// AllocationCapacityResponse reports max GPU capacity per node.
type AllocationCapacityResponse struct {
	MaxGPUPerNode int `json:"max_gpu_per_node"`
}

type CreateInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

type CreateInviteResponse struct {
	InviteID    uuid.UUID `json:"invite_id"`
	InviteURL   string    `json:"invite_url"`
	InviteToken string    `json:"invite_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type AcceptInviteRequest struct {
	Token string `json:"token"`
}

type MemberResponse struct {
	Subject     string    `json:"subject"`
	Email       *string   `json:"email,omitempty"`
	DisplayName *string   `json:"display_name,omitempty"`
	Role        string    `json:"role"`
	AddedAt     time.Time `json:"added_at"`
}

// toWorkspaceResponse converts a domain.Workspace to WorkspaceResponse
func toWorkspaceResponse(workspace *domain.Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:          workspace.ID,
		Slug:        workspace.Slug,
		Name:        workspace.Name,
		Description: workspace.Description,
		CreatedAt:   workspace.CreatedAt,
		UpdatedAt:   workspace.UpdatedAt,
		Metadata:    workspace.Metadata,
	}
}

// toProjectResponse converts a domain.Project to ProjectResponse
func toProjectResponse(project *domain.Project) ProjectResponse {
	return ProjectResponse{
		ID:          project.ID,
		WorkspaceID: project.WorkspaceID,
		Slug:        project.Slug,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
		CreatedBy:   project.CreatedBy,
		Archived:    project.Archived,
		Metadata:    project.Metadata,
	}
}

// toRunResponse converts a domain.Run to RunResponse
func toRunResponse(run *domain.Run) RunResponse {
	return RunResponse{
		ID:          run.ID,
		ProjectID:   run.ProjectID,
		Slug:        run.Slug,
		Name:        run.Name,
		Description: run.Description,
		Status:      string(run.Status),
		CreatedAt:   run.CreatedAt,
		UpdatedAt:   run.UpdatedAt,
		StartedAt:   run.StartedAt,
		FinishedAt:  run.FinishedAt,
		CreatedBy:   run.CreatedBy,
		Metadata:    run.Metadata,
	}
}

// toJobResponse converts a domain.Job to JobResponse
func toJobResponse(job *domain.Job, baseURL string, workspaceSlug string, projectSlug string, runSlug string) JobResponse {
	resp := JobResponse{
		ID:             job.ID,
		Name:           job.Name,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		CreatedBy:      job.CreatedBy,
		Status:         string(job.Status),
		StatusMessage:  job.StatusMessage,
		Image:          job.Image,
		ImageDigest:    job.ImageDigest,
		Command:        job.Command,
		Env:            job.Env,
		CPULimit:       job.CPULimit,
		MemoryLimit:    job.MemoryLimit,
		GPUCount:       job.GPUCount,
		TimeoutSecs:    job.TimeoutSecs,
		Outputs:        job.Outputs,
		StartedAt:      job.StartedAt,
		FinishedAt:     job.FinishedAt,
		ExitCode:       job.ExitCode,
		Executor:       string(job.Executor),
		ExecutorRef:    job.ExecutorRef,
		AssignedNodeID: job.AssignedNodeID,
		Metadata:       job.Metadata,
	}

	// Add log URL if available
	if job.LogObjectKey != nil {
		logURL := fmt.Sprintf("%s/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/logs", baseURL, workspaceSlug, projectSlug, runSlug, job.ID.String())
		resp.LogURL = &logURL
	}

	return resp
}

func ptr[T any](v T) *T {
	return &v
}
