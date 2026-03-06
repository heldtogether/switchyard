package domain

import (
	"time"

	"github.com/google/uuid"
)

// Node represents a worker node and its reported capacity
// Node ID should be stable across restarts (for example, hostname or orchestrator node ID)
type Node struct {
	ID            string       `json:"id"`
	Hostname      string       `json:"hostname"`
	Executor      ExecutorType `json:"executor"`
	GPUTotal      int          `json:"gpu_total"`
	GPUDeviceIDs  []string     `json:"gpu_device_ids,omitempty"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	IsActive      bool         `json:"is_active"`
	StaleAt       *time.Time   `json:"stale_at,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// GPUAllocation tracks per-job GPU reservations
// ReleasedAt is nil while the allocation is active.
type GPUAllocation struct {
	ID          uuid.UUID  `json:"id"`
	JobID       uuid.UUID  `json:"job_id"`
	NodeID      string     `json:"node_id"`
	GPUCount    int        `json:"gpu_count"`
	DeviceIDs   []string   `json:"device_ids,omitempty"`
	AllocatedAt time.Time  `json:"allocated_at"`
	ReleasedAt  *time.Time `json:"released_at,omitempty"`
}
