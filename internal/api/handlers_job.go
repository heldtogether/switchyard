package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/control"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
)

// HandleCreateJob handles POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs
func (a *API) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Validate request
	if req.Image == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "image is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.RegistryAuth != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "registry_auth is not supported; use registry_secret_id",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if len(req.Outputs) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "at least one output path is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Validate environment variables
	if err := validateEnvVars(req.Env); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Get workspace
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to get default workspace",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	// Get project
	project, err := a.store.GetProjectBySlug(r.Context(), workspace.ID, projectSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Project not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if _, ok := a.requireProjectAccess(w, r, workspace, project); !ok {
		return
	}

	// Get run
	run, err := a.store.GetRunBySlug(r.Context(), project.ID, runSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Run not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Create job
	defaults := a.cfg.Executor.Docker.Defaults
	executorType := domain.ExecutorTypeDocker
	switch a.cfg.Executor.Type {
	case "docker":
		defaults = a.cfg.Executor.Docker.Defaults
		executorType = domain.ExecutorTypeDocker
	case "kube":
		defaults = a.cfg.Executor.Docker.Defaults
		executorType = domain.ExecutorTypeKube
	}

	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		Name:        req.Name,
		CreatedBy:   ActorFromRequest(r),
		Status:      domain.JobStatusPending,
		Image:       req.Image,
		Command:     req.Command,
		Env:         req.Env,
		Outputs:     req.Outputs,
		TimeoutSecs: int(defaults.Timeout.Seconds()),
		Executor:    executorType,
		Metadata:    req.Metadata,
	}

	// Set timeout if provided
	if req.TimeoutSecs != nil {
		job.TimeoutSecs = *req.TimeoutSecs
	}

	// Set resources if provided
	gpuCount := 0
	if req.Resources != nil {
		if req.Resources.GPU < 0 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "gpu must be >= 0",
				Code:    http.StatusBadRequest,
			})
			return
		}
		if req.Resources.CPU != "" {
			job.CPULimit = &req.Resources.CPU
		}
		if req.Resources.Memory != "" {
			job.MemoryLimit = &req.Resources.Memory
		}
		gpuCount = req.Resources.GPU

		// If GPU is requested without CPU/Memory, apply defaults.
		if gpuCount > 0 && req.Resources.CPU == "" && req.Resources.Memory == "" {
			cpu := defaults.Resources.CPU
			mem := defaults.Resources.Memory
			job.CPULimit = &cpu
			job.MemoryLimit = &mem
		}
	} else {
		// Use defaults from config
		cpu := defaults.Resources.CPU
		mem := defaults.Resources.Memory
		job.CPULimit = &cpu
		job.MemoryLimit = &mem
	}

	if gpuCount > 0 {
		maxGPU, err := a.store.MaxGPUPerNode(r.Context())
		if err != nil {
			a.logger.Error("failed to fetch max gpu per node", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to validate GPU capacity",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		if maxGPU == 0 {
			writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{
				Error:   "validation_error",
				Message: "No GPU-capable nodes registered",
				Code:    http.StatusUnprocessableEntity,
			})
			return
		}
		if gpuCount > maxGPU {
			writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{
				Error:   "validation_error",
				Message: fmt.Sprintf("gpu request exceeds max GPUs on a single node (%d)", maxGPU),
				Code:    http.StatusUnprocessableEntity,
			})
			return
		}
	}

	job.GPUCount = gpuCount

	if req.RegistrySecretID != nil {
		if _, err := a.store.GetActiveRegistrySecretForWorkspace(r.Context(), workspace.ID, *req.RegistrySecretID); err != nil {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: "Registry secret not found or inactive",
				Code:    http.StatusNotFound,
			})
			return
		}
		job.RegistrySecretID = req.RegistrySecretID
	}

	// Insert job into database
	if err := a.store.CreateJob(r.Context(), job); err != nil {
		a.logger.Error("failed to create job", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create job",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Push to queue
	if err := a.queue.Publish(r.Context(), job.ID.String(), job.GPUCount); err != nil {
		a.logger.Error("failed to push job to queue", "job_id", job.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to queue job",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	a.logger.Info("job created", "job_id", job.ID, "run", runSlug, "project", projectSlug, "workspace", workspaceSlug, "image", job.Image)

	writeJSON(w, http.StatusCreated, toJobResponse(job, a.baseURL, workspaceSlug, projectSlug, runSlug))
}

// HandleGetJob handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}
func (a *API) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")
	jobID := r.PathValue("job_id")

	id, err := uuid.Parse(jobID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	job, err := a.store.GetJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Job not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if !a.authorizeJobPathScope(w, r, workspaceSlug, projectSlug, runSlug, job.RunID) {
		return
	}

	writeJSON(w, http.StatusOK, toJobResponse(job, a.baseURL, workspaceSlug, projectSlug, runSlug))
}

// HandleListJobs handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs
func (a *API) HandleListJobs(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	statusStr := r.URL.Query().Get("status")

	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var status *domain.JobStatus
	if statusStr != "" {
		s := domain.JobStatus(statusStr)
		status = &s
	}

	// Get workspace
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to get default workspace",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Get project
	project, err := a.store.GetProjectBySlug(r.Context(), workspace.ID, projectSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Project not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Get run
	run, err := a.store.GetRunBySlug(r.Context(), project.ID, runSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Run not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	jobs, err := a.store.ListJobs(r.Context(), &run.ID, status, nil, limit, offset)
	if err != nil {
		a.logger.Error("failed to list jobs", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list jobs",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	responses := make([]JobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = toJobResponse(job, a.baseURL, workspaceSlug, projectSlug, runSlug)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"jobs":   responses,
		"limit":  limit,
		"offset": offset,
	})
}

// HandleGetJobLogs handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/logs
func (a *API) HandleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")
	jobID := r.PathValue("job_id")

	id, err := uuid.Parse(jobID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	job, err := a.store.GetJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Job not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if !a.authorizeJobPathScope(w, r, workspaceSlug, projectSlug, runSlug, job.RunID) {
		return
	}

	if job.LogObjectKey == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Logs not available yet",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Stream logs from S3
	reader, err := a.storage.Download(r.Context(), *job.LogObjectKey)
	if err != nil {
		a.logger.Error("failed to get logs from storage", "error", err, "key", *job.LogObjectKey)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve logs",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

// HandleListArtefacts handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts
func (a *API) HandleListArtefacts(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")
	jobID := r.PathValue("job_id")

	id, err := uuid.Parse(jobID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}
	job, err := a.store.GetJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Job not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if !a.authorizeJobPathScope(w, r, workspaceSlug, projectSlug, runSlug, job.RunID) {
		return
	}

	artefacts, err := a.store.GetArtefacts(r.Context(), id)
	if err != nil {
		a.logger.Error("failed to get artefacts", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve artefacts",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	responses := make([]ArtefactResponse, len(artefacts))
	for i, art := range artefacts {
		downloadURL := fmt.Sprintf("%s/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/artefacts/%s",
			a.baseURL, workspaceSlug, projectSlug, runSlug, jobID, art.Path)
		responses[i] = ArtefactResponse{
			Path:        art.Path,
			SizeBytes:   art.SizeBytes,
			ContentType: art.ContentType,
			DownloadURL: downloadURL,
		}
	}

	writeJSON(w, http.StatusOK, ListArtefactsResponse{
		JobID:     id,
		Artefacts: responses,
	})
}

// HandleDownloadArtefact handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts/{path...}
func (a *API) HandleDownloadArtefact(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")
	jobID := r.PathValue("job_id")
	artefactPath := r.PathValue("path")

	// Decode the path (it might be URL encoded)
	artefactPath = strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/artefacts/",
		workspaceSlug, projectSlug, runSlug, jobID))

	id, err := uuid.Parse(jobID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}
	job, err := a.store.GetJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Job not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if !a.authorizeJobPathScope(w, r, workspaceSlug, projectSlug, runSlug, job.RunID) {
		return
	}

	// Get artefacts for this job
	artefacts, err := a.store.GetArtefacts(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve artefacts",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Find the artefact
	var objectKey string
	var contentType string
	for _, art := range artefacts {
		if art.Path == artefactPath {
			objectKey = art.ObjectKey
			contentType = art.ContentType
			break
		}
	}

	if objectKey == "" {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Artefact not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Stream artefact from S3
	reader, err := a.storage.Download(r.Context(), objectKey)
	if err != nil {
		a.logger.Error("failed to get artefact from storage", "error", err, "key", objectKey)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve artefact",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer reader.Close()

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

// HandleCancelJob handles POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/cancel
func (a *API) HandleCancelJob(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")
	jobID := r.PathValue("job_id")

	id, err := uuid.Parse(jobID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	job, err := a.store.GetJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Job not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if !a.authorizeJobPathScope(w, r, workspaceSlug, projectSlug, runSlug, job.RunID) {
		return
	}

	// Check if job is in a cancellable state
	if job.Status.IsTerminal() {
		_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
			JobID:       job.ID,
			EventType:   "skipped_terminal",
			RequestedBy: ActorFromRequest(r),
			Message:     strPtr("job already in terminal state"),
		})
		writeJSON(w, http.StatusConflict, ErrorResponse{
			Error:   "cannot_cancel",
			Message: fmt.Sprintf("Job is already in terminal state: %s", job.Status),
			Code:    http.StatusConflict,
		})
		return
	}

	actor := ActorFromRequest(r)

	// Pending jobs are immediately cancellable.
	if job.Status == domain.JobStatusPending {
		message := "Job cancelled before execution"
		if err := a.store.UpdateJobStatus(r.Context(), id, domain.JobStatusCancelled, &message); err != nil {
			a.logger.Error("failed to update job status", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to cancel job",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		if err := a.store.RecomputeRunStatus(r.Context(), job.RunID); err != nil {
			a.logger.Error("failed to update run status after cancel", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to update run status",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
			JobID:       job.ID,
			EventType:   "requested",
			RequestedBy: actor,
			Message:     strPtr("pending job cancellation requested"),
		})
		_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
			JobID:       job.ID,
			EventType:   "completed",
			RequestedBy: actor,
			Message:     strPtr("pending job cancellation completed"),
		})
		updatedJob, err := a.store.GetJob(r.Context(), id)
		if err != nil {
			a.logger.Error("failed to get updated job", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to retrieve cancelled job",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		run, err := a.store.GetRun(r.Context(), updatedJob.RunID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve run", Code: http.StatusInternalServerError})
			return
		}
		project, err := a.store.GetProject(r.Context(), run.ProjectID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve project", Code: http.StatusInternalServerError})
			return
		}
		workspace, err := a.store.GetWorkspace(r.Context(), project.WorkspaceID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve workspace", Code: http.StatusInternalServerError})
			return
		}
		writeJSON(w, http.StatusOK, toJobResponse(updatedJob, a.baseURL, workspace.Slug, project.Slug, run.Slug))
		return
	}

	// Push-based async cancellation for running jobs when node affinity and control publisher are available.
	if (job.Status == domain.JobStatusRunning || job.Status == domain.JobStatusCancelling) &&
		a.cancelPub != nil && job.AssignedNodeID != nil && *job.AssignedNodeID != "" {
		message := "Job cancellation requested"
		if err := a.store.MarkJobCancelling(r.Context(), job.ID, actor, "user_requested", &message); err != nil {
			a.logger.Error("failed to mark job cancelling", "job_id", job.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "cancel_failed",
				Message: "Failed to request job cancellation",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		if err := a.store.RecomputeRunStatus(r.Context(), job.RunID); err != nil {
			a.logger.Error("failed to recompute run status after cancelling", "job_id", job.ID, "error", err)
		}
		_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
			JobID:       job.ID,
			EventType:   "requested",
			RequestedBy: actor,
			NodeID:      job.AssignedNodeID,
			ExecutorRef: job.ExecutorRef,
			Message:     strPtr("running job cancellation requested"),
		})

		signal := control.CancelSignal{
			JobID:       job.ID,
			RunID:       job.RunID,
			RequestedBy: actor,
			RequestedAt: time.Now().UTC(),
		}
		if err := a.cancelPub.PublishJobCancel(r.Context(), *job.AssignedNodeID, signal); err != nil {
			_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
				JobID:       job.ID,
				EventType:   "failed",
				RequestedBy: actor,
				NodeID:      job.AssignedNodeID,
				ExecutorRef: job.ExecutorRef,
				Message:     strPtr(fmt.Sprintf("failed to dispatch cancellation signal: %v", err)),
			})
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "cancel_failed",
				Message: "Failed to dispatch cancellation request",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
			JobID:       job.ID,
			EventType:   "dispatched",
			RequestedBy: actor,
			NodeID:      job.AssignedNodeID,
			ExecutorRef: job.ExecutorRef,
			Message:     strPtr("cancellation signal dispatched to worker"),
		})

		updatedJob, err := a.store.GetJob(r.Context(), id)
		if err != nil {
			a.logger.Error("failed to get updated job", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to retrieve cancelled job",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		run, err := a.store.GetRun(r.Context(), updatedJob.RunID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve run", Code: http.StatusInternalServerError})
			return
		}
		project, err := a.store.GetProject(r.Context(), run.ProjectID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve project", Code: http.StatusInternalServerError})
			return
		}
		workspace, err := a.store.GetWorkspace(r.Context(), project.WorkspaceID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to retrieve workspace", Code: http.StatusInternalServerError})
			return
		}
		writeJSON(w, http.StatusAccepted, toJobResponse(updatedJob, a.baseURL, workspace.Slug, project.Slug, run.Slug))
		return
	}

	// Fallback to in-process executor cancellation when no control publisher is configured.
	if job.Status == domain.JobStatusRunning {
		// Running job must have an executor reference
		if job.ExecutorRef == nil || *job.ExecutorRef == "" {
			a.logger.Error("running job has no executor reference", "job_id", job.ID)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "cancel_failed",
				Message: "Running job has no executor reference",
				Code:    http.StatusInternalServerError,
			})
			return
		}

		runRef := executor.RunRef{
			ExecutorType: string(job.Executor),
			Reference:    *job.ExecutorRef,
		}
		if err := a.executor.Cancel(r.Context(), runRef); err != nil {
			a.logger.Error("failed to cancel job via executor", "job_id", job.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "cancel_failed",
				Message: fmt.Sprintf("Failed to cancel job: %s", err.Error()),
				Code:    http.StatusInternalServerError,
			})
			return
		}
	}

	message := "Job cancelled by user"

	// Update job status
	if err := a.store.UpdateJobStatus(r.Context(), id, domain.JobStatusCancelled, &message); err != nil {
		a.logger.Error("failed to update job status", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to cancel job",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	if err := a.store.RecomputeRunStatus(r.Context(), job.RunID); err != nil {
		a.logger.Error("failed to update run status after cancel", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to update run status",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
		JobID:       job.ID,
		EventType:   "completed",
		RequestedBy: actor,
		NodeID:      job.AssignedNodeID,
		ExecutorRef: job.ExecutorRef,
		Message:     strPtr("job cancelled via synchronous fallback"),
	})

	// Get updated job to return full details
	updatedJob, err := a.store.GetJob(r.Context(), id)
	if err != nil {
		a.logger.Error("failed to get updated job", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve cancelled job",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Get workspace/project/run slugs for URL generation
	run, err := a.store.GetRun(r.Context(), updatedJob.RunID)
	if err != nil {
		a.logger.Error("failed to get run", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve run",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	project, err := a.store.GetProject(r.Context(), run.ProjectID)
	if err != nil {
		a.logger.Error("failed to get project", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve project",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	workspace, err := a.store.GetWorkspace(r.Context(), project.WorkspaceID)
	if err != nil {
		a.logger.Error("failed to get workspace", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve workspace",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, toJobResponse(updatedJob, a.baseURL, workspace.Slug, project.Slug, run.Slug))
}

func (a *API) authorizeJobPathScope(w http.ResponseWriter, r *http.Request, workspaceSlug, projectSlug, runSlug string, runID uuid.UUID) bool {
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
		})
		return false
	}
	project, err := a.store.GetProjectBySlug(r.Context(), workspace.ID, projectSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Project not found",
			Code:    http.StatusNotFound,
		})
		return false
	}
	if _, ok := a.requireProjectAccess(w, r, workspace, project); !ok {
		return false
	}

	run, err := a.store.GetRunBySlug(r.Context(), project.ID, runSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Run not found",
			Code:    http.StatusNotFound,
		})
		return false
	}
	if run.ID != runID {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Resource not found",
			Code:    http.StatusNotFound,
		})
		return false
	}
	return true
}
