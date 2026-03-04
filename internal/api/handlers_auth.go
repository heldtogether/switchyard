package api

import "net/http"

func (a *API) HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Authentication is not configured",
			Code:    http.StatusNotFound,
		})
		return
	}
	a.auth.StartLogin(w, r)
}

func (a *API) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Authentication is not configured",
			Code:    http.StatusNotFound,
		})
		return
	}
	a.auth.CompleteLogin(w, r)
}

func (a *API) HandleAuthLogoutRedirect(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a.auth.LogoutRedirect(w, r)
}

func (a *API) HandleAuthMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Authentication required",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	response := map[string]any{
		"user": principal,
	}
	if a.rbacEnabled() {
		identity, err := a.resolveIdentity(r)
		if err == nil && identity != nil && identity.principal != nil {
			workspaceMemberships, _ := a.store.ListWorkspaceMembershipsForPrincipal(r.Context(), identity.principal.ID)
			projectMemberships, _ := a.store.ListProjectMembershipsForPrincipal(r.Context(), identity.principal.ID)
			workspaces := make([]map[string]any, 0, len(workspaceMemberships))
			for _, m := range workspaceMemberships {
				if m.Workspace == nil {
					continue
				}
				workspaces = append(workspaces, map[string]any{
					"slug": m.Workspace.Slug,
					"role": m.Role,
				})
			}
			projects := make([]map[string]any, 0, len(projectMemberships))
			for _, m := range projectMemberships {
				if m.Project == nil {
					continue
				}
				workspace, err := a.store.GetWorkspace(r.Context(), m.Project.WorkspaceID)
				if err != nil {
					continue
				}
				projects = append(projects, map[string]any{
					"workspace_slug": workspace.Slug,
					"project_slug":   m.Project.Slug,
					"role":           m.Role,
				})
			}
			response["memberships"] = map[string]any{
				"workspaces": workspaces,
				"projects":   projects,
			}
		}
	}

	writeJSON(w, http.StatusOK, response)
}
