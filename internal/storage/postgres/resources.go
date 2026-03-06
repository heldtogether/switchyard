package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/lib/pq"
)

var (
	ErrNodeNotFound    = errors.New("node not found")
	ErrJobNotFound     = errors.New("job not found")
	ErrInsufficientGPU = errors.New("insufficient gpu capacity")
)

// UpsertNode inserts or updates a node record.
func (s *Store) UpsertNode(ctx context.Context, node *domain.Node) error {
	deviceIDsJSON, err := json.Marshal(node.GPUDeviceIDs)
	if err != nil {
		return fmt.Errorf("marshal gpu_device_ids: %w", err)
	}

	query := `
		INSERT INTO nodes (node_id, hostname, executor, gpu_total, gpu_device_ids, last_heartbeat, is_active, stale_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), true, NULL)
		ON CONFLICT (node_id) DO UPDATE
		SET hostname = EXCLUDED.hostname,
		    executor = EXCLUDED.executor,
		    gpu_total = EXCLUDED.gpu_total,
		    gpu_device_ids = EXCLUDED.gpu_device_ids,
		    last_heartbeat = NOW(),
		    is_active = true,
		    stale_at = NULL,
		    updated_at = NOW()
		RETURNING created_at, updated_at, last_heartbeat, is_active, stale_at
	`

	return s.db.QueryRowContext(ctx, query,
		node.ID, node.Hostname, node.Executor, node.GPUTotal, deviceIDsJSON,
	).Scan(&node.CreatedAt, &node.UpdatedAt, &node.LastHeartbeat, &node.IsActive, &node.StaleAt)
}

// UpdateNodeHeartbeat updates last_heartbeat and GPU total for a node.
func (s *Store) UpdateNodeHeartbeat(ctx context.Context, nodeID string, gpuTotal int, gpuDeviceIDs []string) error {
	deviceIDsJSON, err := json.Marshal(gpuDeviceIDs)
	if err != nil {
		return fmt.Errorf("marshal gpu_device_ids: %w", err)
	}

	query := `
		UPDATE nodes
		SET gpu_total = $1, gpu_device_ids = $2, last_heartbeat = NOW(), is_active = true, stale_at = NULL, updated_at = NOW()
		WHERE node_id = $3
	`
	res, err := s.db.ExecContext(ctx, query, gpuTotal, deviceIDsJSON, nodeID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// MaxGPUPerNode returns the maximum gpu_total across nodes.
func (s *Store) MaxGPUPerNode(ctx context.Context) (int, error) {
	query := `SELECT COALESCE(MAX(gpu_total), 0) FROM nodes WHERE is_active = true`
	var max int
	if err := s.db.QueryRowContext(ctx, query).Scan(&max); err != nil {
		return 0, err
	}
	return max, nil
}

// MarkStaleNodes marks nodes inactive if they have missed heartbeats.
func (s *Store) MarkStaleNodes(ctx context.Context, cutoff time.Time) (int64, error) {
	query := `
		UPDATE nodes
		SET is_active = false, stale_at = NOW(), updated_at = NOW()
		WHERE is_active = true AND last_heartbeat < $1
	`
	res, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ClaimGPUAllocation reserves GPUs for a job on a specific node.
func (s *Store) ClaimGPUAllocation(ctx context.Context, jobID uuid.UUID, nodeID string) (*domain.GPUAllocation, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock node row
	var nodeTotal int
	var nodeDeviceIDsJSON []byte
	if err := tx.QueryRowContext(ctx,
		`SELECT gpu_total, gpu_device_ids FROM nodes WHERE node_id = $1 AND is_active = true FOR UPDATE`, nodeID,
	).Scan(&nodeTotal, &nodeDeviceIDsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNodeNotFound
		}
		return nil, fmt.Errorf("lock node: %w", err)
	}
	nodeDeviceIDs := make([]string, 0, nodeTotal)
	if len(nodeDeviceIDsJSON) > 0 {
		if err := json.Unmarshal(nodeDeviceIDsJSON, &nodeDeviceIDs); err != nil {
			return nil, fmt.Errorf("decode node gpu_device_ids: %w", err)
		}
	}
	sort.Strings(nodeDeviceIDs)

	// Lock job row and read gpu_count
	var jobGPU int
	if err := tx.QueryRowContext(ctx,
		`SELECT gpu_count FROM jobs WHERE id = $1 FOR UPDATE`, jobID,
	).Scan(&jobGPU); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("lock job: %w", err)
	}

	// Check for existing allocation
	existing := &domain.GPUAllocation{}
	existingQuery := `
		SELECT id, job_id, node_id, gpu_count, device_ids, allocated_at, released_at
		FROM gpu_allocations
		WHERE job_id = $1 AND released_at IS NULL
		FOR UPDATE
	`
	switch err := tx.QueryRowContext(ctx, existingQuery, jobID).Scan(
		&existing.ID, &existing.JobID, &existing.NodeID, &existing.GPUCount, pq.Array(&existing.DeviceIDs),
		&existing.AllocatedAt, &existing.ReleasedAt,
	); err {
	case nil:
		if existing.NodeID != nodeID {
			return nil, fmt.Errorf("job already allocated on node %s", existing.NodeID)
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	case sql.ErrNoRows:
		// continue
	default:
		return nil, fmt.Errorf("check existing allocation: %w", err)
	}

	if jobGPU == 0 {
		// No allocation needed; create a zero allocation record for visibility
		alloc := &domain.GPUAllocation{}
		insertQuery := `
			INSERT INTO gpu_allocations (job_id, node_id, gpu_count, device_ids)
			VALUES ($1, $2, $3, $4)
			RETURNING id, allocated_at
		`
		if err := tx.QueryRowContext(ctx, insertQuery, jobID, nodeID, 0, pq.Array([]string{})).Scan(&alloc.ID, &alloc.AllocatedAt); err != nil {
			return nil, fmt.Errorf("insert allocation (zero): %w", err)
		}
		alloc.JobID = jobID
		alloc.NodeID = nodeID
		alloc.GPUCount = 0
		alloc.DeviceIDs = []string{}
		if _, err := tx.ExecContext(ctx,
			`UPDATE jobs SET assigned_node_id = COALESCE(assigned_node_id, $1) WHERE id = $2`, nodeID, jobID,
		); err != nil {
			return nil, fmt.Errorf("assign node (zero): %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return alloc, nil
	}

	// Lock and sum allocations for node (row-level locks, no aggregate FOR UPDATE)
	rows, err := tx.QueryContext(ctx,
		`SELECT gpu_count, device_ids FROM gpu_allocations WHERE node_id = $1 AND released_at IS NULL FOR UPDATE`, nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("sum allocations: %w", err)
	}
	defer rows.Close()

	allocated := 0
	usedIDs := make(map[string]struct{})
	for rows.Next() {
		var count int
		var deviceIDs []string
		if err := rows.Scan(&count, pq.Array(&deviceIDs)); err != nil {
			return nil, fmt.Errorf("sum allocations: %w", err)
		}
		allocated += count
		for _, id := range deviceIDs {
			usedIDs[id] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sum allocations: %w", err)
	}

	if allocated+jobGPU > nodeTotal {
		return nil, ErrInsufficientGPU
	}
	if len(nodeDeviceIDs) < nodeTotal {
		return nil, fmt.Errorf("node gpu inventory missing: node=%s expected=%d got=%d", nodeID, nodeTotal, len(nodeDeviceIDs))
	}

	selectedDeviceIDs := make([]string, 0, jobGPU)
	for _, id := range nodeDeviceIDs {
		if _, inUse := usedIDs[id]; inUse {
			continue
		}
		selectedDeviceIDs = append(selectedDeviceIDs, id)
		if len(selectedDeviceIDs) == jobGPU {
			break
		}
	}
	if len(selectedDeviceIDs) < jobGPU {
		return nil, ErrInsufficientGPU
	}

	alloc := &domain.GPUAllocation{}
	insertQuery := `
		INSERT INTO gpu_allocations (job_id, node_id, gpu_count, device_ids)
		VALUES ($1, $2, $3, $4)
		RETURNING id, allocated_at
	`
	if err := tx.QueryRowContext(ctx, insertQuery, jobID, nodeID, jobGPU, pq.Array(selectedDeviceIDs)).Scan(&alloc.ID, &alloc.AllocatedAt); err != nil {
		return nil, fmt.Errorf("insert allocation: %w", err)
	}

	alloc.JobID = jobID
	alloc.NodeID = nodeID
	alloc.GPUCount = jobGPU
	alloc.DeviceIDs = selectedDeviceIDs

	if _, err := tx.ExecContext(ctx,
		`UPDATE jobs SET assigned_node_id = COALESCE(assigned_node_id, $1) WHERE id = $2`, nodeID, jobID,
	); err != nil {
		return nil, fmt.Errorf("assign node: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return alloc, nil
}

// ReleaseGPUAllocation releases an active allocation.
func (s *Store) ReleaseGPUAllocation(ctx context.Context, jobID uuid.UUID, nodeID string) error {
	query := `
		UPDATE gpu_allocations
		SET released_at = NOW()
		WHERE job_id = $1 AND node_id = $2 AND released_at IS NULL
	`
	res, err := s.db.ExecContext(ctx, query, jobID, nodeID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrJobNotFound
	}
	return nil
}
