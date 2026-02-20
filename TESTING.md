# Switchyard Testing Guide

## Test Coverage Summary

**Overall Coverage:** ~8.3% → Target: 70%+

| Package | Coverage | Status | Priority |
|---------|----------|--------|----------|
| `internal/domain` | **100.0%** | ✅ Complete | HIGH |
| `internal/storage/postgres` | **80.5%** | ✅ Very Good | HIGH |
| `internal/executor` | **27.1%** | 🟡 Partial | HIGH |
| `internal/api` | **1.7%** | 🔴 Started | CRITICAL |
| `internal/worker` | **0%** | 🔴 Not Started | CRITICAL |
| `internal/storage/queue` | **0%** | 🔴 Not Started | MEDIUM |
| `internal/storage/objectstore` | **0%** | 🔴 Not Started | MEDIUM |
| `internal/config` | **0%** | 🔴 Not Started | LOW |

---

## Completed Tests

### ✅ Domain Logic Tests (`internal/domain/job_test.go`)
**Coverage:** 100% | **Tests:** 16 scenarios

**What's Tested:**
- `JobStatus.IsTerminal()` - All 6 statuses (SUCCEEDED, FAILED, CANCELLED, TIMEOUT, PENDING, RUNNING)
- `Job.Duration()` - Timestamp combinations (both present, one nil, both nil)
- Duration calculations with various intervals (1s to 2.5h)

**Why Critical:** Foundation for all business logic. Job state transitions depend on these.

---

### ✅ Environment Variable Validation (`internal/api/validation_test.go`)
**Coverage:** Full validation logic | **Tests:** 9 scenarios

**What's Tested:**
- Valid user variables (MY_VAR, DATABASE_URL, etc.)
- Reserved prefix detection (SWITCHYARD_*)
- Case sensitivity (lowercase 'switchyard_' allowed)
- Partial matches (MY_SWITCHYARD_VAR allowed)
- Empty/nil maps
- Multiple variables with one reserved

**Why Critical:** **SECURITY** - Prevents users from hijacking system environment variables.

---

### ✅ Resource Parsing Tests (`internal/executor/common_test.go`)
**Coverage:** Comprehensive edge cases | **Tests:** 50+ scenarios

**What's Tested:**

**CPU Parsing:**
- Valid formats: "1.0", "0.5", "2", "4.0"
- Edge cases: zero, negative, very large, invalid formats
- Whitespace handling
- Invalid decimals: "1.2.3" → parses as 1.2
- Documents current behavior for edge cases

**Memory Parsing:**
- Units: g/G/gb, m/M/mb, k/K/kb, raw bytes
- Mixed case: "2Gb", "512Mb"
- Decimal values: "1.5g" → truncates to 1
- Invalid units: "512x" → falls through to bytes
- Negative values allowed (current behavior)

**Why Critical:** Invalid resource specs can cause Docker/Swarm failures or DOS attacks.

---

### ✅ Postgres Store Integration Tests (`internal/storage/postgres/store_test.go`)
**Coverage:** 80.5% | **Tests:** 7 integration tests with real Postgres

**What's Tested:**

**CreateJob:**
- Job insertion with all fields
- JSON marshaling (command, env, outputs, metadata)
- Default values applied
- Retrieval verification

**GetJob:**
- Successful retrieval with all fields
- Not found error handling
- JSON unmarshaling verification

**UpdateJob:**
- Status transitions
- Timestamp updates (started_at, finished_at)
- Executor reference storage
- Exit code storage
- Partial updates

**ListJobs:**
- Filter by status (PENDING, RUNNING, SUCCEEDED)
- Filter by created_by user
- Pagination (limit, offset)
- Multiple filter combinations

**SaveArtefacts:**
- Multiple artefact insertion
- Foreign key constraints
- Unique constraint (job_id, path)
- UPSERT behavior for duplicates

**GetRunningJobs:**
- Returns only RUNNING status jobs
- Used by worker recovery
- Empty result handling

**Infrastructure:**
- Uses testcontainers for real Postgres
- Automatic schema creation
- Container cleanup after each test
- Independent test isolation

**Why Critical:** Database is source of truth. Bugs here = data corruption.

---

## Test Infrastructure

### Dependencies Added
```bash
go get github.com/stretchr/testify@latest
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
```

### Running Tests

**All tests:**
```bash
go test ./...
```

**With coverage:**
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out  # View in browser
```

**Specific package:**
```bash
go test -v ./internal/domain/
go test -v ./internal/storage/postgres/
```

**With race detector:**
```bash
go test -race ./...
```

**Integration tests only:**
```bash
go test -v ./internal/storage/postgres/ -timeout=2m
```

---

## Next Priority Tests

### 🔴 CRITICAL: Worker Job Processor
**Package:** `internal/worker/processor_test.go`  
**Priority:** HIGHEST - Core business logic with 0% coverage

**Must Test:**
1. State transitions: PENDING → RUNNING → SUCCEEDED/FAILED/TIMEOUT
2. Timeout handling with executor cancellation
3. Log collection (success and failure paths)
4. Artefact collection (only on success)
5. Database update failures
6. Executor creation failures
7. Exit code interpretation (0 vs non-zero)

**Approach:** Use mocked executor, mocked storage, real test data

---

### 🔴 CRITICAL: API Cancel Handler
**Package:** `internal/api/handlers_cancel_test.go`  
**Priority:** HIGH - User-facing, complex state logic

**Must Test:**
1. Cancel PENDING job (no executor interaction)
2. Cancel RUNNING job (calls executor.Cancel)
3. Reject terminal states (409 Conflict)
4. Invalid job ID (400 Bad Request)
5. Job not found (404 Not Found)
6. Missing executor reference for RUNNING job
7. Executor cancel failure handling
8. Database update failures

**Approach:** Use httptest with mocked dependencies

---

### 🟡 HIGH: Worker Orphan Recovery
**Package:** `internal/worker/worker_test.go`  
**Priority:** HIGH - Critical for reliability

**Must Test:**
1. Find all RUNNING jobs on startup
2. Check executor status for each
3. Map executor status to job status
4. Handle executor status check failures
5. Jobs with missing executor references
6. Multiple orphaned jobs with different states

**Approach:** Integration test with real Postgres, mocked executor

---

### 🟡 HIGH: API Job Creation Handler
**Package:** `internal/api/handlers_create_test.go`  
**Priority:** HIGH - Entry point, validation critical

**Must Test:**
1. Valid job submission
2. Missing required fields (image, outputs)
3. Reserved env var validation
4. Resource limit parsing and defaults
5. Timeout configuration
6. Database failure handling
7. Queue push failure after DB insert (rollback?)
8. Concurrent job submissions

---

## Testing Best Practices

### Table-Driven Tests
Preferred for most scenarios:

```go
func TestSomething(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{"valid input", "123", 123, false},
		{"invalid input", "abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Function(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
```

### Integration Tests with testcontainers
For database/external service tests:

```go
func TestSomething(t *testing.T) {
	ctx := context.Background()
	
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err)
	defer testcontainers.TerminateContainer(container)
	
	connStr, _ := container.ConnectionString(ctx, "sslmode=disable")
	// Use connStr for tests...
}
```

### Mocking
Use testify/mock for interfaces:

```go
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) CreateRun(ctx context.Context, spec executor.RunSpec) (executor.RunRef, error) {
	args := m.Called(ctx, spec)
	return args.Get(0).(executor.RunRef), args.Error(1)
}

// In test:
mockExec := new(MockExecutor)
mockExec.On("CreateRun", mock.Anything, mock.Anything).Return(executor.RunRef{}, nil)
```

---

## Known Edge Cases Documented

### CPU Parsing
- `ParseFloat` accepts "1.2.3" (parses as 1.2)
- `ParseFloat` accepts "1.." (parses as 1.0)
- Negative values not validated (allowed by Docker)

### Memory Parsing
- Decimal values like "1.5g" truncate to integer part (1)
- Invalid units fall through to raw bytes parsing
- Negative values not validated
- Double units like "512gg" parsed as bytes

**Recommendation:** Add validation layer if stricter parsing needed.

---

## Test Coverage Goals

| Phase | Target | Packages |
|-------|--------|----------|
| **Phase 1** (Current) | 40% | Domain, Postgres, Executor basics |
| **Phase 2** (Next) | 60% | API handlers, Worker processor |
| **Phase 3** (Future) | 70%+ | Full integration, E2E |

**Current Status:** Phase 1 in progress (8.3% overall, critical foundations laid)

---

## Running Tests in CI

**Recommended GitHub Actions setup:**

```yaml
- name: Run unit tests
  run: go test ./internal/domain ./internal/api/validation_test.go -v

- name: Run integration tests
  run: |
    # Requires Docker
    go test ./internal/storage/postgres -v -timeout=5m
```

**Note:** Integration tests require Docker for testcontainers.

---

## Test Maintenance

### Adding New Tests
1. Create `*_test.go` file in same package
2. Use table-driven format when possible
3. Mock external dependencies
4. Use testcontainers for real database/services
5. Run `go test ./...` to verify

### Updating Tests
When changing business logic:
1. Update corresponding test expectations
2. Add new test cases for new edge cases
3. Document behavior changes in test comments
4. Verify coverage doesn't decrease

---

## Questions?

- **Why not 100% coverage?** Diminishing returns. 70% is industry standard for good quality.
- **Why integration tests?** Unit tests miss database/concurrency issues.
- **Why testcontainers?** Real Postgres catches SQL/transaction bugs mocks can't.
- **Test speed?** Unit tests: instant. Integration: ~1s per test. Worth it.

**Next steps:** Implement worker processor tests, then API handler tests.
