package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// HandleCreateRegistrySecret handles POST /v1/workspaces/{workspace_slug}/registry-secrets
func (a *API) HandleCreateRegistrySecret(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")

	var req CreateRegistrySecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.Host == "" || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "host, username, and password are required",
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

	secret := &domain.RegistrySecret{
		ID:                uuid.New(),
		CreatedBy:         "api-key-user",
		WorkspaceID:       workspace.ID,
		Host:              req.Host,
		Username:          req.Username,
		PasswordEncrypted: req.Password,
		Active:            true,
	}

	if err := a.store.CreateRegistrySecret(r.Context(), secret); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create registry secret",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusCreated, RegistrySecretResponse{
		ID:        secret.ID,
		CreatedAt: secret.CreatedAt,
		CreatedBy: secret.CreatedBy,
		Host:      secret.Host,
		Username:  secret.Username,
		Active:    secret.Active,
	})
}

// HandleListRegistrySecrets handles GET /v1/workspaces/{workspace_slug}/registry-secrets
func (a *API) HandleListRegistrySecrets(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	secrets, err := a.store.ListRegistrySecrets(r.Context(), workspace.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list registry secrets",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	resp := make([]RegistrySecretResponse, 0, len(secrets))
	for _, secret := range secrets {
		resp = append(resp, RegistrySecretResponse{
			ID:        secret.ID,
			CreatedAt: secret.CreatedAt,
			CreatedBy: secret.CreatedBy,
			Host:      secret.Host,
			Username:  secret.Username,
			Active:    secret.Active,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registry_secrets": resp,
	})
}
