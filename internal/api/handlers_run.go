package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/control"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
)

// HandleCreateRun handles POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs
func (a *API) HandleCreateRun(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")

	var req CreateRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Validate request
	if req.Slug == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "slug is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Get workspace
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
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

	// Create run
	run := &domain.Run{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Status:      domain.RunStatusPending,
		CreatedBy:   ActorFromRequest(r),
		Metadata:    req.Metadata,
	}

	if err := a.store.CreateRun(r.Context(), run); err != nil {
		a.logger.Error("failed to create run", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create run",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	a.logger.Info("run created", "run_id", run.ID, "slug", run.Slug, "project", projectSlug, "workspace", workspaceSlug)

	writeJSON(w, http.StatusCreated, toRunResponse(run))
}

// HandleGetRun handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}
func (a *API) HandleGetRun(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")

	// Get workspace
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
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

	writeJSON(w, http.StatusOK, toRunResponse(run))
}

// HandleListRuns handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs
func (a *API) HandleListRuns(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	statusStr := r.URL.Query().Get("status")

	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var status *domain.RunStatus
	if statusStr != "" {
		s := domain.RunStatus(statusStr)
		status = &s
	}

	// Get workspace
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
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

	runs, err := a.store.ListRuns(r.Context(), project.ID, status, limit, offset)
	if err != nil {
		a.logger.Error("failed to list runs", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list runs",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	responses := make([]RunResponse, len(runs))
	for i, run := range runs {
		responses[i] = toRunResponse(run)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"runs":   responses,
		"limit":  limit,
		"offset": offset,
	})
}

// HandleRerunRun handles POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/rerun
func (a *API) HandleRerunRun(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")

	var req RerunRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	mode := strings.TrimSpace(req.Mode)
	if mode != "all" && mode != "failed_only" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "mode must be one of: all, failed_only",
			Code:    http.StatusBadRequest,
		})
		return
	}

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
		})
		return
	}
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

	sourceRun, err := a.store.GetRunBySlug(r.Context(), project.ID, runSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Run not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	jobs, err := a.store.ListJobs(r.Context(), &sourceRun.ID, nil, nil, 1000, 0)
	if err != nil {
		a.logger.Error("failed to list source run jobs", "run_id", sourceRun.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list source run jobs",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	selectedJobs := filterRerunJobs(jobs, mode)
	if len(selectedJobs) == 0 {
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{
			Error:   "validation_error",
			Message: "No jobs matched rerun criteria",
			Code:    http.StatusUnprocessableEntity,
		})
		return
	}

	newRunSlug := strings.TrimSpace(req.Slug)
	if newRunSlug == "" {
		newRunSlug = generateRerunSlug(sourceRun.Slug)
	}
	newRunName := strings.TrimSpace(req.Name)
	if newRunName == "" {
		sourceName := strings.TrimSpace(sourceRun.Name)
		if sourceName == "" {
			sourceName = sourceRun.Slug
		}
		newRunName = sourceName
	}

	metadata := mergeAnyMap(sourceRun.Metadata, map[string]any{
		"trigger":           "Manual",
		"rerun_of_run_id":   sourceRun.ID.String(),
		"rerun_of_run_slug": sourceRun.Slug,
		"rerun_mode":        mode,
	})

	newRun := &domain.Run{
		ID:          uuid.New(),
		ProjectID:   sourceRun.ProjectID,
		Slug:        newRunSlug,
		Name:        newRunName,
		Description: sourceRun.Description,
		Status:      domain.RunStatusPending,
		CreatedBy:   ActorFromRequest(r),
		Metadata:    metadata,
	}

	if err := a.store.CreateRun(r.Context(), newRun); err != nil {
		a.logger.Error("failed to create rerun", "source_run_id", sourceRun.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create rerun",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	createdCount := 0
	actor := ActorFromRequest(r)
	for _, sourceJob := range selectedJobs {
		cloned := cloneJobForRerun(sourceJob, newRun.ID, sourceRun.ID, actor)
		if err := a.store.CreateJob(r.Context(), cloned); err != nil {
			a.logger.Error("failed to clone rerun job", "source_job_id", sourceJob.ID, "new_run_id", newRun.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to clone jobs for rerun",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		if err := a.queue.Publish(r.Context(), cloned.ID.String(), cloned.GPUCount); err != nil {
			a.logger.Error("failed to enqueue cloned job", "job_id", cloned.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to queue rerun jobs",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		createdCount++
	}

	a.logger.Info("rerun created", "source_run_id", sourceRun.ID, "new_run_id", newRun.ID, "mode", mode, "jobs_created", createdCount)

	writeJSON(w, http.StatusCreated, RerunRunResponse{
		Run:         toRunResponse(newRun),
		JobsCreated: createdCount,
		SourceRunID: sourceRun.ID,
		Mode:        mode,
	})
}

// HandleCancelRun handles POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/cancel
func (a *API) HandleCancelRun(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
		})
		return
	}
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

	run, err := a.store.GetRunBySlug(r.Context(), project.ID, runSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Run not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	jobs, err := a.store.ListJobs(r.Context(), &run.ID, nil, nil, 1000, 0)
	if err != nil {
		a.logger.Error("failed to list run jobs for cancel", "run_id", run.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list run jobs",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	actor := ActorFromRequest(r)
	resp := CancelRunResponse{RunID: run.ID}
	dispatchError := false

	for _, job := range jobs {
		if job.Status.IsTerminal() {
			resp.AlreadyTerminal++
			continue
		}
		resp.TotalTargeted++

		if job.Status == domain.JobStatusPending {
			message := "Job cancelled before execution"
			if err := a.store.UpdateJobStatus(r.Context(), job.ID, domain.JobStatusCancelled, &message); err != nil {
				a.logger.Error("failed to cancel pending run job", "job_id", job.ID, "error", err)
				dispatchError = true
				continue
			}
			resp.PendingCancelled++
			_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
				JobID:       job.ID,
				EventType:   "requested",
				RequestedBy: actor,
				Message:     strPtr("run cancel requested for pending job"),
			})
			_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
				JobID:       job.ID,
				EventType:   "completed",
				RequestedBy: actor,
				Message:     strPtr("run cancel completed for pending job"),
			})
			continue
		}

		if (job.Status == domain.JobStatusRunning || job.Status == domain.JobStatusCancelling) &&
			a.cancelPub != nil && job.AssignedNodeID != nil && *job.AssignedNodeID != "" {
			message := "Job cancellation requested"
			if err := a.store.MarkJobCancelling(r.Context(), job.ID, actor, "run_cancel_requested", &message); err != nil {
				a.logger.Error("failed to mark run job cancelling", "job_id", job.ID, "error", err)
				dispatchError = true
				continue
			}
			_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
				JobID:       job.ID,
				EventType:   "requested",
				RequestedBy: actor,
				NodeID:      job.AssignedNodeID,
				ExecutorRef: job.ExecutorRef,
				Message:     strPtr("run cancellation requested"),
			})
			signal := control.CancelSignal{
				JobID:       job.ID,
				RunID:       job.RunID,
				RequestedBy: actor,
				RequestedAt: time.Now().UTC(),
			}
			if err := a.cancelPub.PublishJobCancel(r.Context(), *job.AssignedNodeID, signal); err != nil {
				a.logger.Error("failed to publish run cancel signal", "job_id", job.ID, "node_id", *job.AssignedNodeID, "error", err)
				dispatchError = true
				_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
					JobID:       job.ID,
					EventType:   "failed",
					RequestedBy: actor,
					NodeID:      job.AssignedNodeID,
					ExecutorRef: job.ExecutorRef,
					Message:     strPtr(fmt.Sprintf("failed to dispatch run cancellation: %v", err)),
				})
				continue
			}
			resp.RunningMarkedCancelling++
			_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
				JobID:       job.ID,
				EventType:   "dispatched",
				RequestedBy: actor,
				NodeID:      job.AssignedNodeID,
				ExecutorRef: job.ExecutorRef,
				Message:     strPtr("run cancellation dispatched to worker"),
			})
			continue
		}

		// Fallback behaviour when push control channel is unavailable.
		msg := "Job cancelled by user"
		if err := a.store.UpdateJobStatus(r.Context(), job.ID, domain.JobStatusCancelled, &msg); err != nil {
			a.logger.Error("failed to fallback-cancel job from run cancel", "job_id", job.ID, "error", err)
			dispatchError = true
			continue
		}
		resp.PendingCancelled++
		_ = a.store.CreateJobCancellationEvent(r.Context(), postgres.JobCancellationEvent{
			JobID:       job.ID,
			EventType:   "completed",
			RequestedBy: actor,
			NodeID:      job.AssignedNodeID,
			ExecutorRef: job.ExecutorRef,
			Message:     strPtr("run cancellation completed via fallback"),
		})
	}

	if err := a.store.RecomputeRunStatus(r.Context(), run.ID); err != nil {
		a.logger.Error("failed to recompute run status after run cancel", "run_id", run.ID, "error", err)
		dispatchError = true
	}

	if resp.TotalTargeted == 0 {
		writeJSON(w, http.StatusConflict, ErrorResponse{
			Error:   "cannot_cancel",
			Message: "All jobs are already terminal",
			Code:    http.StatusConflict,
		})
		return
	}

	if dispatchError {
		writeJSON(w, http.StatusAccepted, resp)
		return
	}
	if resp.RunningMarkedCancelling > 0 {
		writeJSON(w, http.StatusAccepted, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func filterRerunJobs(jobs []*domain.Job, mode string) []*domain.Job {
	if mode == "all" {
		return jobs
	}
	filtered := make([]*domain.Job, 0, len(jobs))
	for _, job := range jobs {
		switch job.Status {
		case domain.JobStatusFailed, domain.JobStatusTimeout, domain.JobStatusCancelled:
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func generateRerunSlug(sourceSlug string) string {
	// Include time and random suffix to avoid collisions.
	return fmt.Sprintf("%s-rerun-%d-%s", sourceSlug, time.Now().Unix(), uuid.NewString()[:8])
}

func cloneJobForRerun(source *domain.Job, runID uuid.UUID, sourceRunID uuid.UUID, actor string) *domain.Job {
	metadata := mergeAnyMap(source.Metadata, map[string]any{
		"rerun_of_job_id": source.ID.String(),
		"rerun_of_run_id": sourceRunID.String(),
	})

	return &domain.Job{
		ID:               uuid.New(),
		RunID:            runID,
		Name:             source.Name,
		CreatedBy:        actor,
		Status:           domain.JobStatusPending,
		Image:            source.Image,
		Command:          append([]string(nil), source.Command...),
		Env:              copyStringMap(source.Env),
		CPULimit:         copyStringPtr(source.CPULimit),
		MemoryLimit:      copyStringPtr(source.MemoryLimit),
		GPUCount:         source.GPUCount,
		TimeoutSecs:      source.TimeoutSecs,
		Outputs:          append([]string(nil), source.Outputs...),
		Executor:         source.Executor,
		RegistrySecretID: source.RegistrySecretID,
		Metadata:         metadata,
	}
}

func mergeAnyMap(base map[string]any, overrides map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overrides))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
