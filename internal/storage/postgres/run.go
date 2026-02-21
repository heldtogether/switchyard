package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// CreateRun inserts a new run
func (s *Store) CreateRun(ctx context.Context, run *domain.Run) error {
	query := `
		INSERT INTO runs (id, project_id, slug, name, description, status, created_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	metadataJSON, _ := json.Marshal(run.Metadata)

	err := s.db.QueryRowContext(ctx, query,
		run.ID, run.ProjectID, run.Slug, run.Name, run.Description,
		run.Status, run.CreatedBy, metadataJSON,
	).Scan(&run.CreatedAt, &run.UpdatedAt)

	return err
}

// GetRun retrieves a run by ID
func (s *Store) GetRun(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	query := `
		SELECT id, project_id, slug, name, description, status, created_at, updated_at,
		       started_at, finished_at, created_by, metadata
		FROM runs WHERE id = $1
	`

	run := &domain.Run{}
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&run.ID, &run.ProjectID, &run.Slug, &run.Name, &run.Description,
		&run.Status, &run.CreatedAt, &run.UpdatedAt, &run.StartedAt,
		&run.FinishedAt, &run.CreatedBy, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run not found")
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metadataJSON, &run.Metadata)
	return run, nil
}

// GetRunBySlug retrieves a run by project ID and slug
func (s *Store) GetRunBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*domain.Run, error) {
	query := `
		SELECT id, project_id, slug, name, description, status, created_at, updated_at,
		       started_at, finished_at, created_by, metadata
		FROM runs WHERE project_id = $1 AND slug = $2
	`

	run := &domain.Run{}
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, projectID, slug).Scan(
		&run.ID, &run.ProjectID, &run.Slug, &run.Name, &run.Description,
		&run.Status, &run.CreatedAt, &run.UpdatedAt, &run.StartedAt,
		&run.FinishedAt, &run.CreatedBy, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run not found")
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metadataJSON, &run.Metadata)
	return run, nil
}

// ListRuns lists runs for a project
func (s *Store) ListRuns(ctx context.Context, projectID uuid.UUID, status *domain.RunStatus, limit, offset int) ([]*domain.Run, error) {
	query := `
		SELECT id, project_id, slug, name, description, status, created_at, updated_at,
		       started_at, finished_at, created_by, metadata
		FROM runs
		WHERE project_id = $1
		  AND ($2::run_status IS NULL OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := s.db.QueryContext(ctx, query, projectID, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*domain.Run
	for rows.Next() {
		run := &domain.Run{}
		var metadataJSON []byte

		err := rows.Scan(
			&run.ID, &run.ProjectID, &run.Slug, &run.Name, &run.Description,
			&run.Status, &run.CreatedAt, &run.UpdatedAt, &run.StartedAt,
			&run.FinishedAt, &run.CreatedBy, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(metadataJSON, &run.Metadata)
		runs = append(runs, run)
	}

	return runs, rows.Err()
}

// UpdateRun updates a run
func (s *Store) UpdateRun(ctx context.Context, run *domain.Run) error {
	query := `
		UPDATE runs
		SET name = $1, description = $2, status = $3, started_at = $4, 
		    finished_at = $5, metadata = $6
		WHERE id = $7
	`

	metadataJSON, _ := json.Marshal(run.Metadata)

	_, err := s.db.ExecContext(ctx, query,
		run.Name, run.Description, run.Status, run.StartedAt,
		run.FinishedAt, metadataJSON, run.ID,
	)

	return err
}

// UpdateRunStatus updates a run's status
func (s *Store) UpdateRunStatus(ctx context.Context, id uuid.UUID, status domain.RunStatus) error {
	query := `UPDATE runs SET status = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, status, id)
	return err
}

// RecomputeRunStatus updates a run's status based on the current job statuses.
func (s *Store) RecomputeRunStatus(ctx context.Context, id uuid.UUID) error {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING'),
			COUNT(*) FILTER (WHERE status = 'RUNNING'),
			COUNT(*) FILTER (WHERE status = 'SUCCEEDED'),
			COUNT(*) FILTER (WHERE status = 'FAILED'),
			COUNT(*) FILTER (WHERE status = 'CANCELLED'),
			COUNT(*) FILTER (WHERE status = 'TIMEOUT'),
			COUNT(*),
			MIN(started_at),
			MAX(finished_at)
		FROM jobs
		WHERE run_id = $1
	`

	var pendingCount, runningCount, succeededCount, failedCount, cancelledCount, timeoutCount, totalCount int
	var minStartedAt sql.NullTime
	var maxFinishedAt sql.NullTime

	if err := s.db.QueryRowContext(ctx, query, id).Scan(
		&pendingCount,
		&runningCount,
		&succeededCount,
		&failedCount,
		&cancelledCount,
		&timeoutCount,
		&totalCount,
		&minStartedAt,
		&maxFinishedAt,
	); err != nil {
		return err
	}

	var newStatus domain.RunStatus
	switch {
	case totalCount == 0:
		newStatus = domain.RunStatusPending
	case runningCount > 0:
		newStatus = domain.RunStatusRunning
	case cancelledCount > 0:
		newStatus = domain.RunStatusCancelled
	case failedCount+timeoutCount > 0:
		newStatus = domain.RunStatusFailed
	case succeededCount == totalCount:
		newStatus = domain.RunStatusSucceeded
	case pendingCount == totalCount:
		newStatus = domain.RunStatusPending
	case pendingCount == 0:
		newStatus = domain.RunStatusPartial
	default:
		newStatus = domain.RunStatusRunning
	}

	var startedAt *time.Time
	if minStartedAt.Valid {
		startedAt = &minStartedAt.Time
	}

	var finishedAt *time.Time
	if newStatus.IsTerminal() && maxFinishedAt.Valid {
		finishedAt = &maxFinishedAt.Time
	}

	updateQuery := `
		UPDATE runs
		SET status = $1,
			started_at = COALESCE(started_at, $2),
			finished_at = $3
		WHERE id = $4
	`

	_, err := s.db.ExecContext(ctx, updateQuery, newStatus, startedAt, finishedAt, id)
	return err
}

// GetRunningRuns retrieves all runs in RUNNING status
func (s *Store) GetRunningRuns(ctx context.Context) ([]*domain.Run, error) {
	status := domain.RunStatusRunning
	// Get all projects (we'll need to optimize this later with a better query)
	query := `
		SELECT id, project_id, slug, name, description, status, created_at, updated_at,
		       started_at, finished_at, created_by, metadata
		FROM runs
		WHERE status = $1
		LIMIT 1000
	`

	rows, err := s.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*domain.Run
	for rows.Next() {
		run := &domain.Run{}
		var metadataJSON []byte

		err := rows.Scan(
			&run.ID, &run.ProjectID, &run.Slug, &run.Name, &run.Description,
			&run.Status, &run.CreatedAt, &run.UpdatedAt, &run.StartedAt,
			&run.FinishedAt, &run.CreatedBy, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(metadataJSON, &run.Metadata)
		runs = append(runs, run)
	}

	return runs, rows.Err()
}
