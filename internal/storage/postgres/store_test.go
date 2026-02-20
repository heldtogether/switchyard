package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a Postgres container with migrations and returns a store
func setupTestDB(t *testing.T) (*Store, func()) {
	t.Helper()

	// Setup Postgres container with migrations
	pgContainer := testutil.SetupTestPostgres(t)

	// Create store
	store, err := New(pgContainer.ConnString)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		pgContainer.Cleanup(t)
	}

	return store, cleanup
}

// setupTestHierarchy creates a test workspace, project, and run for testing jobs
func setupTestHierarchy(t *testing.T, store *Store, ctx context.Context) (workspace *domain.Workspace, project *domain.Project, run *domain.Run) {
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
		Name:      "Test Run",
		Status:    domain.RunStatusPending,
		CreatedBy: "test-user",
	}
	err = store.CreateRun(ctx, run)
	require.NoError(t, err)

	return workspace, project, run
}

func TestStore_CreateJob(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		Env:         map[string]string{"FOO": "bar"},
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
	}

	err := store.CreateJob(ctx, job)
	require.NoError(t, err)

	// Verify job was created
	retrieved, err := store.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, retrieved.ID)
	assert.Equal(t, job.CreatedBy, retrieved.CreatedBy)
	assert.Equal(t, job.Status, retrieved.Status)
	assert.Equal(t, job.Image, retrieved.Image)
	assert.Equal(t, job.Command, retrieved.Command)
	assert.Equal(t, job.Env, retrieved.Env)
	assert.Equal(t, job.Outputs, retrieved.Outputs)
	assert.Equal(t, job.TimeoutSecs, retrieved.TimeoutSecs)
	assert.Equal(t, job.Executor, retrieved.Executor)
}

func TestStore_GetJob_NotFound(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	nonExistentID := uuid.New()
	_, err := store.GetJob(ctx, nonExistentID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStore_UpdateJob(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	// Create initial job
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		Env:         map[string]string{},
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
	}
	err := store.CreateJob(ctx, job)
	require.NoError(t, err)

	// Update job
	job.Status = domain.JobStatusRunning
	msg := "Job is running"
	job.StatusMessage = &msg
	now := time.Now()
	job.StartedAt = &now
	ref := "service-123"
	job.ExecutorRef = &ref
	exitCode := 0
	job.ExitCode = &exitCode

	err = store.UpdateJob(ctx, job)
	require.NoError(t, err)

	// Verify updates
	retrieved, err := store.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusRunning, retrieved.Status)
	assert.Equal(t, msg, *retrieved.StatusMessage)
	assert.NotNil(t, retrieved.StartedAt)
	assert.Equal(t, ref, *retrieved.ExecutorRef)
	assert.Equal(t, 0, *retrieved.ExitCode)
}

func TestStore_ListJobs(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	// Create multiple jobs with different statuses
	jobs := []*domain.Job{
		{
			ID:          uuid.New(),
			RunID:       run.ID,
			CreatedBy:   "user1",
			Status:      domain.JobStatusPending,
			Image:       "alpine:latest",
			Command:     []string{},
			Env:         map[string]string{},
			Outputs:     []string{"/outputs"},
			TimeoutSecs: 3600,
			Executor:    domain.ExecutorTypeSwarm,
		},
		{
			ID:          uuid.New(),
			RunID:       run.ID,
			CreatedBy:   "user1",
			Status:      domain.JobStatusRunning,
			Image:       "alpine:latest",
			Command:     []string{},
			Env:         map[string]string{},
			Outputs:     []string{"/outputs"},
			TimeoutSecs: 3600,
			Executor:    domain.ExecutorTypeSwarm,
		},
		{
			ID:          uuid.New(),
			RunID:       run.ID,
			CreatedBy:   "user2",
			Status:      domain.JobStatusSucceeded,
			Image:       "alpine:latest",
			Command:     []string{},
			Env:         map[string]string{},
			Outputs:     []string{"/outputs"},
			TimeoutSecs: 3600,
			Executor:    domain.ExecutorTypeSwarm,
		},
	}

	for _, job := range jobs {
		err := store.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		status        *domain.JobStatus
		createdBy     *string
		expectedCount int
	}{
		{"all jobs", nil, nil, 3},
		{"pending jobs", func() *domain.JobStatus { s := domain.JobStatusPending; return &s }(), nil, 1},
		{"running jobs", func() *domain.JobStatus { s := domain.JobStatusRunning; return &s }(), nil, 1},
		{"user1 jobs", nil, func() *string { s := "user1"; return &s }(), 2},
		{"user2 jobs", nil, func() *string { s := "user2"; return &s }(), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.ListJobs(ctx, nil, tt.status, tt.createdBy, 100, 0)
			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)
		})
	}
}

func TestStore_SaveArtefacts(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	// Create a job first
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusSucceeded,
		Image:       "alpine:latest",
		Command:     []string{},
		Env:         map[string]string{},
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
	}
	err := store.CreateJob(ctx, job)
	require.NoError(t, err)

	// Save artefacts
	artefacts := []domain.Artefact{
		{
			Path:        "/outputs/file1.txt",
			ObjectKey:   "jobs/123/outputs/file1.txt",
			SizeBytes:   100,
			ContentType: "text/plain",
		},
		{
			Path:        "/outputs/file2.txt",
			ObjectKey:   "jobs/123/outputs/file2.txt",
			SizeBytes:   200,
			ContentType: "text/plain",
		},
	}

	err = store.SaveArtefacts(ctx, job.ID, artefacts)
	require.NoError(t, err)

	// Retrieve artefacts
	retrieved, err := store.GetArtefacts(ctx, job.ID)
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)

	// Verify artefact details
	assert.Equal(t, artefacts[0].Path, retrieved[0].Path)
	assert.Equal(t, artefacts[0].ObjectKey, retrieved[0].ObjectKey)
	assert.Equal(t, artefacts[0].SizeBytes, retrieved[0].SizeBytes)
	assert.Equal(t, artefacts[0].ContentType, retrieved[0].ContentType)
}

func TestStore_GetRunningJobs(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	// Create jobs in various states
	statuses := []domain.JobStatus{
		domain.JobStatusPending,
		domain.JobStatusRunning,
		domain.JobStatusRunning,
		domain.JobStatusSucceeded,
		domain.JobStatusFailed,
	}

	for _, status := range statuses {
		job := &domain.Job{
			ID:          uuid.New(),
			RunID:       run.ID,
			CreatedBy:   "test-user",
			Status:      status,
			Image:       "alpine:latest",
			Command:     []string{},
			Env:         map[string]string{},
			Outputs:     []string{"/outputs"},
			TimeoutSecs: 3600,
			Executor:    domain.ExecutorTypeSwarm,
		}
		err := store.CreateJob(ctx, job)
		require.NoError(t, err)
	}

	// Get only running jobs
	runningJobs, err := store.GetRunningJobs(ctx)
	require.NoError(t, err)
	assert.Len(t, runningJobs, 2)

	// Verify all are RUNNING
	for _, job := range runningJobs {
		assert.Equal(t, domain.JobStatusRunning, job.Status)
	}
}
