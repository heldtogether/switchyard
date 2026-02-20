package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB creates a Postgres container and runs migrations
func setupTestDB(t *testing.T) (*Store, func()) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	store, err := New(connStr)
	require.NoError(t, err)

	// Run migrations by executing the schema
	_, err = store.db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		
		CREATE TYPE job_status AS ENUM (
			'PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'TIMEOUT'
		);
		
		CREATE TYPE executor_type AS ENUM ('swarm', 'kube');
		
		CREATE TABLE jobs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_by VARCHAR(255) NOT NULL,
			status job_status NOT NULL DEFAULT 'PENDING',
			status_message TEXT,
			image VARCHAR(512) NOT NULL,
			image_digest VARCHAR(128),
			command JSONB,
			env JSONB DEFAULT '{}',
			cpu_limit VARCHAR(10),
			memory_limit VARCHAR(20),
			timeout_seconds INTEGER DEFAULT 3600,
			outputs JSONB NOT NULL DEFAULT '[]',
			started_at TIMESTAMPTZ,
			finished_at TIMESTAMPTZ,
			exit_code INTEGER,
			artefact_prefix VARCHAR(512),
			log_object_key VARCHAR(512),
			executor executor_type NOT NULL DEFAULT 'swarm',
			executor_ref VARCHAR(255),
			executor_metadata JSONB DEFAULT '{}',
			registry_secret_id UUID,
			metadata JSONB DEFAULT '{}'
		);
		
		CREATE TABLE artefacts (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			path VARCHAR(512) NOT NULL,
			object_key VARCHAR(512) NOT NULL,
			size_bytes BIGINT NOT NULL,
			content_type VARCHAR(128),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(job_id, path)
		);
	`)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return store, cleanup
}

func TestStore_CreateJob(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	job := &domain.Job{
		ID:          uuid.New(),
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

	// Create initial job
	job := &domain.Job{
		ID:          uuid.New(),
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

	// Create multiple jobs with different statuses
	jobs := []*domain.Job{
		{
			ID:          uuid.New(),
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
			result, err := store.ListJobs(ctx, tt.status, tt.createdBy, 100, 0)
			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)
		})
	}
}

func TestStore_SaveArtefacts(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a job first
	job := &domain.Job{
		ID:          uuid.New(),
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
