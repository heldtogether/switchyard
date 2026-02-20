package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestJobStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   JobStatus
		expected bool
	}{
		{"SUCCEEDED is terminal", JobStatusSucceeded, true},
		{"FAILED is terminal", JobStatusFailed, true},
		{"CANCELLED is terminal", JobStatusCancelled, true},
		{"TIMEOUT is terminal", JobStatusTimeout, true},
		{"PENDING is not terminal", JobStatusPending, false},
		{"RUNNING is not terminal", JobStatusRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsTerminal()
			assert.Equal(t, tt.expected, result, "IsTerminal() for %s", tt.status)
		})
	}
}

func TestJob_Duration(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Minute)
	expected := 5 * time.Minute

	tests := []struct {
		name       string
		startedAt  *time.Time
		finishedAt *time.Time
		expected   *time.Duration
	}{
		{
			name:       "both timestamps present",
			startedAt:  &now,
			finishedAt: &later,
			expected:   &expected,
		},
		{
			name:       "startedAt nil",
			startedAt:  nil,
			finishedAt: &later,
			expected:   nil,
		},
		{
			name:       "finishedAt nil",
			startedAt:  &now,
			finishedAt: nil,
			expected:   nil,
		},
		{
			name:       "both nil",
			startedAt:  nil,
			finishedAt: nil,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{
				ID:         uuid.New(),
				StartedAt:  tt.startedAt,
				FinishedAt: tt.finishedAt,
			}

			result := job.Duration()

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestJob_Duration_Calculation(t *testing.T) {
	// Test actual duration calculation with different intervals
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"1 second", 1 * time.Second},
		{"30 seconds", 30 * time.Second},
		{"5 minutes", 5 * time.Minute},
		{"1 hour", 1 * time.Hour},
		{"2 hours 30 minutes", 2*time.Hour + 30*time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			end := start.Add(tt.duration)

			job := &Job{
				ID:         uuid.New(),
				StartedAt:  &start,
				FinishedAt: &end,
			}

			result := job.Duration()
			assert.NotNil(t, result)
			assert.Equal(t, tt.duration, *result)
		})
	}
}
