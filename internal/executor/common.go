package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/heldtogether/switchyard/internal/domain"
)

// BaseExecutor contains shared state and functionality for Docker-based executors
type BaseExecutor struct {
	Client          *client.Client
	NFSBasePath     string
	NetworkIsolated bool
}

// NewBaseExecutor creates a base executor with Docker client
func NewBaseExecutor(dockerHost, nfsBasePath string, networkIsolated bool) (*BaseExecutor, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &BaseExecutor{
		Client:          cli,
		NFSBasePath:     nfsBasePath,
		NetworkIsolated: networkIsolated,
	}, nil
}

// PrepareOutputDirectory creates NFS output directory for a job
func (b *BaseExecutor) PrepareOutputDirectory(jobID string) (string, error) {
	outputPath := filepath.Join(b.NFSBasePath, "jobs", jobID, "outputs")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	return outputPath, nil
}

// CreateNetwork creates an isolated network for a job
func (b *BaseExecutor) CreateNetwork(ctx context.Context, name, jobID, driver string) (string, error) {
	resp, err := b.Client.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:   driver,
		Internal: b.NetworkIsolated,
		Labels: map[string]string{
			"jobrunner.job_id":  jobID,
			"jobrunner.managed": "true",
		},
	})
	return resp.ID, err
}

// BuildRegistryAuthString encodes registry auth for Docker API
func BuildRegistryAuthString(auth *domain.RegistryAuth) (string, error) {
	if auth == nil {
		return "", nil
	}

	ac := registry.AuthConfig{
		Username: auth.Username,
		Password: auth.Password,
	}
	b, err := json.Marshal(ac)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ParseCPU converts CPU string to nanocpus (1.0 = 1e9 nanocpus = 1 CPU core)
func ParseCPU(cpu string) int64 {
	if cpu == "" {
		return 0
	}
	var value float64
	fmt.Sscanf(cpu, "%f", &value)
	return int64(value * 1e9)
}

// ParseMemory converts memory string to bytes ("2g", "512m", etc.)
func ParseMemory(mem string) int64 {
	if mem == "" {
		return 0
	}
	mem = strings.ToLower(strings.TrimSpace(mem))
	var value int64
	var unit string
	fmt.Sscanf(mem, "%d%s", &value, &unit)

	switch unit {
	case "g", "gb":
		return value * 1024 * 1024 * 1024
	case "m", "mb":
		return value * 1024 * 1024
	case "k", "kb":
		return value * 1024
	default:
		return value
	}
}

// ExtractJobIDFromLabels safely extracts job ID from Docker labels
func ExtractJobIDFromLabels(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	return labels["jobrunner.job_id"]
}

// WalkAndUploadOutputs walks output directory and uploads files to S3
func WalkAndUploadOutputs(ctx context.Context, outputPath string, spec OutputSpec) ([]domain.Artefact, error) {
	var artefacts []domain.Artefact

	err := filepath.Walk(outputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, _ := filepath.Rel(outputPath, path)
		objectKey := filepath.Join(spec.KeyPrefix, relPath)

		// Upload to S3
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		if err := spec.ObjectStore.Upload(ctx, objectKey, file, contentType); err != nil {
			return fmt.Errorf("failed to upload %s: %w", relPath, err)
		}

		artefacts = append(artefacts, domain.Artefact{
			Path:        relPath,
			ObjectKey:   objectKey,
			SizeBytes:   info.Size(),
			ContentType: contentType,
		})

		return nil
	})

	return artefacts, err
}
