package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

func (s *Store) RecordUsageLedgerAndStripeEvents(
	ctx context.Context,
	usage domain.JobUsageEvent,
	ledger domain.JobLedgerEntry,
	stripeEvents []domain.StripeMeterEvent,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var usageID uuid.UUID
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_events (
			id, workspace_id, project_id, run_id, job_id, container_id,
			started_at, finished_at, duration_seconds, cpu_seconds, memory_gb_seconds, gpu_seconds,
			max_memory_bytes, sample_interval_seconds, source
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, 'docker_stats'
		)
		ON CONFLICT (job_id) DO UPDATE
		SET finished_at = EXCLUDED.finished_at
		RETURNING id
	`,
		usage.ID,
		usage.WorkspaceID,
		usage.ProjectID,
		usage.RunID,
		usage.JobID,
		usage.ContainerID,
		usage.StartedAt,
		usage.FinishedAt,
		usage.DurationSeconds,
		usage.CPUSeconds,
		usage.MemoryGBSeconds,
		usage.GPUSeconds,
		usage.MaxMemoryBytes,
		usage.SampleIntervalSec,
	).Scan(&usageID); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO billing_ledger_entries (
			id, usage_event_id, workspace_id, project_id, run_id, job_id, month_key,
			cpu_seconds, memory_gb_seconds, gpu_seconds, pricing_version, currency,
			cpu_unit_price_minor, memory_unit_price_minor, gpu_unit_price_minor,
			stripe_cpu_price_id, stripe_memory_price_id, stripe_gpu_price_id,
			estimated_cpu_minor, estimated_memory_minor, estimated_gpu_minor, estimated_total_minor,
			estimated_cpu_minor_exact, estimated_memory_minor_exact, estimated_gpu_minor_exact, estimated_total_minor_exact
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20, $21, $22,
			$23, $24, $25, $26
		)
		ON CONFLICT (usage_event_id) DO NOTHING
	`,
		ledger.ID,
		usageID,
		ledger.WorkspaceID,
		ledger.ProjectID,
		ledger.RunID,
		ledger.JobID,
		ledger.MonthKey,
		ledger.CPUSeconds,
		ledger.MemoryGBSeconds,
		ledger.GPUSeconds,
		ledger.Pricing.Version,
		ledger.Pricing.Currency,
		ledger.Pricing.CPUUnitPriceMinor,
		ledger.Pricing.MemoryUnitPriceMinor,
		ledger.Pricing.GPUUnitPriceMinor,
		nullIfEmpty(ledger.Pricing.StripeCPUPriceID),
		nullIfEmpty(ledger.Pricing.StripeMemoryGBPriceID),
		nullIfEmpty(ledger.Pricing.StripeGPUPriceID),
		ledger.EstimatedCPUMinor,
		ledger.EstimatedMemoryMinor,
		ledger.EstimatedGPUMinor,
		ledger.EstimatedTotalMinor,
		ledger.EstimatedCPUMinorExact,
		ledger.EstimatedMemoryMinorExact,
		ledger.EstimatedGPUMinorExact,
		ledger.EstimatedTotalMinorExact,
	)
	if err != nil {
		return err
	}

	for _, evt := range stripeEvents {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO stripe_meter_events (
				id, workspace_id, project_id, run_id, job_id, usage_event_id,
				meter_name, meter_value, event_timestamp, idempotency_key,
				status, attempt_count, next_retry_at
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13
			)
			ON CONFLICT (idempotency_key) DO NOTHING
		`,
			evt.ID,
			evt.WorkspaceID,
			evt.ProjectID,
			evt.RunID,
			evt.JobID,
			usageID,
			evt.MeterName,
			evt.MeterValue,
			evt.EventTimestamp,
			evt.IdempotencyKey,
			evt.Status,
			evt.AttemptCount,
			evt.NextRetryAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetWorkspaceBillingAccount(ctx context.Context, workspaceID uuid.UUID) (*domain.WorkspaceBillingAccount, error) {
	var acct domain.WorkspaceBillingAccount
	var subscriptionID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT workspace_id, stripe_customer_id, stripe_subscription_id, invoices_enabled
		FROM workspace_billing_accounts
		WHERE workspace_id = $1
	`, workspaceID).Scan(&acct.WorkspaceID, &acct.StripeCustomerID, &subscriptionID, &acct.InvoicesEnabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if subscriptionID.Valid {
		v := subscriptionID.String
		acct.StripeSubscriptionID = &v
	}
	return &acct, nil
}

func (s *Store) ClaimStripeMeterEventsForRetry(ctx context.Context, batchSize int, lease time.Duration) ([]domain.StripeMeterEvent, error) {
	if batchSize <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		UPDATE stripe_meter_events e
		SET next_retry_at = NOW() + ($2::text)::interval
		WHERE e.id IN (
			SELECT id
			FROM stripe_meter_events
			WHERE status IN ('pending', 'failed', 'blocked')
			  AND next_retry_at <= NOW()
			ORDER BY next_retry_at ASC, created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, workspace_id, project_id, run_id, job_id, usage_event_id, meter_name, meter_value,
		          event_timestamp, idempotency_key, status, attempt_count, next_retry_at, last_attempt_at, last_error, stripe_event_id
	`, batchSize, fmt.Sprintf("%f seconds", lease.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]domain.StripeMeterEvent, 0, batchSize)
	for rows.Next() {
		var evt domain.StripeMeterEvent
		var lastAttemptAt sql.NullTime
		var lastError sql.NullString
		var stripeEventID sql.NullString
		if err := rows.Scan(
			&evt.ID,
			&evt.WorkspaceID,
			&evt.ProjectID,
			&evt.RunID,
			&evt.JobID,
			&evt.UsageEventID,
			&evt.MeterName,
			&evt.MeterValue,
			&evt.EventTimestamp,
			&evt.IdempotencyKey,
			&evt.Status,
			&evt.AttemptCount,
			&evt.NextRetryAt,
			&lastAttemptAt,
			&lastError,
			&stripeEventID,
		); err != nil {
			return nil, err
		}
		if lastAttemptAt.Valid {
			v := lastAttemptAt.Time
			evt.LastAttemptAt = &v
		}
		if lastError.Valid {
			v := lastError.String
			evt.LastError = &v
		}
		if stripeEventID.Valid {
			v := stripeEventID.String
			evt.StripeEventID = &v
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}

func (s *Store) MarkStripeMeterEventSent(ctx context.Context, eventID uuid.UUID, stripeEventID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE stripe_meter_events
		SET status = 'sent',
		    stripe_event_id = $2,
		    last_error = NULL,
		    attempt_count = attempt_count + 1,
		    last_attempt_at = NOW(),
		    next_retry_at = NOW()
		WHERE id = $1
	`, eventID, stripeEventID)
	return err
}

func (s *Store) MarkStripeMeterEventFailed(ctx context.Context, eventID uuid.UUID, blocked bool, lastErr string, nextRetryAt time.Time) error {
	status := domain.StripeMeterEventStatusFailed
	if blocked {
		status = domain.StripeMeterEventStatusBlocked
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE stripe_meter_events
		SET status = $2,
		    last_error = $3,
		    attempt_count = attempt_count + 1,
		    last_attempt_at = NOW(),
		    next_retry_at = $4
		WHERE id = $1
	`, eventID, status, lastErr, nextRetryAt)
	return err
}

func (s *Store) GetWorkspaceMonthToDateBilling(ctx context.Context, workspaceID uuid.UUID, monthKey string) (*domain.WorkspaceMonthToDateBilling, error) {
	var out domain.WorkspaceMonthToDateBilling
	out.WorkspaceID = workspaceID
	out.MonthKey = monthKey
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(cpu_seconds), 0),
			COALESCE(SUM(memory_gb_seconds), 0),
			COALESCE(SUM(gpu_seconds), 0),
			COALESCE(SUM(estimated_total_minor), 0),
			COALESCE(SUM(estimated_total_minor_exact), 0),
			COALESCE(MAX(currency), 'USD')
		FROM billing_ledger_entries
		WHERE workspace_id = $1
		  AND month_key = $2
	`, workspaceID, monthKey).Scan(&out.CPUSeconds, &out.MemoryGBSeconds, &out.GPUSeconds, &out.EstimatedTotalMinor, &out.EstimatedTotalMinorExact, &out.Currency)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) GetRunBillingBreakdown(ctx context.Context, workspaceID, projectID, runID uuid.UUID) (*domain.RunBillingBreakdown, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			job_id,
			cpu_seconds,
			memory_gb_seconds,
			gpu_seconds,
			estimated_cpu_minor,
			estimated_memory_minor,
			estimated_gpu_minor,
			estimated_total_minor,
			estimated_cpu_minor_exact,
			estimated_memory_minor_exact,
			estimated_gpu_minor_exact,
			estimated_total_minor_exact,
			pricing_version,
			currency,
			created_at
		FROM billing_ledger_entries
		WHERE workspace_id = $1
		  AND project_id = $2
		  AND run_id = $3
		ORDER BY created_at ASC
	`, workspaceID, projectID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &domain.RunBillingBreakdown{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		RunID:       runID,
		Items:       []domain.RunBillingLineItem{},
	}
	for rows.Next() {
		var item domain.RunBillingLineItem
		if err := rows.Scan(
			&item.JobID,
			&item.CPUSeconds,
			&item.MemoryGBSeconds,
			&item.GPUSeconds,
			&item.EstimatedCPUMinor,
			&item.EstimatedMemoryMinor,
			&item.EstimatedGPUMinor,
			&item.EstimatedTotalMinor,
			&item.EstimatedCPUMinorExact,
			&item.EstimatedMemoryMinorExact,
			&item.EstimatedGPUMinorExact,
			&item.EstimatedTotalMinorExact,
			&item.PricingVersion,
			&item.Currency,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		out.CPUSeconds += item.CPUSeconds
		out.MemoryGBSeconds += item.MemoryGBSeconds
		out.GPUSeconds += item.GPUSeconds
		out.EstimatedTotalMinor += item.EstimatedTotalMinor
		out.EstimatedTotalMinorExact += item.EstimatedTotalMinorExact
		if out.Currency == "" {
			out.Currency = item.Currency
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
