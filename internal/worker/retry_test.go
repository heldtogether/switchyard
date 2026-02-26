package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryDelayNoJitter(t *testing.T) {
	base := 1 * time.Second
	max := 10 * time.Second

	require.Equal(t, base, retryDelay(0, base, max, 0))
	require.Equal(t, 2*base, retryDelay(1, base, max, 0))
	require.Equal(t, 4*base, retryDelay(2, base, max, 0))
	require.Equal(t, max, retryDelay(5, base, max, 0))
}

func TestRetryDelayWithJitterBounds(t *testing.T) {
	base := 2 * time.Second
	max := 10 * time.Second
	jitter := 0.2

	delay := retryDelay(1, base, max, jitter)
	min := time.Duration(float64(4*time.Second) * (1 - jitter))
	maxBound := time.Duration(float64(4*time.Second) * (1 + jitter))

	require.GreaterOrEqual(t, delay, min)
	require.LessOrEqual(t, delay, maxBound)
}

func TestRetryDelayProgressesAndCaps(t *testing.T) {
	base := 1 * time.Second
	max := 5 * time.Second

	d1 := retryDelay(0, base, max, 0)
	d2 := retryDelay(1, base, max, 0)
	d3 := retryDelay(2, base, max, 0)
	d4 := retryDelay(3, base, max, 0)
	d5 := retryDelay(4, base, max, 0)

	require.Greater(t, d2, d1)
	require.Greater(t, d3, d2)
	require.Equal(t, max, d4)
	require.Equal(t, max, d5)
}
