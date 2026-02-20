package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
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

	// Create run
	run := &domain.Run{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Status:      domain.RunStatusPending,
		CreatedBy:   "api-key-user", // TODO: Get from auth context
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
