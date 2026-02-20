package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupTestPostgres creates a test postgres container with migrations and returns a store
func setupTestPostgres(t *testing.T) (*postgres.Store, func()) {
	t.Helper()

	// Setup Postgres container with migrations
	pgContainer := testutil.SetupTestPostgres(t)

	// Create store
	store, err := postgres.New(pgContainer.ConnString)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		pgContainer.Cleanup(t)
	}

	return store, cleanup
}

// setupTestRouter creates a test HTTP router with the cancel endpoint registered
func setupTestRouter(api *API) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/cancel", api.HandleCancelJob)
	return mux
}

// setupTestHierarchy creates a test workspace, project, and run for testing jobs
func setupTestHierarchy(t *testing.T, store *postgres.Store, ctx context.Context) (workspace *domain.Workspace, project *domain.Project, run *domain.Run) {
	t.Helper()

	// Create workspace
	workspace = &domain.Workspace{
		ID:   uuid.New(),
		Slug: "test-workspace",
		Name: "Test Workspace",
	}
	err := store.CreateWorkspace(ctx, workspace)
	require.NoError(t, err)

	// Create project
	project = &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "test-project",
		Name:        "Test Project",
		CreatedBy:   "test-user",
	}
	err = store.CreateProject(ctx, project)
	require.NoError(t, err)

	// Create run
	run = &domain.Run{
		ID:        uuid.New(),
		ProjectID: project.ID,
		Slug:      "test-run",
		Status:    domain.RunStatusPending,
		CreatedBy: "test-user",
	}
	err = store.CreateRun(ctx, run)
	require.NoError(t, err)

	return workspace, project, run
}

// TestHandleCancelJob_Integration_PendingJob tests cancelling a job that hasn't started
func TestHandleCancelJob_Integration_PendingJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	mockExecutor := new(MockExecutor)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	api := &API{
		cfg:      &config.Config{},
		store:    store,
		executor: mockExecutor,
		logger:   logger,
		baseURL:  "http://test.local",
	}

	// Create a test server with router to properly handle path parameters
	mux := setupTestRouter(api)

	// Create a pending job in the database
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		Outputs:     []string{"/output"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
	}

	err := store.CreateJob(ctx, job)
	require.NoError(t, err)

	// Call the cancel endpoint with hierarchical URL
	url := fmt.Sprintf("/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/cancel",
		"test-workspace", "test-project", "test-run", job.ID.String())
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Verify response
	t.Logf("Response code: %d", w.Code)
	t.Logf("Response body: %s", w.Body.String())
	if w.Code != http.StatusOK {
		t.Logf("Unexpected status code: %d", w.Code)
	}
	assert.Equal(t, http.StatusOK, w.Code)

	var resp JobResponse
	bodyBytes := w.Body.Bytes()
	err = json.Unmarshal(bodyBytes, &resp)
	t.Logf("Response decoded: %+v", resp)
	assert.NoError(t, err)
	assert.Equal(t, job.ID, resp.ID)
	assert.Equal(t, "CANCELLED", resp.Status)
	assert.NotNil(t, resp.StatusMessage)
	assert.Equal(t, "Job cancelled before execution", *resp.StatusMessage)
	assert.NotNil(t, resp.FinishedAt)

	// Verify job was updated in database
	updatedJob, err := store.GetJob(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusCancelled, updatedJob.Status)
	assert.NotNil(t, updatedJob.StatusMessage)
	assert.Equal(t, "Job cancelled before execution", *updatedJob.StatusMessage)
	assert.NotNil(t, updatedJob.FinishedAt)

	// Verify executor was NOT called
	mockExecutor.AssertNotCalled(t, "Cancel")
}

// TestHandleCancelJob_Integration_RunningJob tests cancelling a running job
func TestHandleCancelJob_Integration_RunningJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	mockExecutor := new(MockExecutor)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	api := &API{
		cfg:      &config.Config{},
		store:    store,
		executor: mockExecutor,
		logger:   logger,
		baseURL:  "http://test.local",
	}

	// Create a test server with router to properly handle path parameters
	mux := setupTestRouter(api)

	// Create a job, then update it to RUNNING status
	// (mimicking the real workflow where worker updates job when starting execution)
	executorRef := "swarm-service-123"
	startedAt := time.Now().Add(-5 * time.Minute)
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Command:     []string{"sleep", "3600"},
		Outputs:     []string{"/output"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
	}

	err := store.CreateJob(ctx, job)
	require.NoError(t, err)

	// Update to RUNNING status with executor ref
	// (CreateJob doesn't set executor_ref, started_at - these are set via UpdateJob)
	job.Status = domain.JobStatusRunning
	job.ExecutorRef = &executorRef
	job.StartedAt = &startedAt
	err = store.UpdateJob(ctx, job)
	require.NoError(t, err)

	// Verify the job was updated correctly with executor ref
	runningJob, err := store.GetJob(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotNil(t, runningJob.ExecutorRef, "Executor ref should not be nil")
	require.Equal(t, executorRef, *runningJob.ExecutorRef)

	// Mock executor Cancel to return success
	// Use mock.MatchedBy for flexible matching
	mockExecutor.On("Cancel", mock.Anything, mock.MatchedBy(func(ref executor.RunRef) bool {
		return ref.ExecutorType == string(domain.ExecutorTypeSwarm) &&
			ref.Reference == executorRef
	})).Return(nil)

	// Call the cancel endpoint with hierarchical URL
	url := fmt.Sprintf("/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/cancel",
		"test-workspace", "test-project", "test-run", job.ID.String())
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Logf("Unexpected status code: %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
		t.Logf("Mock calls: %v", mockExecutor.Calls)
	}
	require.Equal(t, http.StatusOK, w.Code, "Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())

	var resp JobResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, job.ID, resp.ID)
	assert.Equal(t, "CANCELLED", resp.Status)
	assert.NotNil(t, resp.StatusMessage)
	assert.Equal(t, "Job cancelled by user", *resp.StatusMessage)
	assert.NotNil(t, resp.FinishedAt)

	// Verify job was updated in database
	updatedJob, err := store.GetJob(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusCancelled, updatedJob.Status)
	assert.NotNil(t, updatedJob.StatusMessage)
	assert.Equal(t, "Job cancelled by user", *updatedJob.StatusMessage)
	assert.NotNil(t, updatedJob.FinishedAt)

	// Verify executor was called
	mockExecutor.AssertExpectations(t)
}

// TestHandleCancelJob_Integration_TerminalStates tests that terminal jobs cannot be cancelled
func TestHandleCancelJob_Integration_TerminalStates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	terminalStates := []struct {
		status domain.JobStatus
		name   string
	}{
		{domain.JobStatusSucceeded, "SUCCEEDED"},
		{domain.JobStatusFailed, "FAILED"},
		{domain.JobStatusCancelled, "CANCELLED"},
		{domain.JobStatusTimeout, "TIMEOUT"},
	}

	for _, ts := range terminalStates {
		t.Run(ts.name, func(t *testing.T) {
			store, cleanup := setupTestPostgres(t)
			defer cleanup()

			ctx := context.Background()
			_, _, run := setupTestHierarchy(t, store, ctx)

			mockExecutor := new(MockExecutor)
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			api := &API{
				cfg:      &config.Config{},
				store:    store,
				executor: mockExecutor,
				logger:   logger,
				baseURL:  "http://test.local",
			}

			// Create a test server with router to properly handle path parameters
			mux := setupTestRouter(api)

			// Create a job in terminal state
			finishedAt := time.Now().Add(-1 * time.Minute)
			job := &domain.Job{
				ID:          uuid.New(),
				RunID:       run.ID,
				CreatedAt:   time.Now().Add(-10 * time.Minute),
				UpdatedAt:   time.Now().Add(-1 * time.Minute),
				CreatedBy:   "test-user",
				Status:      ts.status,
				Image:       "alpine:latest",
				Command:     []string{"echo", "hello"},
				Outputs:     []string{"/output"},
				TimeoutSecs: 3600,
				Executor:    domain.ExecutorTypeSwarm,
				FinishedAt:  &finishedAt,
			}

			err := store.CreateJob(ctx, job)
			require.NoError(t, err)

			// Call the cancel endpoint with hierarchical URL
			url := fmt.Sprintf("/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/cancel",
				"test-workspace", "test-project", "test-run", job.ID.String())
			req := httptest.NewRequest(http.MethodPost, url, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, http.StatusConflict, w.Code)

			var errResp ErrorResponse
			err = json.NewDecoder(w.Body).Decode(&errResp)
			assert.NoError(t, err)
			assert.Equal(t, "cannot_cancel", errResp.Error)
			assert.Contains(t, errResp.Message, "already in terminal state")
			assert.Contains(t, errResp.Message, ts.name)
			assert.Equal(t, http.StatusConflict, errResp.Code)

			// Verify job was NOT modified
			unchangedJob, err := store.GetJob(context.Background(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, ts.status, unchangedJob.Status)

			// Verify executor was NOT called
			mockExecutor.AssertNotCalled(t, "Cancel")
		})
	}
}

// TestHandleCancelJob_Integration_JobNotFound tests 404 error
func TestHandleCancelJob_Integration_JobNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	mockExecutor := new(MockExecutor)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	api := &API{
		cfg:      &config.Config{},
		store:    store,
		executor: mockExecutor,
		logger:   logger,
		baseURL:  "http://test.local",
	}

	// Create a test server with router to properly handle path parameters
	mux := setupTestRouter(api)

	// Use a non-existent job ID
	jobID := uuid.New()

	// Call the cancel endpoint with hierarchical URL
	url := fmt.Sprintf("/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/cancel",
		"test-workspace", "test-project", "test-run", jobID.String())
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Equal(t, "not_found", errResp.Error)
	assert.Equal(t, "Job not found", errResp.Message)

	mockExecutor.AssertNotCalled(t, "Cancel")
}

// TestHandleCancelJob_Integration_RunningJobNoExecutorRef tests error when running job has no executor reference
func TestHandleCancelJob_Integration_RunningJobNoExecutorRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name        string
		executorRef *string
	}{
		{
			name:        "nil executor ref",
			executorRef: nil,
		},
		{
			name:        "empty executor ref",
			executorRef: stringPtr(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestPostgres(t)
			defer cleanup()

			ctx := context.Background()
			_, _, run := setupTestHierarchy(t, store, ctx)

			mockExecutor := new(MockExecutor)
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			api := &API{
				cfg:      &config.Config{},
				store:    store,
				executor: mockExecutor,
				logger:   logger,
				baseURL:  "http://test.local",
			}

			// Create a test server with router to properly handle path parameters
			mux := setupTestRouter(api)

			// Create a running job WITHOUT executor ref (orphaned state)
			startedAt := time.Now().Add(-5 * time.Minute)
			job := &domain.Job{
				ID:          uuid.New(),
				RunID:       run.ID,
				CreatedAt:   time.Now().Add(-10 * time.Minute),
				UpdatedAt:   time.Now().Add(-5 * time.Minute),
				CreatedBy:   "test-user",
				Status:      domain.JobStatusRunning,
				Image:       "alpine:latest",
				Command:     []string{"sleep", "3600"},
				Outputs:     []string{"/output"},
				TimeoutSecs: 3600,
				Executor:    domain.ExecutorTypeSwarm,
				ExecutorRef: tt.executorRef,
				StartedAt:   &startedAt,
			}

			err := store.CreateJob(ctx, job)
			require.NoError(t, err)

			// Call the cancel endpoint with hierarchical URL
			url := fmt.Sprintf("/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/cancel",
				"test-workspace", "test-project", "test-run", job.ID.String())
			req := httptest.NewRequest(http.MethodPost, url, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, http.StatusInternalServerError, w.Code)

			var errResp ErrorResponse
			err = json.NewDecoder(w.Body).Decode(&errResp)
			assert.NoError(t, err)
			assert.Equal(t, "cancel_failed", errResp.Error)
			assert.Contains(t, errResp.Message, "no executor reference")

			// Verify job was NOT modified
			unchangedJob, err := store.GetJob(context.Background(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, domain.JobStatusRunning, unchangedJob.Status)

			// Verify executor was NOT called
			mockExecutor.AssertNotCalled(t, "Cancel")
		})
	}
}

// TestHandleCancelJob_Integration_ExecutorCancelFails tests handling of executor failures
func TestHandleCancelJob_Integration_ExecutorCancelFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	mockExecutor := new(MockExecutor)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	api := &API{
		cfg:      &config.Config{},
		store:    store,
		executor: mockExecutor,
		logger:   logger,
		baseURL:  "http://test.local",
	}

	// Create a test server with router to properly handle path parameters
	mux := setupTestRouter(api)

	// Create a job, then update it to RUNNING status
	executorRef := "swarm-service-123"
	startedAt := time.Now().Add(-5 * time.Minute)
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Command:     []string{"sleep", "3600"},
		Outputs:     []string{"/output"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
	}

	err := store.CreateJob(ctx, job)
	require.NoError(t, err)

	// Update to RUNNING status with executor ref
	job.Status = domain.JobStatusRunning
	job.ExecutorRef = &executorRef
	job.StartedAt = &startedAt
	err = store.UpdateJob(ctx, job)
	require.NoError(t, err)

	// Mock executor Cancel to fail
	// Use mock.MatchedBy for flexible matching
	mockExecutor.On("Cancel", mock.Anything, mock.MatchedBy(func(ref executor.RunRef) bool {
		return ref.ExecutorType == string(domain.ExecutorTypeSwarm) &&
			ref.Reference == executorRef
	})).Return(fmt.Errorf("executor service unavailable"))

	// Call the cancel endpoint with hierarchical URL
	url := fmt.Sprintf("/v1/workspaces/%s/projects/%s/runs/%s/jobs/%s/cancel",
		"test-workspace", "test-project", "test-run", job.ID.String())
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResp ErrorResponse
	err = json.NewDecoder(w.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Equal(t, "cancel_failed", errResp.Error)
	assert.Contains(t, errResp.Message, "Failed to cancel job")
	assert.Contains(t, errResp.Message, "executor service unavailable")

	// Verify job was NOT modified (cancel failed, so shouldn't update DB)
	unchangedJob, err := store.GetJob(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusRunning, unchangedJob.Status)

	// Verify executor was called
	mockExecutor.AssertExpectations(t)
}
