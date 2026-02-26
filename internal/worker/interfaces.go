package worker

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// JobStore interface for database operations
type JobStore interface {
	GetJob(ctx context.Context, id uuid.UUID) (*domain.Job, error)
	UpdateJob(ctx context.Context, job *domain.Job) error
	SaveArtefacts(ctx context.Context, jobID uuid.UUID, artefacts []domain.Artefact) error
	GetRun(ctx context.Context, id uuid.UUID) (*domain.Run, error)
	GetProject(ctx context.Context, id uuid.UUID) (*domain.Project, error)
	GetWorkspace(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	RecomputeRunStatus(ctx context.Context, id uuid.UUID) error
	GetRegistrySecret(ctx context.Context, id uuid.UUID) (*domain.RegistrySecret, error)
}

// ObjectStorage interface for S3/object storage operations
type ObjectStorage interface {
	Upload(ctx context.Context, key string, r io.Reader, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}

// ObjectInfo holds metadata about an object in storage
type ObjectInfo struct {
	Key          string
	SizeBytes    int64
	ContentType  string
	LastModified time.Time
}
