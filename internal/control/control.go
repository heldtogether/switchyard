package control

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CancelSignal is sent to a specific worker node to request job cancellation.
type CancelSignal struct {
	JobID       uuid.UUID `json:"job_id"`
	RunID       uuid.UUID `json:"run_id"`
	RequestedBy string    `json:"requested_by"`
	RequestedAt time.Time `json:"requested_at"`
}

func (s CancelSignal) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

func UnmarshalCancelSignal(data []byte) (CancelSignal, error) {
	var sig CancelSignal
	if err := json.Unmarshal(data, &sig); err != nil {
		return CancelSignal{}, err
	}
	if sig.JobID == uuid.Nil {
		return CancelSignal{}, fmt.Errorf("job_id is required")
	}
	if sig.RequestedAt.IsZero() {
		sig.RequestedAt = time.Now().UTC()
	}
	if sig.RequestedBy == "" {
		sig.RequestedBy = "system"
	}
	return sig, nil
}

// Publisher publishes cancel requests to a node-specific control channel.
type Publisher interface {
	PublishJobCancel(ctx context.Context, nodeID string, signal CancelSignal) error
	Close() error
}

// Subscriber consumes cancel requests for a worker node.
type Subscriber interface {
	ConsumeJobCancels(ctx context.Context, nodeID string, handler func(context.Context, CancelSignal) error) error
	Close() error
}
