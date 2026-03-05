package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heldtogether/switchyard/internal/config"
	"github.com/stretchr/testify/require"
)

func setupWorkspaceRouter(api *API) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces", api.HandleCreateWorkspace)
	return mux
}

func TestHandleCreateWorkspace_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	api := &API{
		cfg:    &config.Config{},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := setupWorkspaceRouter(api)

	body := bytes.NewBufferString(`{"slug":"acme","name":"Acme Workspace"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var response WorkspaceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "acme", response.Slug)
	require.Equal(t, "Acme Workspace", response.Name)
}

func TestHandleCreateWorkspace_InvalidSlug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	api := &API{
		cfg:    &config.Config{},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := setupWorkspaceRouter(api)

	body := bytes.NewBufferString(`{"slug":"Invalid Slug","name":"Acme Workspace"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateWorkspace_DuplicateSlug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	api := &API{
		cfg:    &config.Config{},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := setupWorkspaceRouter(api)

	body := bytes.NewBufferString(`{"slug":"acme","name":"Acme Workspace"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	reqDup := httptest.NewRequest(http.MethodPost, "/v1/workspaces", bytes.NewBufferString(`{"slug":"acme","name":"Acme Duplicate"}`))
	wDup := httptest.NewRecorder()
	mux.ServeHTTP(wDup, reqDup)

	require.Equal(t, http.StatusConflict, wDup.Code)
}
