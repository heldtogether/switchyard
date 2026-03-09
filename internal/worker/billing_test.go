package worker

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/metrics"
	"github.com/stretchr/testify/require"
)

func TestBuildBillingRecords_EstimationAndPricingVersion(t *testing.T) {
	started := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	finished := started.Add(30 * time.Second)
	job := &domain.Job{
		ID:         uuid.New(),
		RunID:      uuid.New(),
		StartedAt:  &started,
		FinishedAt: &finished,
	}
	cfg := config.BillingConfig{
		Enabled:             true,
		InvoicesEnabled:     true,
		UsageSampleInterval: 10 * time.Second,
		Pricing: config.BillingPricing{
			Version:                 "2026-03-01",
			Currency:                "USD",
			UnitPriceCPUSecondMinor: 2,
			UnitPriceMemoryGBSMinor: 3,
			UnitPriceGPUSecondMinor: 4,
			StripeCPUPriceID:        "price_cpu",
			StripeMemoryGBSPriceID:  "price_mem",
			StripeGPUPriceID:        "price_gpu",
		},
		Stripe: config.BillingStripeConfig{
			Meters: config.BillingMeterConfig{
				CPUSeconds:      "switchyard_cpu_seconds",
				MemoryGBSeconds: "switchyard_memory_gb_seconds",
				GPUSeconds:      "switchyard_gpu_seconds",
			},
		},
	}
	job.GPUCount = 2

	summary := &metrics.UsageSummary{
		CPUSeconds:      12.5,
		MemoryGBSeconds: 8.2,
		MaxMemoryBytes:  1024,
		DurationSeconds: 30,
	}

	usage, ledger, events, err := buildBillingRecords(cfg, uuid.New(), uuid.New(), job, "container-1", summary)
	require.NoError(t, err)
	require.Equal(t, "2026-03-01", ledger.Pricing.Version)
	require.Equal(t, int64(25), ledger.EstimatedCPUMinor)
	require.Equal(t, int64(25), ledger.EstimatedMemoryMinor)
	require.Equal(t, int64(240), ledger.EstimatedGPUMinor)
	require.Equal(t, int64(290), ledger.EstimatedTotalMinor)
	require.InDelta(t, 25.0, ledger.EstimatedCPUMinorExact, 1e-9)
	require.InDelta(t, 24.6, ledger.EstimatedMemoryMinorExact, 1e-9)
	require.InDelta(t, 240.0, ledger.EstimatedGPUMinorExact, 1e-9)
	require.InDelta(t, 289.6, ledger.EstimatedTotalMinorExact, 1e-9)
	require.InDelta(t, 60.0, usage.GPUSeconds, 1e-9)
	require.Equal(t, "2026-03", ledger.MonthKey)
	require.Len(t, events, 3)
	require.Equal(t, usage.FinishedAt, events[0].EventTimestamp)
}

func TestBuildBillingRecords_InvoicesDisabled_NoStripeEvents(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	finished := time.Now().UTC()
	job := &domain.Job{
		ID:         uuid.New(),
		RunID:      uuid.New(),
		StartedAt:  &started,
		FinishedAt: &finished,
	}
	cfg := config.BillingConfig{
		Enabled:         true,
		InvoicesEnabled: false,
		Pricing: config.BillingPricing{
			Version:                 "v1",
			Currency:                "USD",
			UnitPriceCPUSecondMinor: 1,
			UnitPriceMemoryGBSMinor: 1,
			UnitPriceGPUSecondMinor: 1,
		},
	}
	summary := &metrics.UsageSummary{DurationSeconds: 10}

	_, _, events, err := buildBillingRecords(cfg, uuid.New(), uuid.New(), job, "container-1", summary)
	require.NoError(t, err)
	require.Empty(t, events)
}
