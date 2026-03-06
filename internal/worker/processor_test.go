package worker

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/registrysecrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock Executor
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) CreateRun(ctx context.Context, spec executor.RunSpec) (executor.RunRef, error) {
	args := m.Called(ctx, spec)
	return args.Get(0).(executor.RunRef), args.Error(1)
}

func (m *MockExecutor) Wait(ctx context.Context, ref executor.RunRef) (executor.Result, error) {
	args := m.Called(ctx, ref)
	return args.Get(0).(executor.Result), args.Error(1)
}

func (m *MockExecutor) GetLogs(ctx context.Context, ref executor.RunRef, w io.Writer) error {
	args := m.Called(ctx, ref, w)
	return args.Error(0)
}

func (m *MockExecutor) CollectOutputs(ctx context.Context, ref executor.RunRef, spec executor.OutputSpec) ([]domain.Artefact, error) {
	args := m.Called(ctx, ref, spec)
	return args.Get(0).([]domain.Artefact), args.Error(1)
}

func (m *MockExecutor) Cancel(ctx context.Context, ref executor.RunRef) error {
	args := m.Called(ctx, ref)
	return args.Error(0)
}

func (m *MockExecutor) Cleanup(ctx context.Context, ref executor.RunRef) error {
	args := m.Called(ctx, ref)
	return args.Error(0)
}

func (m *MockExecutor) Status(ctx context.Context, ref executor.RunRef) (executor.ExecutorStatus, error) {
	args := m.Called(ctx, ref)
	return args.Get(0).(executor.ExecutorStatus), args.Error(1)
}

// Mock Store
type MockStore struct {
	mock.Mock
}

func (m *MockStore) GetJob(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Job), args.Error(1)
}

func (m *MockStore) UpdateJob(ctx context.Context, job *domain.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockStore) SaveArtefacts(ctx context.Context, jobID uuid.UUID, artefacts []domain.Artefact) error {
	args := m.Called(ctx, jobID, artefacts)
	return args.Error(0)
}

func (m *MockStore) GetRun(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Run), args.Error(1)
}

func (m *MockStore) GetProject(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Project), args.Error(1)
}

func (m *MockStore) GetWorkspace(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Workspace), args.Error(1)
}

func (m *MockStore) RecomputeRunStatus(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStore) GetRegistrySecret(ctx context.Context, id uuid.UUID) (*domain.RegistrySecret, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RegistrySecret), args.Error(1)
}

// Mock Storage
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Upload(ctx context.Context, key string, r io.Reader, contentType string) error {
	args := m.Called(ctx, key, r, contentType)
	return args.Error(0)
}

func (m *MockStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorage) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	args := m.Called(ctx, key, expiry)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ObjectInfo), args.Error(1)
}

// createTestHierarchy creates test workspace, project, and run objects
func createTestHierarchy() (*domain.Workspace, *domain.Project, *domain.Run) {
	workspace := &domain.Workspace{
		ID:   uuid.New(),
		Slug: "test-workspace",
		Name: "Test Workspace",
	}

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "test-project",
		Name:        "Test Project",
		CreatedBy:   "test-user",
	}

	run := &domain.Run{
		ID:        uuid.New(),
		ProjectID: project.ID,
		Slug:      "test-run",
		Status:    domain.RunStatusPending,
		CreatedBy: "test-user",
	}

	return workspace, project, run
}

// createTestJob creates a test job with default values
func createTestJob(runID uuid.UUID) *domain.Job {
	return &domain.Job{
		ID:          uuid.New(),
		RunID:       runID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		Env:         map[string]string{"FOO": "bar"},
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		CreatedAt:   time.Now(),
	}
}

func TestProcessor_Process_Success(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)
	job.GPUCount = 1

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Setup expectations
	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	ref := executor.RunRef{
		ExecutorType: "swarm",
		Reference:    "service-123",
	}
	mockExecutor.On("CreateRun", ctx, mock.MatchedBy(func(spec executor.RunSpec) bool {
		return spec.GPUCount == 1 && len(spec.GPUDeviceIDs) == 1 && spec.GPUDeviceIDs[0] == "0"
	})).Return(ref, nil)

	result := executor.Result{
		Status:     executor.StatusSuccess,
		ExitCode:   0,
		StartedAt:  time.Now(),
		FinishedAt: time.Now().Add(5 * time.Second),
	}
	mockExecutor.On("Wait", mock.Anything, ref).Return(result, nil)
	mockExecutor.On("GetLogs", ctx, ref, mock.Anything).Return(nil)

	artefacts := []domain.Artefact{
		{Path: "/outputs/file.txt", ObjectKey: "key", SizeBytes: 100, ContentType: "text/plain"},
	}
	mockExecutor.On("CollectOutputs", ctx, ref, mock.Anything).Return(artefacts, nil)

	mockStore.On("SaveArtefacts", ctx, job.ID, artefacts).Return(nil)
	mockStorage.On("Upload", ctx, mock.Anything, mock.Anything, "text/plain").Return(nil)

	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusSucceeded && j.ExitCode != nil && *j.ExitCode == 0
	})).Return(nil)

	mockExecutor.On("Cleanup", ctx, ref).Return(nil)

	// Execute
	err := processor.ProcessWithAllocation(ctx, job.ID, []string{"0"})

	// Verify
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestProcessor_Process_FailedExitCode(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Setup expectations
	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	ref := executor.RunRef{ExecutorType: "swarm", Reference: "service-123"}
	mockExecutor.On("CreateRun", ctx, mock.Anything).Return(ref, nil)

	// Job exits with non-zero code
	result := executor.Result{
		Status:     executor.StatusFailed,
		ExitCode:   1,
		StartedAt:  time.Now(),
		FinishedAt: time.Now().Add(5 * time.Second),
	}
	mockExecutor.On("Wait", mock.Anything, ref).Return(result, nil)
	mockExecutor.On("GetLogs", ctx, ref, mock.Anything).Return(nil)
	mockStorage.On("Upload", ctx, mock.Anything, mock.Anything, "text/plain").Return(nil)

	// Should mark as FAILED with exit code
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusFailed &&
			j.ExitCode != nil && *j.ExitCode == 1 &&
			j.StatusMessage != nil
	})).Return(nil)

	mockExecutor.On("Cleanup", ctx, ref).Return(nil)

	// Execute
	err := processor.Process(ctx, job.ID)

	// Verify - should complete without error even though job failed
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}

func TestProcessor_Process_Timeout(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)
	job.TimeoutSecs = 1 // 1 second timeout

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Setup expectations
	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	ref := executor.RunRef{ExecutorType: "swarm", Reference: "service-123"}
	mockExecutor.On("CreateRun", ctx, mock.Anything).Return(ref, nil)

	// Wait returns context deadline exceeded
	mockExecutor.On("Wait", mock.Anything, ref).Return(executor.Result{}, context.DeadlineExceeded)
	mockExecutor.On("Cancel", mock.Anything, ref).Return(nil)
	mockExecutor.On("GetLogs", mock.Anything, ref, mock.Anything).Return(nil)
	mockStorage.On("Upload", mock.Anything, mock.Anything, mock.Anything, "text/plain").Return(nil)

	// Should mark as TIMEOUT
	mockStore.On("UpdateJob", mock.Anything, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusTimeout &&
			j.StatusMessage != nil
	})).Return(nil)

	mockExecutor.On("Cleanup", mock.Anything, ref).Return(nil)

	// Execute
	err := processor.Process(ctx, job.ID)

	// Verify
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}

func TestProcessor_Process_ExecutorCreateFailure(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Setup expectations
	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	// Executor creation fails
	mockExecutor.On("CreateRun", ctx, mock.Anything).Return(executor.RunRef{}, errors.New("failed to create service"))

	// Should mark as FAILED
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusFailed &&
			j.StatusMessage != nil
	})).Return(nil)

	// Execute
	err := processor.Process(ctx, job.ID)

	// Verify - should return error (job marked failed in DB though)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create service")
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}

func TestProcessor_Process_GetJobFailure(t *testing.T) {
	ctx := context.Background()
	jobID := uuid.New()

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Job not found
	mockStore.On("GetJob", ctx, jobID).Return(nil, errors.New("job not found"))

	// Execute
	err := processor.Process(ctx, jobID)

	// Verify - should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch job")
	mockStore.AssertExpectations(t)
}

func TestProcessor_Process_LogUploadFailure_NonFatal(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Setup expectations
	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	ref := executor.RunRef{ExecutorType: "swarm", Reference: "service-123"}
	mockExecutor.On("CreateRun", ctx, mock.Anything).Return(ref, nil)

	result := executor.Result{
		Status:     executor.StatusSuccess,
		ExitCode:   0,
		StartedAt:  time.Now(),
		FinishedAt: time.Now().Add(5 * time.Second),
	}
	mockExecutor.On("Wait", mock.Anything, ref).Return(result, nil)
	mockExecutor.On("GetLogs", ctx, ref, mock.Anything).Return(nil)

	// Log upload fails - should not stop job processing
	mockStorage.On("Upload", ctx, mock.Anything, mock.Anything, "text/plain").Return(errors.New("s3 upload failed"))

	artefacts := []domain.Artefact{}
	mockExecutor.On("CollectOutputs", ctx, ref, mock.Anything).Return(artefacts, nil)
	// Note: SaveArtefacts is NOT called when artefacts list is empty

	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusSucceeded
	})).Return(nil)

	mockExecutor.On("Cleanup", ctx, ref).Return(nil)

	// Execute
	err := processor.Process(ctx, job.ID)

	// Verify - should still succeed despite log upload failure
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestProcessor_Process_ArtefactCollectionFailure_NonFatal(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Setup expectations
	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	ref := executor.RunRef{ExecutorType: "swarm", Reference: "service-123"}
	mockExecutor.On("CreateRun", ctx, mock.Anything).Return(ref, nil)

	result := executor.Result{
		Status:     executor.StatusSuccess,
		ExitCode:   0,
		StartedAt:  time.Now(),
		FinishedAt: time.Now().Add(5 * time.Second),
	}
	mockExecutor.On("Wait", mock.Anything, ref).Return(result, nil)
	mockExecutor.On("GetLogs", ctx, ref, mock.Anything).Return(nil)
	mockStorage.On("Upload", ctx, mock.Anything, mock.Anything, "text/plain").Return(nil)

	// Artefact collection fails - should not stop job from being marked successful
	mockExecutor.On("CollectOutputs", ctx, ref, mock.Anything).Return([]domain.Artefact{}, errors.New("nfs read failed"))

	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusSucceeded
	})).Return(nil)

	mockExecutor.On("Cleanup", ctx, ref).Return(nil)

	// Execute
	err := processor.Process(ctx, job.ID)

	// Verify - should still succeed
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}

func TestProcessor_Process_NoArtefactsOnFailedJob(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cleanupCfg := config.CleanupConfig{RemoveOnComplete: true}
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", cleanupCfg)

	// Setup expectations
	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	ref := executor.RunRef{ExecutorType: "swarm", Reference: "service-123"}
	mockExecutor.On("CreateRun", ctx, mock.Anything).Return(ref, nil)

	// Job fails with non-zero exit code
	result := executor.Result{
		Status:     executor.StatusFailed,
		ExitCode:   1,
		StartedAt:  time.Now(),
		FinishedAt: time.Now().Add(5 * time.Second),
	}
	mockExecutor.On("Wait", mock.Anything, ref).Return(result, nil)
	mockExecutor.On("GetLogs", ctx, ref, mock.Anything).Return(nil)
	mockStorage.On("Upload", ctx, mock.Anything, mock.Anything, "text/plain").Return(nil)

	// CollectOutputs should NOT be called for failed jobs
	// (verified by not setting up the mock expectation)

	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusFailed
	})).Return(nil)

	mockExecutor.On("Cleanup", ctx, ref).Return(nil)

	// Execute
	err := processor.Process(ctx, job.ID)

	// Verify
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	// Verify CollectOutputs was NOT called
	mockExecutor.AssertNotCalled(t, "CollectOutputs", ctx, ref, mock.Anything)
}

func TestProcessor_Process_DecryptsRegistrySecretBeforeCreateRun(t *testing.T) {
	ctx := context.Background()
	workspace, project, run := createTestHierarchy()
	job := createTestJob(run.ID)
	secretID := uuid.New()
	job.RegistrySecretID = &secretID

	mockStore := new(MockStore)
	mockExecutor := new(MockExecutor)
	mockStorage := new(MockStorage)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	processor := NewProcessor(mockStore, mockExecutor, mockStorage, logger, "http://localhost:8080", "test-bucket", "", config.CleanupConfig{RemoveOnComplete: true})

	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	codec, err := registrysecrets.NewCodec(config.RegistrySecretEncryptionConfig{
		Enabled:     true,
		ActiveKeyID: "key-1",
		ActiveKey:   key,
	})
	require.NoError(t, err)
	processor.SetSecretCodec(codec)

	ciphertext, encoding, keyID, err := codec.Encrypt(workspace.ID, "docker.io", "robot", "super-secret")
	require.NoError(t, err)

	secret := &domain.RegistrySecret{
		ID:                secretID,
		WorkspaceID:       workspace.ID,
		Host:              "docker.io",
		Username:          "robot",
		PasswordEncrypted: ciphertext,
		SecretEncoding:    encoding,
		SecretKeyID:       keyID,
		Active:            true,
	}

	mockStore.On("GetJob", ctx, job.ID).Return(job, nil)
	mockStore.On("GetRun", ctx, run.ID).Return(run, nil)
	mockStore.On("GetProject", ctx, project.ID).Return(project, nil)
	mockStore.On("GetWorkspace", ctx, workspace.ID).Return(workspace, nil)
	mockStore.On("GetRegistrySecret", ctx, secretID).Return(secret, nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusRunning
	})).Return(nil)
	mockStore.On("RecomputeRunStatus", ctx, run.ID).Return(nil).Twice()

	ref := executor.RunRef{ExecutorType: "swarm", Reference: "service-123"}
	mockExecutor.On("CreateRun", ctx, mock.MatchedBy(func(spec executor.RunSpec) bool {
		return spec.RegistryAuth != nil && spec.RegistryAuth.Password == "super-secret"
	})).Return(ref, nil)
	mockExecutor.On("Wait", mock.Anything, ref).Return(executor.Result{
		Status:     executor.StatusSuccess,
		ExitCode:   0,
		StartedAt:  time.Now(),
		FinishedAt: time.Now().Add(1 * time.Second),
	}, nil)
	mockExecutor.On("GetLogs", ctx, ref, mock.Anything).Return(nil)
	mockExecutor.On("CollectOutputs", ctx, ref, mock.Anything).Return([]domain.Artefact{}, nil)
	mockStorage.On("Upload", ctx, mock.Anything, mock.Anything, "text/plain").Return(nil)
	mockStore.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.Job) bool {
		return j.Status == domain.JobStatusSucceeded
	})).Return(nil)
	mockExecutor.On("Cleanup", ctx, ref).Return(nil)

	err = processor.Process(ctx, job.ID)
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}
