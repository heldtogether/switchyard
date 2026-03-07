package worker

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/billing"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/metrics"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
)

type billingStore interface {
	RecordUsageLedgerAndStripeEvents(ctx context.Context, usage domain.JobUsageEvent, ledger domain.JobLedgerEntry, stripeEvents []domain.StripeMeterEvent) error
}

func monthKeyUTC(t time.Time) string {
	return t.UTC().Format("2006-01")
}

func estimateAmountMinor(quantity float64, unitPriceMinor int64) int64 {
	return int64(math.Round(quantity * float64(unitPriceMinor)))
}

func estimateAmountMinorExact(quantity float64, unitPriceMinor int64) float64 {
	return quantity * float64(unitPriceMinor)
}

func buildBillingRecords(
	cfg config.BillingConfig,
	workspaceID, projectID uuid.UUID,
	job *domain.Job,
	containerID string,
	usageSummary *metrics.UsageSummary,
) (domain.JobUsageEvent, domain.JobLedgerEntry, []domain.StripeMeterEvent, error) {
	if job.StartedAt == nil || job.FinishedAt == nil {
		return domain.JobUsageEvent{}, domain.JobLedgerEntry{}, nil, fmt.Errorf("job start/finish timestamps required")
	}

	usageID := uuid.New()
	usage := domain.JobUsageEvent{
		ID:                usageID,
		WorkspaceID:       workspaceID,
		ProjectID:         projectID,
		RunID:             job.RunID,
		JobID:             job.ID,
		ContainerID:       containerID,
		StartedAt:         job.StartedAt.UTC(),
		FinishedAt:        job.FinishedAt.UTC(),
		DurationSeconds:   usageSummary.DurationSeconds,
		CPUSeconds:        usageSummary.CPUSeconds,
		MemoryGBSeconds:   usageSummary.MemoryGBSeconds,
		MaxMemoryBytes:    usageSummary.MaxMemoryBytes,
		SampleIntervalSec: int(cfg.UsageSampleInterval.Seconds()),
	}

	cpuMinor := estimateAmountMinor(usage.CPUSeconds, cfg.Pricing.UnitPriceCPUSecondMinor)
	memMinor := estimateAmountMinor(usage.MemoryGBSeconds, cfg.Pricing.UnitPriceMemoryGBSMinor)
	cpuMinorExact := estimateAmountMinorExact(usage.CPUSeconds, cfg.Pricing.UnitPriceCPUSecondMinor)
	memMinorExact := estimateAmountMinorExact(usage.MemoryGBSeconds, cfg.Pricing.UnitPriceMemoryGBSMinor)

	ledger := domain.JobLedgerEntry{
		ID:              uuid.New(),
		UsageEventID:    usageID,
		WorkspaceID:     workspaceID,
		ProjectID:       projectID,
		RunID:           job.RunID,
		JobID:           job.ID,
		MonthKey:        monthKeyUTC(usage.FinishedAt),
		CPUSeconds:      usage.CPUSeconds,
		MemoryGBSeconds: usage.MemoryGBSeconds,
		Pricing: domain.LedgerPricingSnapshot{
			Version:               cfg.Pricing.Version,
			Currency:              cfg.Pricing.Currency,
			CPUUnitPriceMinor:     cfg.Pricing.UnitPriceCPUSecondMinor,
			MemoryUnitPriceMinor:  cfg.Pricing.UnitPriceMemoryGBSMinor,
			StripeCPUPriceID:      cfg.Pricing.StripeCPUPriceID,
			StripeMemoryGBPriceID: cfg.Pricing.StripeMemoryGBSPriceID,
		},
		EstimatedCPUMinor:         cpuMinor,
		EstimatedMemoryMinor:      memMinor,
		EstimatedTotalMinor:       cpuMinor + memMinor,
		EstimatedCPUMinorExact:    cpuMinorExact,
		EstimatedMemoryMinorExact: memMinorExact,
		EstimatedTotalMinorExact:  cpuMinorExact + memMinorExact,
	}

	events := []domain.StripeMeterEvent{}
	if cfg.InvoicesEnabled {
		events = append(events, domain.StripeMeterEvent{
			ID:             uuid.New(),
			WorkspaceID:    workspaceID,
			ProjectID:      projectID,
			RunID:          job.RunID,
			JobID:          job.ID,
			UsageEventID:   usageID,
			MeterName:      cfg.Stripe.Meters.CPUSeconds,
			MeterValue:     usage.CPUSeconds,
			EventTimestamp: usage.FinishedAt,
			IdempotencyKey: billing.BuildCPUMeterIdempotencyKey(workspaceID, job.RunID, job.ID),
			Status:         domain.StripeMeterEventStatusPending,
			AttemptCount:   0,
			NextRetryAt:    time.Now().UTC(),
		})
		events = append(events, domain.StripeMeterEvent{
			ID:             uuid.New(),
			WorkspaceID:    workspaceID,
			ProjectID:      projectID,
			RunID:          job.RunID,
			JobID:          job.ID,
			UsageEventID:   usageID,
			MeterName:      cfg.Stripe.Meters.MemoryGBSeconds,
			MeterValue:     usage.MemoryGBSeconds,
			EventTimestamp: usage.FinishedAt,
			IdempotencyKey: billing.BuildMemoryMeterIdempotencyKey(workspaceID, job.RunID, job.ID),
			Status:         domain.StripeMeterEventStatusPending,
			AttemptCount:   0,
			NextRetryAt:    time.Now().UTC(),
		})
	}

	return usage, ledger, events, nil
}

func runStripeMeterRetryLoop(ctx context.Context, store *postgres.Store, emitter billing.StripeMeterEmitter, cfg config.BillingConfig) {
	if emitter == nil || store == nil {
		return
	}
	ticker := time.NewTicker(cfg.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processStripeMeterRetries(ctx, store, emitter, cfg)
		}
	}
}

func processStripeMeterRetries(ctx context.Context, store *postgres.Store, emitter billing.StripeMeterEmitter, cfg config.BillingConfig) {
	events, err := store.ClaimStripeMeterEventsForRetry(ctx, cfg.RetryBatchSize, 30*time.Second)
	if err != nil {
		return
	}
	for _, evt := range events {
		account, err := store.GetWorkspaceBillingAccount(ctx, evt.WorkspaceID)
		if err != nil {
			_ = store.MarkStripeMeterEventFailed(ctx, evt.ID, false, err.Error(), nextStripeRetryAt(evt.AttemptCount))
			continue
		}
		if account == nil || account.StripeCustomerID == "" {
			_ = store.MarkStripeMeterEventFailed(ctx, evt.ID, true, "workspace billing account not configured", nextStripeRetryAt(evt.AttemptCount))
			continue
		}
		if !account.InvoicesEnabled {
			_ = store.MarkStripeMeterEventFailed(ctx, evt.ID, true, "workspace invoices disabled", nextStripeRetryAt(evt.AttemptCount))
			continue
		}

		stripeEventID, err := emitter.Emit(ctx, account.StripeCustomerID, evt.MeterName, evt.MeterValue, evt.EventTimestamp, evt.IdempotencyKey)
		if err != nil {
			_ = store.MarkStripeMeterEventFailed(ctx, evt.ID, false, err.Error(), nextStripeRetryAt(evt.AttemptCount))
			continue
		}
		_ = store.MarkStripeMeterEventSent(ctx, evt.ID, stripeEventID)
	}
}

func nextStripeRetryAt(attemptCount int) time.Time {
	base := time.Second * 10
	delay := base * time.Duration(1<<max(0, attemptCount))
	if delay > time.Minute*30 {
		delay = time.Minute * 30
	}
	return time.Now().UTC().Add(delay)
}
