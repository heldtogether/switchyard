package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// JobCancellationEvent records an immutable cancellation lifecycle event.
type JobCancellationEvent struct {
	JobID       uuid.UUID
	EventType   string
	RequestedBy string
	NodeID      *string
	ExecutorRef *string
	Message     *string
}

func (s *Store) CreateJobCancellationEvent(ctx context.Context, event JobCancellationEvent) error {
	eventType := strings.TrimSpace(event.EventType)
	if eventType == "" {
		return fmt.Errorf("event_type is required")
	}
	requestedBy := strings.TrimSpace(event.RequestedBy)
	if requestedBy == "" {
		requestedBy = "system"
	}

	query := `
		INSERT INTO job_cancellation_events (
			workspace_id, project_id, run_id, job_id,
			event_type, requested_by, node_id, executor_ref, message
		)
		SELECT p.workspace_id, r.project_id, j.run_id, j.id,
			$2, $3, $4, $5, $6
		FROM jobs j
		JOIN runs r ON r.id = j.run_id
		JOIN projects p ON p.id = r.project_id
		WHERE j.id = $1
	`
	result, err := s.db.ExecContext(
		ctx,
		query,
		event.JobID,
		eventType,
		requestedBy,
		event.NodeID,
		event.ExecutorRef,
		event.Message,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("job not found")
	}
	return nil
}

// MarkJobCancelling marks a running job as cancelling and stores request metadata.
func (s *Store) MarkJobCancelling(ctx context.Context, id uuid.UUID, requestedBy, reason string, statusMessage *string) error {
	query := `
		UPDATE jobs
		SET status = $1,
			status_message = $2,
			cancel_requested_at = NOW(),
			cancel_requested_by = $3,
			cancel_reason = $4
		WHERE id = $5
		  AND status IN ('RUNNING', 'CANCELLING')
	`
	res, err := s.db.ExecContext(ctx, query, domain.JobStatusCancelling, statusMessage, requestedBy, reason, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
