package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/lib/pq"
)

func normalizeSecretInputs(host, username string) (string, string) {
	normalizedHost := strings.TrimSpace(strings.ToLower(host))
	normalizedUsername := strings.TrimSpace(username)
	return normalizedHost, normalizedUsername
}

func toRegistrySecretResponse(secret domain.RegistrySecret) RegistrySecretResponse {
	return RegistrySecretResponse{
		ID:                  secret.ID,
		CreatedAt:           secret.CreatedAt,
		CreatedBy:           secret.CreatedBy,
		Host:                secret.Host,
		Username:            secret.Username,
		Active:              secret.Active,
		DeactivatedAt:       secret.DeactivatedAt,
		DeactivatedBy:       secret.DeactivatedBy,
		RotatedFromSecretID: secret.RotatedFromID,
	}
}

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

	host, username := normalizeSecretInputs(req.Host, req.Username)
	password := strings.TrimSpace(req.Password)
	if host == "" || username == "" || password == "" {
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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return
	}

	secret := &domain.RegistrySecret{
		ID:                uuid.New(),
		CreatedBy:         ActorFromRequest(r),
		WorkspaceID:       workspace.ID,
		Host:              host,
		Username:          username,
		PasswordEncrypted: password,
		Active:            true,
	}

	if err := a.store.CreateRegistrySecret(r.Context(), secret); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error:   "conflict",
				Message: "an active secret already exists for this host and username",
				Code:    http.StatusConflict,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create registry secret",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusCreated, toRegistrySecretResponse(*secret))
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
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
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
		resp = append(resp, toRegistrySecretResponse(secret))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registry_secrets": resp,
	})
}

// HandleDeleteRegistrySecret handles DELETE /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id}
func (a *API) HandleDeleteRegistrySecret(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	secretID, err := uuid.Parse(r.PathValue("secret_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid secret id", Code: http.StatusBadRequest})
		return
	}

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return
	}

	if err := a.store.DeactivateRegistrySecret(r.Context(), workspace.ID, secretID, ActorFromRequest(r)); err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Registry secret not found", Code: http.StatusNotFound})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Registry secret deactivated"})
}

// HandleRotateRegistrySecret handles POST /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id}/rotate
func (a *API) HandleRotateRegistrySecret(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	secretID, err := uuid.Parse(r.PathValue("secret_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid secret id", Code: http.StatusBadRequest})
		return
	}

	var req RotateRegistrySecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "password is required", Code: http.StatusBadRequest})
		return
	}

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, true); !ok {
		return
	}

	rotated, err := a.store.RotateRegistrySecret(r.Context(), workspace.ID, secretID, password, ActorFromRequest(r))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error:   "conflict",
				Message: "an active secret already exists for this host and username",
				Code:    http.StatusConflict,
			})
			return
		}
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Registry secret not found", Code: http.StatusNotFound})
		return
	}

	writeJSON(w, http.StatusOK, toRegistrySecretResponse(*rotated))
}
