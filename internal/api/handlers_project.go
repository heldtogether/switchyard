package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// HandleCreateProject handles POST /v1/workspaces/{workspace_slug}/projects
func (a *API) HandleCreateProject(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	var req CreateProjectRequest
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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return
	}

	// Create project
	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   ActorFromRequest(r),
		Archived:    false,
		Metadata:    req.Metadata,
	}

	if err := a.store.CreateProject(r.Context(), project); err != nil {
		a.logger.Error("failed to create project", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create project",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	if a.rbacEnabled() {
		if principalID, ok, err := a.currentPrincipalID(r); err == nil && ok {
			_ = a.store.CreateProjectMembership(r.Context(), &domain.ProjectMembership{
				ProjectID:   project.ID,
				PrincipalID: principalID,
				Role:        domain.MemberRoleOwner,
				CreatedBy:   ActorFromRequest(r),
			})
		}
	}

	a.logger.Info("project created", "project_id", project.ID, "slug", project.Slug)

	writeJSON(w, http.StatusCreated, toProjectResponse(project))
}

// HandleGetProject handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}
func (a *API) HandleGetProject(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")

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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
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

	writeJSON(w, http.StatusOK, toProjectResponse(project))
}

// HandleListProjects handles GET /v1/workspaces/{workspace_slug}/projects
func (a *API) HandleListProjects(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	includeArchived := r.URL.Query().Get("include_archived") == "true"

	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
		return
	}

	projects, err := a.store.ListProjects(r.Context(), workspace.ID, includeArchived, limit, offset)
	if err != nil {
		a.logger.Error("failed to list projects", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list projects",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	responses := make([]ProjectResponse, 0, len(projects))
	for _, p := range projects {
		if a.rbacEnabled() {
			_, ok, err := a.authorizeProject(r, workspace, p)
			if err != nil {
				a.logger.Error("failed to authorize project access", "error", err, "project_id", p.ID)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{
					Error:   "internal_error",
					Message: "Failed to authorize project access",
					Code:    http.StatusInternalServerError,
				})
				return
			}
			if !ok {
				continue
			}
		}
		responses = append(responses, toProjectResponse(p))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"projects": responses,
		"limit":    limit,
		"offset":   offset,
	})
}

// HandleUpdateProject handles PUT /v1/workspaces/{workspace_slug}/projects/{project_slug}
func (a *API) HandleUpdateProject(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")

	var req UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
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

	// Update fields if provided
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = req.Description
	}
	if req.Metadata != nil {
		project.Metadata = req.Metadata
	}

	if err := a.store.UpdateProject(r.Context(), project); err != nil {
		a.logger.Error("failed to update project", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to update project",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, toProjectResponse(project))
}

// HandleArchiveProject handles POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/archive
func (a *API) HandleArchiveProject(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")

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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
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

	if err := a.store.ArchiveProject(r.Context(), project.ID); err != nil {
		a.logger.Error("failed to archive project", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to archive project",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Project archived successfully",
	})
}
