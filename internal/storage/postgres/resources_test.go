package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/stretchr/testify/require"
)

func makeGPUDeviceIDs(total int) []string {
	ids := make([]string, 0, total)
	for i := 0; i < total; i++ {
		ids = append(ids, fmt.Sprintf("%d", i))
	}
	return ids
}

func TestClaimGPUAllocation_HappyPath(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	node := &domain.Node{
		ID:           "node-1",
		Hostname:     "node-1",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     2,
		GPUDeviceIDs: makeGPUDeviceIDs(2),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		GPUCount:    1,
	}
	require.NoError(t, store.CreateJob(ctx, job))

	alloc, err := store.ClaimGPUAllocation(ctx, job.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, 1, alloc.GPUCount)
	require.Equal(t, node.ID, alloc.NodeID)
	require.Equal(t, []string{"0"}, alloc.DeviceIDs)

	updated, err := store.GetJob(ctx, job.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.AssignedNodeID)
	require.Equal(t, node.ID, *updated.AssignedNodeID)
}

func TestClaimGPUAllocation_Insufficient(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	node := &domain.Node{
		ID:           "node-2",
		Hostname:     "node-2",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     1,
		GPUDeviceIDs: makeGPUDeviceIDs(1),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	jobA := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		GPUCount:    1,
	}
	require.NoError(t, store.CreateJob(ctx, jobA))
	_, err := store.ClaimGPUAllocation(ctx, jobA.ID, node.ID)
	require.NoError(t, err)

	jobB := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		GPUCount:    1,
	}
	require.NoError(t, store.CreateJob(ctx, jobB))
	_, err = store.ClaimGPUAllocation(ctx, jobB.ID, node.ID)
	require.ErrorIs(t, err, ErrInsufficientGPU)
}

func TestClaimGPUAllocation_NodeInactive(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	node := &domain.Node{
		ID:           "node-3",
		Hostname:     "node-3",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     2,
		GPUDeviceIDs: makeGPUDeviceIDs(2),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	_, err := store.db.ExecContext(ctx, `UPDATE nodes SET is_active = false WHERE node_id = $1`, node.ID)
	require.NoError(t, err)

	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		GPUCount:    1,
	}
	require.NoError(t, store.CreateJob(ctx, job))

	_, err = store.ClaimGPUAllocation(ctx, job.ID, node.ID)
	require.ErrorIs(t, err, ErrNodeNotFound)
}

func TestMarkStaleNodes(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	node := &domain.Node{
		ID:           "node-4",
		Hostname:     "node-4",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     1,
		GPUDeviceIDs: makeGPUDeviceIDs(1),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	_, err := store.db.ExecContext(ctx, `UPDATE nodes SET last_heartbeat = NOW() - interval '1 hour' WHERE node_id = $1`, node.ID)
	require.NoError(t, err)

	cutoff := time.Now().Add(-30 * time.Minute)
	count, err := store.MarkStaleNodes(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	var isActive bool
	var staleAt *time.Time
	row := store.db.QueryRowContext(ctx, `SELECT is_active, stale_at FROM nodes WHERE node_id = $1`, node.ID)
	require.NoError(t, row.Scan(&isActive, &staleAt))
	require.False(t, isActive)
	require.NotNil(t, staleAt)
}

func TestClaimGPUAllocation_Concurrent(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	node := &domain.Node{
		ID:           "node-concurrent",
		Hostname:     "node-concurrent",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     2,
		GPUDeviceIDs: makeGPUDeviceIDs(2),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	jobs := make([]*domain.Job, 3)
	for i := 0; i < 3; i++ {
		jobs[i] = &domain.Job{
			ID:          uuid.New(),
			RunID:       run.ID,
			CreatedBy:   "test-user",
			Status:      domain.JobStatusPending,
			Image:       "alpine:latest",
			Outputs:     []string{"/outputs"},
			TimeoutSecs: 3600,
			Executor:    domain.ExecutorTypeSwarm,
			GPUCount:    1,
		}
		require.NoError(t, store.CreateJob(ctx, jobs[i]))
	}

	var wg sync.WaitGroup
	wg.Add(len(jobs))

	results := make(chan error, len(jobs))
	for _, job := range jobs {
		job := job
		go func() {
			defer wg.Done()
			var err error
			for i := 0; i < 5; i++ {
				_, err = store.ClaimGPUAllocation(ctx, job.ID, node.ID)
				if err == nil || errors.Is(err, ErrInsufficientGPU) {
					break
				}
				if strings.Contains(err.Error(), "could not serialize access") {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				break
			}
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	success := 0
	insufficient := 0
	for err := range results {
		if err == nil {
			success++
			continue
		}
		if errors.Is(err, ErrInsufficientGPU) {
			insufficient++
			continue
		}
		require.NoError(t, err)
	}

	require.Equal(t, 2, success)
	require.Equal(t, 1, insufficient)
}

func TestClaimGPUAllocation_Idempotent(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	node := &domain.Node{
		ID:           "node-idempotent",
		Hostname:     "node-idempotent",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     2,
		GPUDeviceIDs: makeGPUDeviceIDs(2),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		GPUCount:    1,
	}
	require.NoError(t, store.CreateJob(ctx, job))

	alloc1, err := store.ClaimGPUAllocation(ctx, job.ID, node.ID)
	require.NoError(t, err)

	alloc2, err := store.ClaimGPUAllocation(ctx, job.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, alloc1.ID, alloc2.ID)
	require.Equal(t, alloc1.DeviceIDs, alloc2.DeviceIDs)
}

func TestUpdateNodeHeartbeat_Reactivates(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	node := &domain.Node{
		ID:           "node-reactivate",
		Hostname:     "node-reactivate",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     1,
		GPUDeviceIDs: makeGPUDeviceIDs(1),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	_, err := store.db.ExecContext(ctx, `UPDATE nodes SET is_active = false, stale_at = NOW() WHERE node_id = $1`, node.ID)
	require.NoError(t, err)

	require.NoError(t, store.UpdateNodeHeartbeat(ctx, node.ID, 1, makeGPUDeviceIDs(1)))

	var isActive bool
	var staleAt *time.Time
	row := store.db.QueryRowContext(ctx, `SELECT is_active, stale_at FROM nodes WHERE node_id = $1`, node.ID)
	require.NoError(t, row.Scan(&isActive, &staleAt))
	require.True(t, isActive)
	require.Nil(t, staleAt)
}

func TestReleaseGPUAllocation_Reenable(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, _, run := setupTestHierarchy(t, store, ctx)

	node := &domain.Node{
		ID:           "node-release",
		Hostname:     "node-release",
		Executor:     domain.ExecutorTypeSwarm,
		GPUTotal:     1,
		GPUDeviceIDs: makeGPUDeviceIDs(1),
	}
	require.NoError(t, store.UpsertNode(ctx, node))

	jobA := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		GPUCount:    1,
	}
	require.NoError(t, store.CreateJob(ctx, jobA))
	allocA, err := store.ClaimGPUAllocation(ctx, jobA.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"0"}, allocA.DeviceIDs)

	require.NoError(t, store.ReleaseGPUAllocation(ctx, jobA.ID, node.ID))

	jobB := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusPending,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 3600,
		Executor:    domain.ExecutorTypeSwarm,
		GPUCount:    1,
	}
	require.NoError(t, store.CreateJob(ctx, jobB))
	allocB, err := store.ClaimGPUAllocation(ctx, jobB.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"0"}, allocB.DeviceIDs)
}
