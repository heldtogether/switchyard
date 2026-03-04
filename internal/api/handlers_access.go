package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/heldtogether/switchyard/internal/domain"
)

const inviteTTL = 7 * 24 * time.Hour

func (a *API) HandleCreateWorkspaceInvite(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return
	}

	var req CreateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "email is required", Code: http.StatusBadRequest})
		return
	}
	role := domain.MemberRoleMember
	if req.Role == string(domain.MemberRoleOwner) {
		role = domain.MemberRoleOwner
	}

	token := generateInviteToken(32)
	invite := &domain.WorkspaceInvite{
		WorkspaceID: workspace.ID,
		Email:       email,
		Role:        role,
		TokenHash:   hashInviteToken(token),
		ExpiresAt:   time.Now().Add(inviteTTL),
		CreatedBy:   ActorFromRequest(r),
	}
	if err := a.store.CreateWorkspaceInvite(r.Context(), invite); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to create invite", Code: http.StatusInternalServerError})
		return
	}

	writeJSON(w, http.StatusCreated, CreateInviteResponse{
		InviteID:    invite.ID,
		InviteURL:   a.baseURL + "/accept-invite?token=" + token,
		InviteToken: token,
		ExpiresAt:   invite.ExpiresAt,
	})
}

func (a *API) HandleAcceptWorkspaceInvite(w http.ResponseWriter, r *http.Request) {
	var req AcceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "token is required", Code: http.StatusBadRequest})
		return
	}
	identity, err := a.resolveIdentity(r)
	if err != nil || identity == nil || identity.principal == nil {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Only authenticated users may accept invites", Code: http.StatusForbidden})
		return
	}
	email := ""
	if identity.principal.Email != nil {
		email = *identity.principal.Email
	}
	if _, err := a.store.AcceptWorkspaceInvite(r.Context(), hashInviteToken(token), identity.principal.ID, email, ActorFromRequest(r)); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: err.Error(), Code: http.StatusBadRequest})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Invite accepted"})
}

func (a *API) HandleListWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return
	}
	members, err := a.store.ListWorkspaceMembers(r.Context(), workspace.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list members", Code: http.StatusInternalServerError})
		return
	}
	resp := make([]MemberResponse, 0, len(members))
	for _, m := range members {
		if m.Principal == nil {
			continue
		}
		resp = append(resp, MemberResponse{
			Subject:     m.Principal.Subject,
			Email:       m.Principal.Email,
			DisplayName: m.Principal.DisplayName,
			Role:        string(m.Role),
			AddedAt:     m.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": resp})
}

func (a *API) HandleCreateProjectInvite(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return
	}
	project, err := a.store.GetProjectBySlug(r.Context(), workspace.ID, projectSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Project not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireProjectAccess(w, r, workspace, project); !ok {
		return
	}

	var req CreateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "email is required", Code: http.StatusBadRequest})
		return
	}
	role := domain.MemberRoleMember
	if req.Role == string(domain.MemberRoleOwner) {
		role = domain.MemberRoleOwner
	}

	token := generateInviteToken(32)
	invite := &domain.ProjectInvite{
		ProjectID: project.ID,
		Email:     email,
		Role:      role,
		TokenHash: hashInviteToken(token),
		ExpiresAt: time.Now().Add(inviteTTL),
		CreatedBy: ActorFromRequest(r),
	}
	if err := a.store.CreateProjectInvite(r.Context(), invite); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to create invite", Code: http.StatusInternalServerError})
		return
	}

	writeJSON(w, http.StatusCreated, CreateInviteResponse{
		InviteID:    invite.ID,
		InviteURL:   a.baseURL + "/accept-invite?token=" + token,
		InviteToken: token,
		ExpiresAt:   invite.ExpiresAt,
	})
}

func (a *API) HandleAcceptProjectInvite(w http.ResponseWriter, r *http.Request) {
	var req AcceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "token is required", Code: http.StatusBadRequest})
		return
	}
	identity, err := a.resolveIdentity(r)
	if err != nil || identity == nil || identity.principal == nil {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Only authenticated users may accept invites", Code: http.StatusForbidden})
		return
	}
	email := ""
	if identity.principal.Email != nil {
		email = *identity.principal.Email
	}
	if _, err := a.store.AcceptProjectInvite(r.Context(), hashInviteToken(token), identity.principal.ID, email, ActorFromRequest(r)); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: err.Error(), Code: http.StatusBadRequest})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Invite accepted"})
}

func (a *API) HandleListProjectMembers(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
		return
	}
	project, err := a.store.GetProjectBySlug(r.Context(), workspace.ID, projectSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Project not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireProjectAccess(w, r, workspace, project); !ok {
		return
	}

	members, err := a.store.ListProjectMembers(r.Context(), project.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list members", Code: http.StatusInternalServerError})
		return
	}
	resp := make([]MemberResponse, 0, len(members))
	for _, m := range members {
		if m.Principal == nil {
			continue
		}
		resp = append(resp, MemberResponse{
			Subject:     m.Principal.Subject,
			Email:       m.Principal.Email,
			DisplayName: m.Principal.DisplayName,
			Role:        string(m.Role),
			AddedAt:     m.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": resp})
}

func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateInviteToken(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
