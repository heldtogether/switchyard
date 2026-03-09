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

func setupProjectRouter(api *API) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects", api.HandleCreateProject)
	return mux
}

func TestHandleCreateProject_ReservedSlug(t *testing.T) {
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
	mux := setupProjectRouter(api)

	body := bytes.NewBufferString(`{"slug":"runs","name":"Reserved Slug Project"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "validation_error", response.Error)
	require.Equal(t, "slug is reserved for system routes", response.Message)
}
