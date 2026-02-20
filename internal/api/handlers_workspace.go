package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// HandleCreateWorkspace handles POST /v1/workspaces
func (a *API) HandleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkspaceRequest
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

	// Create workspace
	workspace := &domain.Workspace{
		ID:          uuid.New(),
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
	}

	if err := a.store.CreateWorkspace(r.Context(), workspace); err != nil {
		a.logger.Error("failed to create workspace", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create workspace",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	a.logger.Info("workspace created", "workspace_id", workspace.ID, "slug", workspace.Slug)

	writeJSON(w, http.StatusCreated, toWorkspaceResponse(workspace))
}

// HandleGetWorkspace handles GET /v1/workspaces/{slug}
func (a *API) HandleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), slug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	writeJSON(w, http.StatusOK, toWorkspaceResponse(workspace))
}

// HandleListWorkspaces handles GET /v1/workspaces
func (a *API) HandleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	workspaces, err := a.store.ListWorkspaces(r.Context(), limit, offset)
	if err != nil {
		a.logger.Error("failed to list workspaces", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list workspaces",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	responses := make([]WorkspaceResponse, len(workspaces))
	for i, ws := range workspaces {
		responses[i] = toWorkspaceResponse(ws)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"workspaces": responses,
		"limit":      limit,
		"offset":     offset,
	})
}
