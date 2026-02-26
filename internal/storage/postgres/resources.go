package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

var (
	ErrNodeNotFound    = errors.New("node not found")
	ErrJobNotFound     = errors.New("job not found")
	ErrInsufficientGPU = errors.New("insufficient gpu capacity")
)

// UpsertNode inserts or updates a node record.
func (s *Store) UpsertNode(ctx context.Context, node *domain.Node) error {
	query := `
		INSERT INTO nodes (node_id, hostname, executor, gpu_total, last_heartbeat, is_active, stale_at)
		VALUES ($1, $2, $3, $4, NOW(), true, NULL)
		ON CONFLICT (node_id) DO UPDATE
		SET hostname = EXCLUDED.hostname,
		    executor = EXCLUDED.executor,
		    gpu_total = EXCLUDED.gpu_total,
		    last_heartbeat = NOW(),
		    is_active = true,
		    stale_at = NULL,
		    updated_at = NOW()
		RETURNING created_at, updated_at, last_heartbeat, is_active, stale_at
	`

	return s.db.QueryRowContext(ctx, query,
		node.ID, node.Hostname, node.Executor, node.GPUTotal,
	).Scan(&node.CreatedAt, &node.UpdatedAt, &node.LastHeartbeat, &node.IsActive, &node.StaleAt)
}

// UpdateNodeHeartbeat updates last_heartbeat and GPU total for a node.
func (s *Store) UpdateNodeHeartbeat(ctx context.Context, nodeID string, gpuTotal int) error {
	query := `
		UPDATE nodes SET gpu_total = $1, last_heartbeat = NOW(), is_active = true, stale_at = NULL, updated_at = NOW()
		WHERE node_id = $2
	`
	res, err := s.db.ExecContext(ctx, query, gpuTotal, nodeID)
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
	if err := tx.QueryRowContext(ctx,
		`SELECT gpu_total FROM nodes WHERE node_id = $1 AND is_active = true FOR UPDATE`, nodeID,
	).Scan(&nodeTotal); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNodeNotFound
		}
		return nil, fmt.Errorf("lock node: %w", err)
	}

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
		SELECT id, job_id, node_id, gpu_count, allocated_at, released_at
		FROM gpu_allocations
		WHERE job_id = $1 AND released_at IS NULL
		FOR UPDATE
	`
	switch err := tx.QueryRowContext(ctx, existingQuery, jobID).Scan(
		&existing.ID, &existing.JobID, &existing.NodeID, &existing.GPUCount,
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
			INSERT INTO gpu_allocations (job_id, node_id, gpu_count)
			VALUES ($1, $2, $3)
			RETURNING id, allocated_at
		`
		if err := tx.QueryRowContext(ctx, insertQuery, jobID, nodeID, 0).Scan(&alloc.ID, &alloc.AllocatedAt); err != nil {
			return nil, fmt.Errorf("insert allocation (zero): %w", err)
		}
		alloc.JobID = jobID
		alloc.NodeID = nodeID
		alloc.GPUCount = 0
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
		`SELECT gpu_count FROM gpu_allocations WHERE node_id = $1 AND released_at IS NULL FOR UPDATE`, nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("sum allocations: %w", err)
	}
	defer rows.Close()

	allocated := 0
	for rows.Next() {
		var count int
		if err := rows.Scan(&count); err != nil {
			return nil, fmt.Errorf("sum allocations: %w", err)
		}
		allocated += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sum allocations: %w", err)
	}

	if allocated+jobGPU > nodeTotal {
		return nil, ErrInsufficientGPU
	}

	alloc := &domain.GPUAllocation{}
	insertQuery := `
		INSERT INTO gpu_allocations (job_id, node_id, gpu_count)
		VALUES ($1, $2, $3)
		RETURNING id, allocated_at
	`
	if err := tx.QueryRowContext(ctx, insertQuery, jobID, nodeID, jobGPU).Scan(&alloc.ID, &alloc.AllocatedAt); err != nil {
		return nil, fmt.Errorf("insert allocation: %w", err)
	}

	alloc.JobID = jobID
	alloc.NodeID = nodeID
	alloc.GPUCount = jobGPU

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
