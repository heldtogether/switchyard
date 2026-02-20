package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// testAPI wraps the API to allow overriding the store for testing
type testAPI struct {
	*API
	mockStore *MockStore
}

// Override GetJob to use the mock
func (t *testAPI) getJob(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	return t.mockStore.GetJob(ctx, id)
}

// Override UpdateJob to use the mock
func (t *testAPI) updateJob(ctx context.Context, job *domain.Job) error {
	return t.mockStore.UpdateJob(ctx, job)
}

// MockStore is a mock implementation of postgres.Store methods used by the API
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

func (m *MockStore) CreateJob(ctx context.Context, job *domain.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockStore) ListJobs(ctx context.Context, status *domain.JobStatus, createdBy *string, limit, offset int) ([]*domain.Job, error) {
	args := m.Called(ctx, status, createdBy, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Job), args.Error(1)
}

func (m *MockStore) GetArtefacts(ctx context.Context, jobID uuid.UUID) ([]domain.Artefact, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Artefact), args.Error(1)
}

func (m *MockStore) SaveArtefacts(ctx context.Context, jobID uuid.UUID, artefacts []domain.Artefact) error {
	args := m.Called(ctx, jobID, artefacts)
	return args.Error(0)
}

func (m *MockStore) GetRunningJobs(ctx context.Context) ([]*domain.Job, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Job), args.Error(1)
}

// MockExecutor is a mock implementation of executor.Executor
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

// Helper to create a testable API with mocks
func newTestAPI(mockStore *MockStore, mockExecutor executor.Executor) *API {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create the API struct with a temporary store
	// We'll intercept the calls via modified handlers if needed
	api := &API{
		cfg:      &config.Config{},
		store:    &postgres.Store{}, // placeholder - not actually used in our tests
		executor: mockExecutor,
		logger:   logger,
		baseURL:  "http://test.local",
	}

	// We'll use a wrapper that intercepts store calls
	// For now, we'll test integration-style by using a real DB or refactoring
	return api
}

/*
Note: The current API implementation uses concrete *postgres.Store type.
For proper unit testing, we have two options:

1. Refactor the API to use an interface (recommended for production)
2. Use integration tests with testcontainers (current approach for API tests)

The tests below demonstrate the test scenarios we want to cover.
For now, we'll document them and implement them as integration tests.
*/

// TestHandleCancelJob_Scenarios documents the test scenarios for the cancel handler
// These should be implemented as either:
// - Integration tests with real postgres (using testcontainers)
// - Unit tests after refactoring API to use store interface
func TestHandleCancelJob_Scenarios(t *testing.T) {
	t.Skip("Skipping scenario documentation - implement as integration tests")

	scenarios := []struct {
		name           string
		jobStatus      domain.JobStatus
		hasExecutorRef bool
		executorError  error
		dbError        error
		expectedCode   int
		expectedError  string
	}{
		{
			name:         "cancel pending job successfully",
			jobStatus:    domain.JobStatusPending,
			expectedCode: http.StatusOK,
		},
		{
			name:           "cancel running job successfully",
			jobStatus:      domain.JobStatusRunning,
			hasExecutorRef: true,
			expectedCode:   http.StatusOK,
		},
		{
			name:          "cannot cancel succeeded job",
			jobStatus:     domain.JobStatusSucceeded,
			expectedCode:  http.StatusConflict,
			expectedError: "cannot_cancel",
		},
		{
			name:          "cannot cancel failed job",
			jobStatus:     domain.JobStatusFailed,
			expectedCode:  http.StatusConflict,
			expectedError: "cannot_cancel",
		},
		{
			name:          "cannot cancel already cancelled job",
			jobStatus:     domain.JobStatusCancelled,
			expectedCode:  http.StatusConflict,
			expectedError: "cannot_cancel",
		},
		{
			name:          "cannot cancel timed out job",
			jobStatus:     domain.JobStatusTimeout,
			expectedCode:  http.StatusConflict,
			expectedError: "cannot_cancel",
		},
		{
			name:           "running job without executor ref",
			jobStatus:      domain.JobStatusRunning,
			hasExecutorRef: false,
			expectedCode:   http.StatusInternalServerError,
			expectedError:  "cancel_failed",
		},
		{
			name:           "executor cancel fails",
			jobStatus:      domain.JobStatusRunning,
			hasExecutorRef: true,
			executorError:  errors.New("executor unavailable"),
			expectedCode:   http.StatusInternalServerError,
			expectedError:  "cancel_failed",
		},
		{
			name:          "database update fails after cancel",
			jobStatus:     domain.JobStatusPending,
			dbError:       errors.New("db connection lost"),
			expectedCode:  http.StatusInternalServerError,
			expectedError: "internal_error",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// Test implementation would go here
			// using either integration tests or unit tests after refactoring
		})
	}
}

// Unit tests for path parsing and validation
// Integration tests are in handlers_cancel_integration_test.go
func TestHandleCancelJob_InvalidJobID(t *testing.T) {
	mockExecutor := new(MockExecutor)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	api := &API{
		cfg:      &config.Config{},
		store:    nil, // Will not be called for invalid ID
		executor: mockExecutor,
		logger:   logger,
		baseURL:  "http://test.local",
	}

	tests := []struct {
		name  string
		jobID string
	}{
		{
			name:  "not a UUID",
			jobID: "not-a-uuid",
		},
		{
			name:  "empty string",
			jobID: "",
		},
		{
			name:  "malformed UUID",
			jobID: "123e4567-e89b-12d3-a456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/jobs/"+tt.jobID+"/cancel", nil)
			w := httptest.NewRecorder()

			api.HandleCancelJob(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var errResp ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&errResp)
			assert.NoError(t, err)
			assert.Equal(t, "invalid_id", errResp.Error)
			assert.Equal(t, "Invalid job ID", errResp.Message)
			assert.Equal(t, http.StatusBadRequest, errResp.Code)
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
