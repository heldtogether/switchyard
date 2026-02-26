package queue

import (
	"context"
	"testing"
	"time"

	"github.com/heldtogether/switchyard/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestRedisQueueDelayAndRequeue(t *testing.T) {
	redis := testutil.SetupTestRedis(t)
	defer redis.Cleanup(t)

	q, err := NewRedis(redis.URL, "test:jobs")
	require.NoError(t, err)
	defer q.Close()

	ctx := context.Background()

	jobID := "job-123"
	retryAt := time.Now().Add(-1 * time.Second)

	require.NoError(t, q.Delay(ctx, jobID, retryAt))

	count, err := q.RequeueReady(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	msg, err := q.Pop(ctx, 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Equal(t, jobID, msg.JobID())
}

func TestRedisQueueDelayNotReady(t *testing.T) {
	redis := testutil.SetupTestRedis(t)
	defer redis.Cleanup(t)

	q, err := NewRedis(redis.URL, "test:jobs")
	require.NoError(t, err)
	defer q.Close()

	ctx := context.Background()

	require.NoError(t, q.Delay(ctx, "job-456", time.Now().Add(5*time.Second)))

	count, err := q.RequeueReady(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestRedisQueueRequeueIdempotent(t *testing.T) {
	redis := testutil.SetupTestRedis(t)
	defer redis.Cleanup(t)

	q, err := NewRedis(redis.URL, "test:jobs")
	require.NoError(t, err)
	defer q.Close()

	ctx := context.Background()
	jobID := "job-dup"
	require.NoError(t, q.Delay(ctx, jobID, time.Now().Add(-1*time.Second)))
	require.NoError(t, q.Delay(ctx, jobID, time.Now().Add(-1*time.Second)))

	count, err := q.RequeueReady(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	msg, err := q.Pop(ctx, 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Equal(t, jobID, msg.JobID())

	msg2, err := q.Pop(ctx, 200*time.Millisecond)
	require.NoError(t, err)
	require.Nil(t, msg2)
}
