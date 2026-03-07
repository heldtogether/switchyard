package domain

import (
	"time"

	"github.com/google/uuid"
)

type JobUsageEvent struct {
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	ProjectID         uuid.UUID
	RunID             uuid.UUID
	JobID             uuid.UUID
	ContainerID       string
	StartedAt         time.Time
	FinishedAt        time.Time
	DurationSeconds   float64
	CPUSeconds        float64
	MemoryGBSeconds   float64
	MaxMemoryBytes    uint64
	SampleIntervalSec int
}

type LedgerPricingSnapshot struct {
	Version               string
	Currency              string
	CPUUnitPriceMinor     int64
	MemoryUnitPriceMinor  int64
	StripeCPUPriceID      string
	StripeMemoryGBPriceID string
}

type JobLedgerEntry struct {
	ID                        uuid.UUID
	UsageEventID              uuid.UUID
	WorkspaceID               uuid.UUID
	ProjectID                 uuid.UUID
	RunID                     uuid.UUID
	JobID                     uuid.UUID
	MonthKey                  string
	CPUSeconds                float64
	MemoryGBSeconds           float64
	Pricing                   LedgerPricingSnapshot
	EstimatedCPUMinor         int64
	EstimatedMemoryMinor      int64
	EstimatedTotalMinor       int64
	EstimatedCPUMinorExact    float64
	EstimatedMemoryMinorExact float64
	EstimatedTotalMinorExact  float64
}

type StripeMeterEventStatus string

const (
	StripeMeterEventStatusPending StripeMeterEventStatus = "pending"
	StripeMeterEventStatusSent    StripeMeterEventStatus = "sent"
	StripeMeterEventStatusFailed  StripeMeterEventStatus = "failed"
	StripeMeterEventStatusBlocked StripeMeterEventStatus = "blocked"
)

type StripeMeterEvent struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	ProjectID      uuid.UUID
	RunID          uuid.UUID
	JobID          uuid.UUID
	UsageEventID   uuid.UUID
	MeterName      string
	MeterValue     float64
	EventTimestamp time.Time
	IdempotencyKey string
	Status         StripeMeterEventStatus
	AttemptCount   int
	NextRetryAt    time.Time
	LastAttemptAt  *time.Time
	LastError      *string
	StripeEventID  *string
}

type WorkspaceBillingAccount struct {
	WorkspaceID          uuid.UUID
	StripeCustomerID     string
	StripeSubscriptionID *string
	InvoicesEnabled      bool
}

type WorkspaceMonthToDateBilling struct {
	WorkspaceID              uuid.UUID
	MonthKey                 string
	CPUSeconds               float64
	MemoryGBSeconds          float64
	EstimatedTotalMinor      int64
	EstimatedTotalMinorExact float64
	Currency                 string
}

type RunBillingLineItem struct {
	JobID                     uuid.UUID
	CPUSeconds                float64
	MemoryGBSeconds           float64
	EstimatedCPUMinor         int64
	EstimatedMemoryMinor      int64
	EstimatedTotalMinor       int64
	EstimatedCPUMinorExact    float64
	EstimatedMemoryMinorExact float64
	EstimatedTotalMinorExact  float64
	PricingVersion            string
	Currency                  string
	CreatedAt                 time.Time
}

type RunBillingBreakdown struct {
	WorkspaceID              uuid.UUID
	ProjectID                uuid.UUID
	RunID                    uuid.UUID
	CPUSeconds               float64
	MemoryGBSeconds          float64
	EstimatedTotalMinor      int64
	EstimatedTotalMinorExact float64
	Currency                 string
	Items                    []RunBillingLineItem
}
