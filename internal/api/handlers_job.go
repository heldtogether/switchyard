package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
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
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		Name:        req.Name,
		CreatedBy:   "api-key-user", // TODO: Get from auth context
		Status:      domain.JobStatusPending,
		Image:       req.Image,
		Command:     req.Command,
		Env:         req.Env,
		Outputs:     req.Outputs,
		TimeoutSecs: int(a.cfg.Executor.Swarm.Defaults.Timeout.Seconds()),
		Executor:    domain.ExecutorTypeSwarm,
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
	} else {
		// Use defaults from config
		cpu := a.cfg.Executor.Swarm.Defaults.Resources.CPU
		mem := a.cfg.Executor.Swarm.Defaults.Resources.Memory
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
	// Note: workspace_slug, project_slug, run_slug are in path but not needed for this operation
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
	// Note: workspace_slug, project_slug, run_slug are in path but not needed for cancel operation
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

	// Check if job is in a cancellable state
	if job.Status.IsTerminal() {
		writeJSON(w, http.StatusConflict, ErrorResponse{
			Error:   "cannot_cancel",
			Message: fmt.Sprintf("Job is already in terminal state: %s", job.Status),
			Code:    http.StatusConflict,
		})
		return
	}

	// Cancel the job via executor if it's running
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

	// Determine the cancellation message based on current status
	var message string
	if job.Status == domain.JobStatusPending {
		message = "Job cancelled before execution"
	} else {
		message = "Job cancelled by user"
	}

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
