package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRabbitQueueDelayNoop(t *testing.T) {
	q := &RabbitQueue{}

	require.NoError(t, q.Delay(context.Background(), "job-1", time.Now()))

	count, err := q.RequeueReady(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
