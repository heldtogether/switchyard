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

type recordingQueue struct {
	published []string
}

func (q *recordingQueue) Publish(ctx context.Context, jobID string, gpuCount int) error {
	q.published = append(q.published, jobID)
	return nil
}

func (q *recordingQueue) Close() error { return nil }

func TestHandleRerunRun_AllJobs(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	workspace, err := store.GetWorkspaceBySlug(ctx, "default")
	require.NoError(t, err)

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "test-project",
		Name:        "Test Project",
		CreatedBy:   "test-user",
	}
	require.NoError(t, store.CreateProject(ctx, project))

	sourceRun := &domain.Run{
		ID:        uuid.New(),
		ProjectID: project.ID,
		Slug:      "source-run",
		Name:      "Source Run",
		Status:    domain.RunStatusFailed,
		CreatedBy: "test-user",
		Metadata: map[string]any{
			"trigger": "API",
		},
	}
	require.NoError(t, store.CreateRun(ctx, sourceRun))

	jobs := []*domain.Job{
		{
			ID:          uuid.New(),
			RunID:       sourceRun.ID,
			Name:        ptr("job-success"),
			CreatedBy:   "test-user",
			Status:      domain.JobStatusSucceeded,
			Image:       "alpine:latest",
			Command:     []string{"echo", "ok"},
			Env:         map[string]string{"A": "1"},
			Outputs:     []string{"/outputs"},
			TimeoutSecs: 60,
			Executor:    domain.ExecutorTypeDocker,
		},
		{
			ID:          uuid.New(),
			RunID:       sourceRun.ID,
			Name:        ptr("job-failed"),
			CreatedBy:   "test-user",
			Status:      domain.JobStatusFailed,
			Image:       "alpine:latest",
			Command:     []string{"echo", "fail"},
			Env:         map[string]string{"B": "2"},
			Outputs:     []string{"/outputs"},
			TimeoutSecs: 60,
			Executor:    domain.ExecutorTypeDocker,
		},
	}
	for _, job := range jobs {
		require.NoError(t, store.CreateJob(ctx, job))
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	queue := &recordingQueue{}
	api := &API{
		cfg: &config.Config{
			API: config.APIConfig{BaseURL: "http://localhost:8080"},
		},
		store:  store,
		logger: logger,
		queue:  queue,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/rerun", api.HandleRerunRun)

	body, _ := json.Marshal(map[string]any{"mode": "all"})
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects/test-project/runs/source-run/rerun", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp RerunRunResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, "all", resp.Mode)
	require.Equal(t, 2, resp.JobsCreated)
	require.Equal(t, sourceRun.ID, resp.SourceRunID)

	newRun, err := store.GetRunBySlug(ctx, project.ID, resp.Run.Slug)
	require.NoError(t, err)
	require.Equal(t, "Manual", newRun.Metadata["trigger"])
	require.Equal(t, sourceRun.ID.String(), newRun.Metadata["rerun_of_run_id"])
	require.Equal(t, sourceRun.Slug, newRun.Metadata["rerun_of_run_slug"])
	require.Equal(t, "all", newRun.Metadata["rerun_mode"])

	clonedJobs, err := store.ListJobs(ctx, &newRun.ID, nil, nil, 100, 0)
	require.NoError(t, err)
	require.Len(t, clonedJobs, 2)
	require.Len(t, queue.published, 2)
	for _, cloned := range clonedJobs {
		require.Equal(t, domain.JobStatusPending, cloned.Status)
		require.Equal(t, sourceRun.ID.String(), cloned.Metadata["rerun_of_run_id"])
		require.NotEmpty(t, cloned.Metadata["rerun_of_job_id"])
	}
}

func TestHandleRerunRun_FailedOnlyRequiresMatchingJobs(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	workspace, err := store.GetWorkspaceBySlug(ctx, "default")
	require.NoError(t, err)

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "test-project",
		Name:        "Test Project",
		CreatedBy:   "test-user",
	}
	require.NoError(t, store.CreateProject(ctx, project))

	sourceRun := &domain.Run{
		ID:        uuid.New(),
		ProjectID: project.ID,
		Slug:      "source-run",
		Name:      "Source Run",
		Status:    domain.RunStatusSucceeded,
		CreatedBy: "test-user",
	}
	require.NoError(t, store.CreateRun(ctx, sourceRun))

	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       sourceRun.ID,
		Name:        ptr("job-success"),
		CreatedBy:   "test-user",
		Status:      domain.JobStatusSucceeded,
		Image:       "alpine:latest",
		Command:     []string{"echo", "ok"},
		Env:         map[string]string{},
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 60,
		Executor:    domain.ExecutorTypeDocker,
	}
	require.NoError(t, store.CreateJob(ctx, job))

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	queue := &recordingQueue{}
	api := &API{cfg: &config.Config{API: config.APIConfig{BaseURL: "http://localhost:8080"}}, store: store, logger: logger, queue: queue}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/rerun", api.HandleRerunRun)

	body, _ := json.Marshal(map[string]any{"mode": "failed_only"})
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects/test-project/runs/source-run/rerun", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Empty(t, queue.published)
}
