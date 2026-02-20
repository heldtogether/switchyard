package objectstore

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store implements object storage using S3-compatible backends
type S3Store struct {
	client *s3.Client
	bucket string
}

// NewS3 creates a new S3 object store
func NewS3(endpoint, accessKey, secretKey, region, bucket string, useSSL bool) (*S3Store, error) {
	// Build custom resolver for endpoint
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			HostnameImmutable: true,
			SigningRegion:     region,
		}, nil
	})

	cfg := aws.Config{
		Region:                      region,
		Credentials:                 credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		EndpointResolverWithOptions: resolver,
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // Required for MinIO and some S3-compatible stores
	})

	store := &S3Store{
		client: client,
		bucket: bucket,
	}

	return store, nil
}

// CreateBucket creates the bucket if it doesn't exist
func (s *S3Store) CreateBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})

	if err == nil {
		return nil // Bucket exists
	}

	// Create bucket
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})

	return err
}

// Upload uploads an object
func (s *S3Store) Upload(ctx context.Context, key string, r io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	return err
}

// Download downloads an object
func (s *S3Store) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

// PresignedURL generates a pre-signed URL for downloading
func (s *S3Store) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return "", err
	}

	return result.URL, nil
}

// List lists objects with a prefix
func (s *S3Store) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	var objects []ObjectInfo
	for _, obj := range result.Contents {
		info := ObjectInfo{
			Key:          aws.ToString(obj.Key),
			SizeBytes:    aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		}
		objects = append(objects, info)
	}

	return objects, nil
}

// Delete deletes an object
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeletePrefix deletes all objects with a prefix
func (s *S3Store) DeletePrefix(ctx context.Context, prefix string) error {
	// List objects
	objects, err := s.List(ctx, prefix)
	if err != nil {
		return err
	}

	if len(objects) == 0 {
		return nil
	}

	// Build delete request
	var objectIds []types.ObjectIdentifier
	for _, obj := range objects {
		objectIds = append(objectIds, types.ObjectIdentifier{
			Key: aws.String(obj.Key),
		})
	}

	_, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(s.bucket),
		Delete: &types.Delete{
			Objects: objectIds,
			Quiet:   aws.Bool(true),
		},
	})

	return err
}

// Exists checks if an object exists
func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}
	return true, nil
}

// ObjectInfo holds metadata about an object
type ObjectInfo struct {
	Key          string
	SizeBytes    int64
	ContentType  string
	LastModified time.Time
}
