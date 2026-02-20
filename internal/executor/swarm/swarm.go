package swarm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
)

// SwarmExecutor implements job execution on Docker Swarm
type SwarmExecutor struct {
	client          *client.Client
	nfsBasePath     string
	networkIsolated bool
}

// New creates a new Swarm executor
func New(dockerHost, nfsBasePath string, networkIsolated bool) (*SwarmExecutor, error) {
	opts := []client.Opt{
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &SwarmExecutor{
		client:          cli,
		nfsBasePath:     nfsBasePath,
		networkIsolated: networkIsolated,
	}, nil
}

// CreateRun starts a job as a Swarm service
func (e *SwarmExecutor) CreateRun(ctx context.Context, spec executor.RunSpec) (executor.RunRef, error) {
	serviceName := fmt.Sprintf("job-%s", spec.JobID)

	// Create isolated network
	networkName := fmt.Sprintf("%s-net", serviceName)
	networkID, err := e.createNetwork(ctx, networkName, spec.JobID)
	if err != nil {
		return executor.RunRef{}, fmt.Errorf("failed to create network: %w", err)
	}

	// Prepare job output directory on NFS
	jobOutputPath := filepath.Join(e.nfsBasePath, "jobs", spec.JobID, "outputs")
	if err := os.MkdirAll(jobOutputPath, 0755); err != nil {
		return executor.RunRef{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build service spec
	serviceSpec := e.buildServiceSpec(spec, serviceName, jobOutputPath, networkID)

	// Create service
	resp, err := e.client.ServiceCreate(ctx, serviceSpec, types.ServiceCreateOptions{})
	if err != nil {
		// Cleanup network on failure
		e.client.NetworkRemove(ctx, networkID)
		return executor.RunRef{}, fmt.Errorf("failed to create service: %w", err)
	}

	return executor.RunRef{
		ExecutorType: "swarm",
		Reference:    resp.ID,
	}, nil
}

// buildServiceSpec constructs the Swarm service specification
func (e *SwarmExecutor) buildServiceSpec(spec executor.RunSpec, serviceName, outputPath, networkID string) swarm.ServiceSpec {
	// Parse resource limits
	resources := &swarm.ResourceRequirements{}
	if spec.CPU != "" || spec.Memory != "" {
		resources.Limits = &swarm.Limit{}
		if spec.CPU != "" {
			cpuNano := parseCPU(spec.CPU)
			resources.Limits.NanoCPUs = cpuNano
		}
		if spec.Memory != "" {
			memBytes := parseMemory(spec.Memory)
			resources.Limits.MemoryBytes = memBytes
		}
	}

	// Build environment variables
	env := []string{
		fmt.Sprintf("JOB_ID=%s", spec.JobID),
	}
	for k, v := range spec.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Container spec
	containerSpec := &swarm.ContainerSpec{
		Image:   spec.Image,
		Command: spec.Command,
		Env:     env,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: outputPath,
				Target: "/outputs",
			},
		},
	}

	// Add registry auth if provided
	if spec.RegistryAuth != nil {
		containerSpec.Privileges = &swarm.Privileges{
			CredentialSpec: buildRegistryAuth(spec.RegistryAuth),
		}
	}

	// Task template
	taskTemplate := swarm.TaskSpec{
		ContainerSpec: containerSpec,
		Resources:     resources,
		RestartPolicy: &swarm.RestartPolicy{
			Condition: swarm.RestartPolicyConditionNone,
		},
		Networks: []swarm.NetworkAttachmentConfig{
			{Target: networkID},
		},
	}

	// Service spec
	return swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: serviceName,
			Labels: map[string]string{
				"jobrunner.job_id":  spec.JobID,
				"jobrunner.managed": "true",
			},
		},
		TaskTemplate: taskTemplate,
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: uint64Ptr(1),
			},
		},
	}
}

// Wait blocks until the job completes
func (e *SwarmExecutor) Wait(ctx context.Context, ref executor.RunRef) (executor.Result, error) {
	result := executor.Result{}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-ticker.C:
			status, err := e.Status(ctx, ref)
			if err != nil {
				return result, err
			}

			switch status {
			case executor.StatusSuccess:
				result.Status = status
				result.ExitCode = 0
				result.FinishedAt = time.Now()
				return result, nil
			case executor.StatusFailed:
				result.Status = status
				result.ExitCode = 1
				result.Error = fmt.Errorf("job failed")
				result.FinishedAt = time.Now()
				return result, nil
			case executor.StatusRunning:
				// Continue waiting
			default:
				// Still pending
			}
		}
	}
}

// Status checks the current status of a job
func (e *SwarmExecutor) Status(ctx context.Context, ref executor.RunRef) (executor.ExecutorStatus, error) {
	service, _, err := e.client.ServiceInspectWithRaw(ctx, ref.Reference, types.ServiceInspectOptions{})
	if err != nil {
		return executor.StatusUnknown, err
	}

	// Get tasks for this service
	tasks, err := e.client.TaskList(ctx, types.TaskListOptions{
		Filters: filters.NewArgs(filters.Arg("service", service.ID)),
	})
	if err != nil {
		return executor.StatusUnknown, err
	}

	if len(tasks) == 0 {
		return executor.StatusPending, nil
	}

	// Check most recent task
	task := tasks[0]
	switch task.Status.State {
	case swarm.TaskStateNew, swarm.TaskStatePending, swarm.TaskStateAssigned, swarm.TaskStateAccepted, swarm.TaskStatePreparing, swarm.TaskStateReady, swarm.TaskStateStarting:
		return executor.StatusPending, nil
	case swarm.TaskStateRunning:
		return executor.StatusRunning, nil
	case swarm.TaskStateComplete:
		return executor.StatusSuccess, nil
	case swarm.TaskStateFailed, swarm.TaskStateRejected:
		return executor.StatusFailed, nil
	case swarm.TaskStateShutdown:
		return executor.StatusCancelled, nil
	default:
		return executor.StatusUnknown, nil
	}
}

// GetLogs retrieves logs from the service
func (e *SwarmExecutor) GetLogs(ctx context.Context, ref executor.RunRef, w io.Writer) error {
	logs, err := e.client.ServiceLogs(ctx, ref.Reference, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	})
	if err != nil {
		return err
	}
	defer logs.Close()

	_, err = io.Copy(w, logs)
	return err
}

// CollectOutputs uploads job outputs to object storage
func (e *SwarmExecutor) CollectOutputs(ctx context.Context, ref executor.RunRef, spec executor.OutputSpec) ([]domain.Artefact, error) {
	var artefacts []domain.Artefact

	// Extract job ID from reference
	jobID := extractJobID(ref.Reference)
	jobOutputPath := filepath.Join(e.nfsBasePath, "jobs", jobID, "outputs")

	// Walk output directory
	err := filepath.Walk(jobOutputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, _ := filepath.Rel(jobOutputPath, path)
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

// Cancel stops a running job
func (e *SwarmExecutor) Cancel(ctx context.Context, ref executor.RunRef) error {
	return e.client.ServiceRemove(ctx, ref.Reference)
}

// Cleanup removes all resources for a job
func (e *SwarmExecutor) Cleanup(ctx context.Context, ref executor.RunRef) error {
	// Get service to find network
	service, _, err := e.client.ServiceInspectWithRaw(ctx, ref.Reference, types.ServiceInspectOptions{})
	if err == nil {
		// Remove service
		e.client.ServiceRemove(ctx, ref.Reference)

		// Remove network
		for _, network := range service.Spec.TaskTemplate.Networks {
			e.client.NetworkRemove(ctx, network.Target)
		}
	}

	return nil
}

// Helper functions

func (e *SwarmExecutor) createNetwork(ctx context.Context, name, jobID string) (string, error) {
	resp, err := e.client.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "overlay",
		Labels: map[string]string{
			"jobrunner.job_id":  jobID,
			"jobrunner.managed": "true",
		},
		Internal: e.networkIsolated,
	})
	return resp.ID, err
}

func buildRegistryAuth(auth *domain.RegistryAuth) *swarm.CredentialSpec {
	authConfig := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: auth.Username,
		Password: auth.Password,
	}
	encoded, _ := json.Marshal(authConfig)
	return &swarm.CredentialSpec{
		Registry: base64.URLEncoding.EncodeToString(encoded),
	}
}

func parseCPU(cpu string) int64 {
	// Convert "1.0" to nanocpus (1 CPU = 1e9 nanocpus)
	var value float64
	fmt.Sscanf(cpu, "%f", &value)
	return int64(value * 1e9)
}

func parseMemory(mem string) int64 {
	// Parse "2g", "512m", etc.
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

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func extractJobID(serviceID string) string {
	// Service name format: job-{jobID}
	// This is a simplified extraction; in production, query service labels
	return strings.TrimPrefix(serviceID, "job-")
}
