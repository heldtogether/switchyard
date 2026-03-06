package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/stretchr/testify/require"
)

func setupRegistrySecretRouter(api *API) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/registry-secrets", api.HandleCreateRegistrySecret)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/registry-secrets", api.HandleListRegistrySecrets)
	mux.HandleFunc("DELETE /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id}", api.HandleDeleteRegistrySecret)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id}/rotate", api.HandleRotateRegistrySecret)
	return mux
}

func TestHandleRegistrySecrets_CRUDLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	workspace := &domain.Workspace{
		ID:   uuid.New(),
		Slug: "registry-workspace",
		Name: "Registry Workspace",
	}
	require.NoError(t, store.CreateWorkspace(context.Background(), workspace))

	api := &API{
		cfg:    &config.Config{},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := setupRegistrySecretRouter(api)

	createReq := bytes.NewBufferString(`{"host":"Docker.IO","username":"robot","password":"secret-one"}`)
	createHTTPReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/registry-workspace/registry-secrets", createReq)
	createResp := httptest.NewRecorder()
	mux.ServeHTTP(createResp, createHTTPReq)
	require.Equal(t, http.StatusCreated, createResp.Code)

	var created RegistrySecretResponse
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &created))
	require.Equal(t, "docker.io", created.Host)
	require.Equal(t, "robot", created.Username)
	require.True(t, created.Active)

	rotateReq := bytes.NewBufferString(`{"password":"secret-two"}`)
	rotateHTTPReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/registry-workspace/registry-secrets/"+created.ID.String()+"/rotate", rotateReq)
	rotateResp := httptest.NewRecorder()
	mux.ServeHTTP(rotateResp, rotateHTTPReq)
	require.Equal(t, http.StatusOK, rotateResp.Code)

	var rotated RegistrySecretResponse
	require.NoError(t, json.Unmarshal(rotateResp.Body.Bytes(), &rotated))
	require.NotEqual(t, created.ID, rotated.ID)
	require.True(t, rotated.Active)
	require.NotNil(t, rotated.RotatedFromSecretID)
	require.Equal(t, created.ID, *rotated.RotatedFromSecretID)

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/registry-workspace/registry-secrets", nil)
	listResp := httptest.NewRecorder()
	mux.ServeHTTP(listResp, listReq)
	require.Equal(t, http.StatusOK, listResp.Code)

	var listed struct {
		RegistrySecrets []RegistrySecretResponse `json:"registry_secrets"`
	}
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listed))
	require.Len(t, listed.RegistrySecrets, 2)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/workspaces/registry-workspace/registry-secrets/"+rotated.ID.String(), nil)
	deleteResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteResp, deleteReq)
	require.Equal(t, http.StatusOK, deleteResp.Code)

	listAfterReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/registry-workspace/registry-secrets", nil)
	listAfterResp := httptest.NewRecorder()
	mux.ServeHTTP(listAfterResp, listAfterReq)
	require.Equal(t, http.StatusOK, listAfterResp.Code)

	var listedAfter struct {
		RegistrySecrets []RegistrySecretResponse `json:"registry_secrets"`
	}
	require.NoError(t, json.Unmarshal(listAfterResp.Body.Bytes(), &listedAfter))
	require.Len(t, listedAfter.RegistrySecrets, 2)
	inactiveCount := 0
	for _, s := range listedAfter.RegistrySecrets {
		if !s.Active {
			inactiveCount++
		}
	}
	require.Equal(t, 2, inactiveCount)
}
