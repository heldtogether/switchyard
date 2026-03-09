package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/lib/pq"
)

var workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

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
	req.Slug = strings.TrimSpace(strings.ToLower(req.Slug))
	if !workspaceSlugPattern.MatchString(req.Slug) {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "slug must be lowercase letters, numbers, and hyphens only",
			Code:    http.StatusBadRequest,
		})
		return
	}
	if isReservedSlug(req.Slug) {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "slug is reserved for system routes",
			Code:    http.StatusBadRequest,
		})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
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
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error:   "conflict",
				Message: "workspace slug already exists",
				Code:    http.StatusConflict,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create workspace",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	if a.rbacEnabled() {
		if principalID, ok, err := a.currentPrincipalID(r); err == nil && ok {
			_ = a.store.CreateWorkspaceMembership(r.Context(), &domain.WorkspaceMembership{
				WorkspaceID: workspace.ID,
				PrincipalID: principalID,
				Role:        domain.MemberRoleOwner,
				CreatedBy:   ActorFromRequest(r),
			})
		}
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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
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

	if a.rbacEnabled() {
		identity, err := a.resolveIdentity(r)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to resolve principal",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		if identity == nil {
			writeJSON(w, http.StatusForbidden, ErrorResponse{
				Error:   "forbidden",
				Message: "Forbidden",
				Code:    http.StatusForbidden,
			})
			return
		}

		var responses []WorkspaceResponse
		if identity.isService {
			workspaces, err := a.store.ListWorkspaces(r.Context(), 1000, 0)
			if err != nil {
				a.logger.Error("failed to list workspaces", "error", err)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{
					Error:   "internal_error",
					Message: "Failed to list workspaces",
					Code:    http.StatusInternalServerError,
				})
				return
			}
			allow := map[string]struct{}{}
			if identity.serviceAccount != nil {
				for _, slug := range identity.serviceAccount.AllowedWorkspaces {
					allow[slug] = struct{}{}
				}
			}
			for _, ws := range workspaces {
				if _, ok := allow[ws.Slug]; ok {
					responses = append(responses, toWorkspaceResponse(ws))
				}
			}
		} else {
			memberships, err := a.store.ListWorkspaceMembershipsForPrincipal(r.Context(), identity.principal.ID)
			if err != nil {
				a.logger.Error("failed to list workspace memberships", "error", err)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{
					Error:   "internal_error",
					Message: "Failed to list workspaces",
					Code:    http.StatusInternalServerError,
				})
				return
			}
			for _, m := range memberships {
				if m.Workspace != nil {
					responses = append(responses, toWorkspaceResponse(m.Workspace))
				}
			}
		}

		if offset > len(responses) {
			offset = len(responses)
		}
		end := offset + limit
		if end > len(responses) {
			end = len(responses)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workspaces": responses[offset:end],
			"limit":      limit,
			"offset":     offset,
		})
		return
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
