package executor

import (
	"context"
	"io"
	"time"

	"github.com/heldtogether/switchyard/internal/domain"
)

// Executor abstracts job execution environments
type Executor interface {
	// CreateRun starts job execution and returns a reference
	CreateRun(ctx context.Context, spec RunSpec) (RunRef, error)

	// Wait blocks until job completes or context cancelled
	Wait(ctx context.Context, ref RunRef) (Result, error)

	// GetLogs streams logs to writer
	GetLogs(ctx context.Context, ref RunRef, w io.Writer) error

	// CollectOutputs finds and uploads outputs to object store
	CollectOutputs(ctx context.Context, ref RunRef, spec OutputSpec) ([]domain.Artefact, error)

	// Cancel stops running job
	Cancel(ctx context.Context, ref RunRef) error

	// Cleanup removes all resources
	Cleanup(ctx context.Context, ref RunRef) error

	// Status checks current state (for recovery)
	Status(ctx context.Context, ref RunRef) (ExecutorStatus, error)
}

// RunSpec defines a job execution request
type RunSpec struct {
	JobID        string
	Image        string
	ImageDigest  string
	Command      []string
	Env          map[string]string
	Outputs      []string
	CPU          string
	Memory       string
	GPUCount     int
	Timeout      time.Duration
	RegistryAuth *domain.RegistryAuth

	// Additional context for system environment variables
	CreatedAt         time.Time
	ArtefactPrefix    string
	Bucket            string
	APIBaseURL        string
	SwitchyardVersion string
	ExecutorType      string
	NodeID            string
}

// Output Spec defines output collection
type OutputSpec struct {
	Paths       []string
	ObjectStore ObjectStore
	KeyPrefix   string
}

// RunRef is an executor-specific handle to a running job
type RunRef struct {
	ExecutorType string
	Reference    string // Service ID, Pod name, etc.
}

// Result is the execution outcome
type Result struct {
	Status     ExecutorStatus
	ExitCode   int
	Error      error
	StartedAt  time.Time
	FinishedAt time.Time
}

// ExecutorStatus represents runtime status
type ExecutorStatus string

const (
	StatusPending   ExecutorStatus = "PENDING"
	StatusRunning   ExecutorStatus = "RUNNING"
	StatusSuccess   ExecutorStatus = "SUCCESS"
	StatusFailed    ExecutorStatus = "FAILED"
	StatusCancelled ExecutorStatus = "CANCELLED"
	StatusTimeout   ExecutorStatus = "TIMEOUT"
	StatusUnknown   ExecutorStatus = "UNKNOWN"
)

// ObjectStore interface for uploading artefacts
type ObjectStore interface {
	Upload(ctx context.Context, key string, r io.Reader, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}

// ObjectInfo holds object metadata
type ObjectInfo struct {
	Key          string
	SizeBytes    int64
	ContentType  string
	LastModified time.Time
}
