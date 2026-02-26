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
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestCreateJob_GPUValidation(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	workspace, err := store.GetWorkspaceBySlug(ctx, "default")
	require.NoError(t, err)
	project := &domain.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: "test-project", Name: "Test Project", CreatedBy: "test-user"}
	require.NoError(t, store.CreateProject(ctx, project))
	run := &domain.Run{ID: uuid.New(), ProjectID: project.ID, Slug: "test-run", Name: "Test Run", Status: domain.RunStatusPending, CreatedBy: "test-user"}
	require.NoError(t, store.CreateRun(ctx, run))

	cfg := &config.Config{
		API:      config.APIConfig{BaseURL: "http://localhost:8080"},
		Executor: config.ExecutorConfig{Swarm: config.SwarmConfig{Defaults: config.SwarmDefaultsConfig{Resources: config.ResourcesConfig{CPU: "1.0", Memory: "1g"}}}},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	api := &API{cfg: cfg, store: store, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs", api.HandleCreateJob)

	// gpu < 0 -> 400
	payload := map[string]any{
		"image":   "alpine:latest",
		"outputs": []string{"/outputs"},
		"resources": map[string]any{
			"gpu": -1,
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects/test-project/runs/test-run/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// gpu > max (no nodes) -> 422
	payload["resources"] = map[string]any{"gpu": 1}
	body, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects/test-project/runs/test-run/jobs", bytes.NewReader(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
