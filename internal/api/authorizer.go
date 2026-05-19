package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
)

type resolvedIdentity struct {
	principal      *domain.Principal
	isService      bool
	serviceAccount *config.RBACServiceAccountSpec
}

func (a *API) rbacEnabled() bool {
	return a.cfg != nil && a.cfg.API.RBAC.Enabled
}

func (a *API) serviceAccountSpec() *config.RBACServiceAccountSpec {
	if len(a.cfg.API.RBAC.ServiceAccounts) == 0 {
		return nil
	}
	return &a.cfg.API.RBAC.ServiceAccounts[0]
}

func (a *API) resolveIdentity(r *http.Request) (*resolvedIdentity, error) {
	p, ok := PrincipalFromContext(r.Context())
	if !ok {
		return nil, nil
	}

	if p.AuthMethod == "api_key" {
		spec := a.serviceAccountSpec()
		return &resolvedIdentity{
			isService:      true,
			serviceAccount: spec,
		}, nil
	}

	if p.AuthMethod == "service_account" {
		dp, err := a.store.GetPrincipalBySubject(r.Context(), p.Subject)
		if err != nil {
			return nil, err
		}
		return &resolvedIdentity{principal: dp}, nil
	}

	dp := &domain.Principal{
		Subject:     p.Subject,
		Email:       ptrOrNil(strings.TrimSpace(p.Email)),
		DisplayName: ptrOrNil(strings.TrimSpace(p.Name)),
		Provider:    ptrOrNil(strings.TrimSpace(p.Provider)),
	}
	if err := a.store.UpsertPrincipal(r.Context(), dp); err != nil {
		return nil, err
	}
	return &resolvedIdentity{principal: dp}, nil
}

func ptrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (a *API) ensureSingleTenantOwnerIfNeeded(r *http.Request, identity *resolvedIdentity, workspace *domain.Workspace) error {
	if !a.cfg.API.RBAC.SingleTenant || identity == nil || identity.principal == nil {
		return nil
	}
	if workspace.Slug != a.cfg.API.RBAC.DefaultWorkspaceSlug {
		return nil
	}
	owners, err := a.store.CountWorkspaceOwners(r.Context(), workspace.ID)
	if err != nil {
		return err
	}
	if owners > 0 {
		return nil
	}
	m := &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: identity.principal.ID,
		Role:        domain.MemberRoleOwner,
		CreatedBy:   ActorFromRequest(r),
	}
	return a.store.CreateWorkspaceMembership(r.Context(), m)
}

func (a *API) authorizeWorkspace(r *http.Request, workspace *domain.Workspace, ownerRequired bool) (domain.MemberRole, bool, error) {
	if !a.rbacEnabled() {
		return domain.MemberRoleOwner, true, nil
	}

	identity, err := a.resolveIdentity(r)
	if err != nil {
		return "", false, err
	}
	if identity == nil {
		return "", false, nil
	}

	if identity.isService {
		if identity.serviceAccount == nil {
			return "", false, nil
		}
		if !slices.Contains(identity.serviceAccount.AllowedWorkspaces, workspace.Slug) {
			return "", false, nil
		}
		// service account is treated as owner within its allowlist.
		return domain.MemberRoleOwner, true, nil
	}

	if err := a.ensureSingleTenantOwnerIfNeeded(r, identity, workspace); err != nil {
		return "", false, err
	}

	role, err := a.store.WorkspaceRoleForPrincipal(r.Context(), workspace.ID, identity.principal.ID)
	if err != nil {
		return "", false, err
	}
	if role == nil {
		return "", false, nil
	}
	if ownerRequired && *role != domain.MemberRoleOwner {
		return *role, false, nil
	}
	return *role, true, nil
}

func (a *API) authorizeProject(r *http.Request, workspace *domain.Workspace, project *domain.Project) (domain.MemberRole, bool, error) {
	if !a.rbacEnabled() {
		return domain.MemberRoleOwner, true, nil
	}

	identity, err := a.resolveIdentity(r)
	if err != nil {
		return "", false, err
	}
	if identity == nil {
		return "", false, nil
	}

	if identity.isService {
		if identity.serviceAccount == nil {
			return "", false, nil
		}
		if !slices.Contains(identity.serviceAccount.AllowedWorkspaces, workspace.Slug) {
			return "", false, nil
		}
		allowedProjects := identity.serviceAccount.AllowedProjects[workspace.Slug]
		if len(allowedProjects) > 0 && !slices.Contains(allowedProjects, project.Slug) {
			return "", false, nil
		}
		return domain.MemberRoleOwner, true, nil
	}

	workspaceRole, workspaceOK, err := a.authorizeWorkspace(r, workspace, false)
	if err != nil {
		return "", false, err
	}
	if workspaceOK && workspaceRole == domain.MemberRoleOwner {
		return workspaceRole, true, nil
	}

	role, err := a.store.ProjectRoleForPrincipal(r.Context(), project.ID, identity.principal.ID)
	if err != nil {
		return "", false, err
	}
	if role != nil {
		return *role, true, nil
	}
	if workspaceOK {
		return workspaceRole, true, nil
	}
	return "", false, nil
}

func (a *API) authorizeWorkspaceOrProjectAccess(r *http.Request, workspace *domain.Workspace) (bool, error) {
	if !a.rbacEnabled() {
		return true, nil
	}

	identity, err := a.resolveIdentity(r)
	if err != nil {
		return false, err
	}
	if identity == nil {
		return false, nil
	}

	if identity.isService {
		return identity.serviceAccount != nil && slices.Contains(identity.serviceAccount.AllowedWorkspaces, workspace.Slug), nil
	}

	if _, ok, err := a.authorizeWorkspace(r, workspace, false); err != nil || ok {
		return ok, err
	}

	memberships, err := a.store.ListProjectMembershipsForPrincipal(r.Context(), identity.principal.ID)
	if err != nil {
		return false, err
	}
	for _, m := range memberships {
		if m.Project != nil && m.Project.WorkspaceID == workspace.ID {
			return true, nil
		}
	}
	return false, nil
}

func (a *API) requireWorkspaceAccess(w http.ResponseWriter, r *http.Request, workspace *domain.Workspace, ownerRequired bool) (domain.MemberRole, bool) {
	role, ok, err := a.authorizeWorkspace(r, workspace, ownerRequired)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to authorize workspace access",
			Code:    http.StatusInternalServerError,
		})
		return "", false
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Forbidden",
			Code:    http.StatusForbidden,
		})
		return "", false
	}
	return role, true
}

func (a *API) requireWorkspaceOrProjectAccess(w http.ResponseWriter, r *http.Request, workspace *domain.Workspace) bool {
	ok, err := a.authorizeWorkspaceOrProjectAccess(r, workspace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to authorize workspace access",
			Code:    http.StatusInternalServerError,
		})
		return false
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Forbidden",
			Code:    http.StatusForbidden,
		})
		return false
	}
	return true
}

func (a *API) requireProjectAccess(w http.ResponseWriter, r *http.Request, workspace *domain.Workspace, project *domain.Project) (domain.MemberRole, bool) {
	role, ok, err := a.authorizeProject(r, workspace, project)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to authorize project access",
			Code:    http.StatusInternalServerError,
		})
		return "", false
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Forbidden",
			Code:    http.StatusForbidden,
		})
		return "", false
	}
	return role, true
}

func (a *API) requireProjectOwner(w http.ResponseWriter, r *http.Request, workspace *domain.Workspace, project *domain.Project) bool {
	role, ok := a.requireProjectAccess(w, r, workspace, project)
	if !ok {
		return false
	}
	if role != domain.MemberRoleOwner {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Forbidden",
			Code:    http.StatusForbidden,
		})
		return false
	}
	return true
}

func (a *API) currentPrincipalID(r *http.Request) (uuid.UUID, bool, error) {
	identity, err := a.resolveIdentity(r)
	if err != nil {
		return uuid.Nil, false, err
	}
	if identity == nil || identity.principal == nil {
		return uuid.Nil, false, nil
	}
	return identity.principal.ID, true, nil
}
