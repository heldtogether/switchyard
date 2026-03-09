package billing

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
)

// StripeMeterEmitter emits usage quantities to Stripe billing meters.
type StripeMeterEmitter interface {
	Emit(ctx context.Context, customerID string, meterName string, value float64, ts time.Time, idempotencyKey string) (stripeEventID string, err error)
}

type StripeSDKMeterEmitter struct {
	apiKey  string
	backend stripe.Backend
}

func NewStripeSDKMeterEmitter(apiKey string) *StripeSDKMeterEmitter {
	return &StripeSDKMeterEmitter{
		apiKey:  apiKey,
		backend: stripe.GetBackend(stripe.APIBackend),
	}
}

func (s *StripeSDKMeterEmitter) Emit(ctx context.Context, customerID string, meterName string, value float64, ts time.Time, idempotencyKey string) (string, error) {
	if customerID == "" {
		return "", fmt.Errorf("stripe customer id is required")
	}
	if meterName == "" {
		return "", fmt.Errorf("meter name is required")
	}
	if idempotencyKey == "" {
		return "", fmt.Errorf("idempotency key is required")
	}

	params := &stripe.Params{Context: ctx}
	params.SetIdempotencyKey(idempotencyKey)
	params.AddExtra("event_name", meterName)
	params.AddExtra("timestamp", strconv.FormatInt(ts.UTC().Unix(), 10))
	params.AddExtra("payload[stripe_customer_id]", customerID)
	params.AddExtra("payload[value]", strconv.FormatFloat(value, 'f', 9, 64))

	var resp struct {
		stripe.APIResource
		ID string `json:"id"`
	}
	if err := s.backend.Call(http.MethodPost, "/v1/billing/meter_events", s.apiKey, params, &resp); err != nil {
		return "", err
	}
	if resp.ID == "" {
		return "", fmt.Errorf("stripe meter event id missing in response")
	}
	return resp.ID, nil
}

func BuildCPUMeterIdempotencyKey(workspaceID, runID, jobID uuid.UUID) string {
	return fmt.Sprintf("org_%s_run_%s_job_%s_meter_cpu_seconds", workspaceID, runID, jobID)
}

func BuildMemoryMeterIdempotencyKey(workspaceID, runID, jobID uuid.UUID) string {
	return fmt.Sprintf("org_%s_run_%s_job_%s_meter_memory_gb_seconds", workspaceID, runID, jobID)
}

func BuildGPUMeterIdempotencyKey(workspaceID, runID, jobID uuid.UUID) string {
	return fmt.Sprintf("org_%s_run_%s_job_%s_meter_gpu_seconds", workspaceID, runID, jobID)
}

type MockStripeMeterEmitter struct {
	EmitFn func(ctx context.Context, customerID string, meterName string, value float64, ts time.Time, idempotencyKey string) (string, error)
}

func (m *MockStripeMeterEmitter) Emit(ctx context.Context, customerID string, meterName string, value float64, ts time.Time, idempotencyKey string) (string, error) {
	if m.EmitFn == nil {
		return "evt_mock", nil
	}
	return m.EmitFn(ctx, customerID, meterName, value, ts, idempotencyKey)
}
