package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

func (a *API) HandleCreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.serviceAccountWorkspaceForOwner(w, r)
	if !ok {
		return
	}

	var req CreateServiceAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "name is required", Code: http.StatusBadRequest})
		return
	}
	if !futureExpiry(req.ExpiresAt) {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "expires_at must be in the future", Code: http.StatusBadRequest})
		return
	}

	projectIDs, projectSlugs, ok := a.resolveProjectGrantSlugs(w, r, workspace.ID, req.ProjectSlugs)
	if !ok {
		return
	}

	accountID := uuid.New()
	displayName := name
	provider := "service_account"
	principal := &domain.Principal{
		ID:          uuid.New(),
		Subject:     "service_account:" + accountID.String(),
		DisplayName: &displayName,
		Provider:    &provider,
	}
	account := &domain.ServiceAccount{
		ID:          accountID,
		WorkspaceID: workspace.ID,
		Name:        name,
		Description: trimStringPtr(req.Description),
		CreatedBy:   ActorFromRequest(r),
	}
	if err := a.store.CreateServiceAccount(r.Context(), account, principal, projectIDs, domain.MemberRoleMember, domain.MemberRoleMember); err != nil {
		a.logger.Error("failed to create service account", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to create service account", Code: http.StatusInternalServerError})
		return
	}
	account.Principal = principal

	rawKey, key := newServiceAccountKey(account.ID, nil, req.ExpiresAt, ActorFromRequest(r))
	if err := a.store.CreateServiceAccountKey(r.Context(), key); err != nil {
		a.logger.Error("failed to create service account key", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to create service account key", Code: http.StatusInternalServerError})
		return
	}

	resp := toServiceAccountResponse(account, projectSlugs, []domain.ServiceAccountKey{*key})
	writeJSON(w, http.StatusCreated, CreateServiceAccountResponse{ServiceAccount: resp, Key: rawKey})
}

func (a *API) HandleListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.serviceAccountWorkspaceForOwner(w, r)
	if !ok {
		return
	}

	accounts, err := a.store.ListServiceAccounts(r.Context(), workspace.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list service accounts", Code: http.StatusInternalServerError})
		return
	}

	responses := make([]ServiceAccountResponse, 0, len(accounts))
	for i := range accounts {
		keys, err := a.store.ListServiceAccountKeys(r.Context(), accounts[i].ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list service account keys", Code: http.StatusInternalServerError})
			return
		}
		projectSlugs, err := a.projectSlugsForPrincipal(r.Context(), workspace.ID, accounts[i].PrincipalID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list service account projects", Code: http.StatusInternalServerError})
			return
		}
		responses = append(responses, toServiceAccountResponse(&accounts[i], projectSlugs, keys))
	}
	writeJSON(w, http.StatusOK, map[string]any{"service_accounts": responses})
}

func (a *API) HandleCreateServiceAccountKey(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.serviceAccountWorkspaceForOwner(w, r)
	if !ok {
		return
	}
	accountID, err := uuid.Parse(r.PathValue("service_account_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid service account id", Code: http.StatusBadRequest})
		return
	}
	account, err := a.store.GetServiceAccount(r.Context(), workspace.ID, accountID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Service account not found", Code: http.StatusNotFound})
		return
	}
	if account.DisabledAt != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "service account is disabled", Code: http.StatusBadRequest})
		return
	}

	var req CreateServiceAccountKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}
	if !futureExpiry(req.ExpiresAt) {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "expires_at must be in the future", Code: http.StatusBadRequest})
		return
	}

	rawKey, key := newServiceAccountKey(accountID, trimStringPtr(req.Name), req.ExpiresAt, ActorFromRequest(r))
	if err := a.store.CreateServiceAccountKey(r.Context(), key); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to create service account key", Code: http.StatusInternalServerError})
		return
	}
	writeJSON(w, http.StatusCreated, CreateServiceAccountKeyResponse{KeyID: key.ID, Key: rawKey, TokenPrefix: key.TokenPrefix, ExpiresAt: key.ExpiresAt})
}

func (a *API) HandleRevokeServiceAccountKey(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.serviceAccountWorkspaceForOwner(w, r)
	if !ok {
		return
	}
	accountID, err := uuid.Parse(r.PathValue("service_account_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid service account id", Code: http.StatusBadRequest})
		return
	}
	keyID, err := uuid.Parse(r.PathValue("key_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid key id", Code: http.StatusBadRequest})
		return
	}
	if err := a.store.RevokeServiceAccountKey(r.Context(), workspace.ID, accountID, keyID, ActorFromRequest(r)); err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Service account key not found", Code: http.StatusNotFound})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) HandleDisableServiceAccount(w http.ResponseWriter, r *http.Request) {
	workspace, ok := a.serviceAccountWorkspaceForOwner(w, r)
	if !ok {
		return
	}
	accountID, err := uuid.Parse(r.PathValue("service_account_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid service account id", Code: http.StatusBadRequest})
		return
	}
	if err := a.store.DisableServiceAccount(r.Context(), workspace.ID, accountID, ActorFromRequest(r)); err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Service account not found", Code: http.StatusNotFound})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) serviceAccountWorkspaceForOwner(w http.ResponseWriter, r *http.Request) (*domain.Workspace, bool) {
	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), r.PathValue("workspace_slug"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return nil, false
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return nil, false
	}
	return workspace, true
}

func (a *API) resolveProjectGrantSlugs(w http.ResponseWriter, r *http.Request, workspaceID uuid.UUID, slugs []string) ([]uuid.UUID, []string, bool) {
	projectIDs := make([]uuid.UUID, 0, len(slugs))
	projectSlugs := make([]string, 0, len(slugs))
	seen := map[string]struct{}{}
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		project, err := a.store.GetProjectBySlug(r.Context(), workspaceID, slug)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "unknown project slug: " + slug, Code: http.StatusBadRequest})
			return nil, nil, false
		}
		seen[slug] = struct{}{}
		projectIDs = append(projectIDs, project.ID)
		projectSlugs = append(projectSlugs, project.Slug)
	}
	return projectIDs, projectSlugs, true
}

func (a *API) projectSlugsForPrincipal(ctx context.Context, workspaceID, principalID uuid.UUID) ([]string, error) {
	memberships, err := a.store.ListProjectMembershipsForPrincipal(ctx, principalID)
	if err != nil {
		return nil, err
	}
	var slugs []string
	for _, m := range memberships {
		if m.Project != nil && m.Project.WorkspaceID == workspaceID {
			slugs = append(slugs, m.Project.Slug)
		}
	}
	return slugs, nil
}

func newServiceAccountKey(accountID uuid.UUID, name *string, expiresAt time.Time, actor string) (string, *domain.ServiceAccountKey) {
	keyID := uuid.New()
	raw := generateServiceAccountToken(keyID)
	return raw, &domain.ServiceAccountKey{
		ID:               keyID,
		ServiceAccountID: accountID,
		Name:             name,
		TokenHash:        hashServiceAccountToken(raw),
		TokenPrefix:      serviceAccountTokenPrefix(raw),
		ExpiresAt:        expiresAt.UTC(),
		CreatedBy:        actor,
	}
}

func toServiceAccountResponse(account *domain.ServiceAccount, projectSlugs []string, keys []domain.ServiceAccountKey) ServiceAccountResponse {
	resp := ServiceAccountResponse{
		ID:           account.ID,
		WorkspaceID:  account.WorkspaceID,
		PrincipalID:  account.PrincipalID,
		Name:         account.Name,
		Description:  account.Description,
		DisabledAt:   account.DisabledAt,
		CreatedAt:    account.CreatedAt,
		UpdatedAt:    account.UpdatedAt,
		CreatedBy:    account.CreatedBy,
		ProjectSlugs: projectSlugs,
	}
	if account.Principal != nil {
		resp.Subject = account.Principal.Subject
	}
	for _, key := range keys {
		resp.Keys = append(resp.Keys, ServiceAccountKeyResponse{
			ID:          key.ID,
			Name:        key.Name,
			TokenPrefix: key.TokenPrefix,
			ExpiresAt:   key.ExpiresAt,
			LastUsedAt:  key.LastUsedAt,
			RevokedAt:   key.RevokedAt,
			CreatedAt:   key.CreatedAt,
			CreatedBy:   key.CreatedBy,
		})
	}
	return resp
}

func futureExpiry(expiresAt time.Time) bool {
	return !expiresAt.IsZero() && expiresAt.After(time.Now().UTC())
}

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
