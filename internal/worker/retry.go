package worker

import (
	"math"
	"math/rand"
	"time"
)

// retryDelay returns an exponential backoff with jitter.
func retryDelay(attempt int, base, max time.Duration, jitter float64) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if delay > max {
		delay = max
	}

	if jitter <= 0 {
		return delay
	}

	// Apply jitter: +/- jitter fraction
	delta := float64(delay) * jitter
	offset := (rand.Float64()*2 - 1) * delta
	jittered := time.Duration(float64(delay) + offset)
	if jittered < 0 {
		return 0
	}
	return jittered
}
