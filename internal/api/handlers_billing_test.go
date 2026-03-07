package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestHandleWorkspaceMonthToDateBilling(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	workspace, err := store.GetWorkspaceBySlug(ctx, "default")
	require.NoError(t, err)

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "billing-project",
		Name:        "Billing Project",
		CreatedBy:   "test-user",
	}
	require.NoError(t, store.CreateProject(ctx, project))

	run := &domain.Run{
		ID:        uuid.New(),
		ProjectID: project.ID,
		Slug:      "billing-run",
		Name:      "Billing Run",
		Status:    domain.RunStatusSucceeded,
		CreatedBy: "test-user",
	}
	require.NoError(t, store.CreateRun(ctx, run))

	started := time.Now().UTC().Add(-2 * time.Minute)
	finished := time.Now().UTC().Add(-1 * time.Minute)
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusSucceeded,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 30,
		Executor:    domain.ExecutorTypeDocker,
		StartedAt:   &started,
		FinishedAt:  &finished,
	}
	require.NoError(t, store.CreateJob(ctx, job))
	require.NoError(t, store.UpdateJob(ctx, job))

	usage := domain.JobUsageEvent{
		ID:                uuid.New(),
		WorkspaceID:       workspace.ID,
		ProjectID:         project.ID,
		RunID:             run.ID,
		JobID:             job.ID,
		ContainerID:       "container-1",
		StartedAt:         started,
		FinishedAt:        finished,
		DurationSeconds:   60,
		CPUSeconds:        12.5,
		MemoryGBSeconds:   8.5,
		MaxMemoryBytes:    1024,
		SampleIntervalSec: 10,
	}
	ledger := domain.JobLedgerEntry{
		ID:              uuid.New(),
		UsageEventID:    usage.ID,
		WorkspaceID:     workspace.ID,
		ProjectID:       project.ID,
		RunID:           run.ID,
		JobID:           job.ID,
		MonthKey:        time.Now().UTC().Format("2006-01"),
		CPUSeconds:      usage.CPUSeconds,
		MemoryGBSeconds: usage.MemoryGBSeconds,
		Pricing: domain.LedgerPricingSnapshot{
			Version:              "2026-03-01",
			Currency:             "USD",
			CPUUnitPriceMinor:    1,
			MemoryUnitPriceMinor: 1,
		},
		EstimatedCPUMinor:         13,
		EstimatedMemoryMinor:      9,
		EstimatedTotalMinor:       22,
		EstimatedCPUMinorExact:    12.5,
		EstimatedMemoryMinorExact: 8.5,
		EstimatedTotalMinorExact:  21.0,
	}
	require.NoError(t, store.RecordUsageLedgerAndStripeEvents(ctx, usage, ledger, nil))

	api := &API{
		cfg:    &config.Config{},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/billing/month-to-date", api.HandleWorkspaceMonthToDateBilling)

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/default/billing/month-to-date", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp WorkspaceMonthToDateBillingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, int64(22), resp.EstimatedTotalMinor)
	require.InDelta(t, 21.0, resp.EstimatedTotalMinorExact, 1e-9)
	require.Equal(t, "USD", resp.Currency)
}

func TestHandleRunBillingBreakdown(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	workspace, err := store.GetWorkspaceBySlug(ctx, "default")
	require.NoError(t, err)

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "billing-project",
		Name:        "Billing Project",
		CreatedBy:   "test-user",
	}
	require.NoError(t, store.CreateProject(ctx, project))

	run := &domain.Run{
		ID:        uuid.New(),
		ProjectID: project.ID,
		Slug:      "billing-run",
		Name:      "Billing Run",
		Status:    domain.RunStatusSucceeded,
		CreatedBy: "test-user",
	}
	require.NoError(t, store.CreateRun(ctx, run))

	started := time.Now().UTC().Add(-2 * time.Minute)
	finished := time.Now().UTC().Add(-1 * time.Minute)
	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusSucceeded,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 30,
		Executor:    domain.ExecutorTypeDocker,
		StartedAt:   &started,
		FinishedAt:  &finished,
	}
	require.NoError(t, store.CreateJob(ctx, job))
	require.NoError(t, store.UpdateJob(ctx, job))

	usage := domain.JobUsageEvent{
		ID:                uuid.New(),
		WorkspaceID:       workspace.ID,
		ProjectID:         project.ID,
		RunID:             run.ID,
		JobID:             job.ID,
		ContainerID:       "container-1",
		StartedAt:         started,
		FinishedAt:        finished,
		DurationSeconds:   60,
		CPUSeconds:        5,
		MemoryGBSeconds:   10,
		MaxMemoryBytes:    1024,
		SampleIntervalSec: 10,
	}
	ledger := domain.JobLedgerEntry{
		ID:              uuid.New(),
		UsageEventID:    usage.ID,
		WorkspaceID:     workspace.ID,
		ProjectID:       project.ID,
		RunID:           run.ID,
		JobID:           job.ID,
		MonthKey:        time.Now().UTC().Format("2006-01"),
		CPUSeconds:      usage.CPUSeconds,
		MemoryGBSeconds: usage.MemoryGBSeconds,
		Pricing: domain.LedgerPricingSnapshot{
			Version:              "2026-03-01",
			Currency:             "USD",
			CPUUnitPriceMinor:    1,
			MemoryUnitPriceMinor: 2,
		},
		EstimatedCPUMinor:         5,
		EstimatedMemoryMinor:      20,
		EstimatedTotalMinor:       25,
		EstimatedCPUMinorExact:    5.0,
		EstimatedMemoryMinorExact: 20.0,
		EstimatedTotalMinorExact:  25.0,
	}
	require.NoError(t, store.RecordUsageLedgerAndStripeEvents(ctx, usage, ledger, nil))

	api := &API{
		cfg:    &config.Config{},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/billing", api.HandleRunBillingBreakdown)

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/default/projects/billing-project/runs/billing-run/billing", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp RunBillingBreakdownResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, int64(25), resp.EstimatedTotalMinor)
	require.InDelta(t, 25.0, resp.EstimatedTotalMinorExact, 1e-9)
	require.Len(t, resp.Items, 1)
	require.Equal(t, job.ID, resp.Items[0].JobID)
	require.InDelta(t, 25.0, resp.Items[0].EstimatedTotalMinorExact, 1e-9)
}
