package worker

import (
	"context"
	"io"
	"time"

	"github.com/heldtogether/switchyard/internal/executor"
)

// S3Adapter adapts ObjectStorage to executor.ObjectStore interface
type S3Adapter struct {
	store ObjectStorage
}

func NewS3Adapter(store ObjectStorage) *S3Adapter {
	return &S3Adapter{store: store}
}

func (a *S3Adapter) Upload(ctx context.Context, key string, r io.Reader, contentType string) error {
	return a.store.Upload(ctx, key, r, contentType)
}

func (a *S3Adapter) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return a.store.Download(ctx, key)
}

func (a *S3Adapter) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return a.store.PresignedURL(ctx, key, expiry)
}

func (a *S3Adapter) List(ctx context.Context, prefix string) ([]executor.ObjectInfo, error) {
	objects, err := a.store.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	result := make([]executor.ObjectInfo, len(objects))
	for i, obj := range objects {
		result[i] = executor.ObjectInfo{
			Key:          obj.Key,
			SizeBytes:    obj.SizeBytes,
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified,
		}
	}
	return result, nil
}
