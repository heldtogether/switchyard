package swarm

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
)

// SwarmExecutor implements job execution on Docker Swarm
type SwarmExecutor struct {
	*executor.BaseExecutor
}

// New creates a new Swarm executor
func New(dockerHost, nfsBasePath string, networkIsolated bool) (*SwarmExecutor, error) {
	base, err := executor.NewBaseExecutor(dockerHost, nfsBasePath, networkIsolated)
	if err != nil {
		return nil, err
	}
	return &SwarmExecutor{BaseExecutor: base}, nil
}

// CreateRun starts a job as a Swarm service
func (e *SwarmExecutor) CreateRun(ctx context.Context, spec executor.RunSpec) (executor.RunRef, error) {
	serviceName := fmt.Sprintf("job-%s", spec.JobID)
	networkName := fmt.Sprintf("%s-net", serviceName)

	// Create isolated overlay network
	networkID, err := e.CreateNetwork(ctx, networkName, spec.JobID, "overlay")
	if err != nil {
		return executor.RunRef{}, fmt.Errorf("failed to create network: %w", err)
	}

	// Prepare job output directory on NFS
	jobOutputPath, err := e.PrepareOutputDirectory(spec.JobID)
	if err != nil {
		e.Client.NetworkRemove(ctx, networkID)
		return executor.RunRef{}, err
	}

	// Build service spec
	serviceSpec := e.buildServiceSpec(spec, serviceName, jobOutputPath, networkID)

	createOpts := types.ServiceCreateOptions{}
	if spec.RegistryAuth != nil {
		encoded, err := executor.BuildRegistryAuthString(spec.RegistryAuth)
		if err != nil {
			e.Client.NetworkRemove(ctx, networkID)
			return executor.RunRef{}, fmt.Errorf("failed to build registry auth: %w", err)
		}
		createOpts.EncodedRegistryAuth = encoded
	}

	// Create service
	resp, err := e.Client.ServiceCreate(ctx, serviceSpec, createOpts)
	if err != nil {
		// Cleanup network on failure
		e.Client.NetworkRemove(ctx, networkID)
		return executor.RunRef{}, fmt.Errorf("failed to create service: %w", err)
	}

	return executor.RunRef{
		ExecutorType: "swarm",
		Reference:    resp.ID,
	}, nil
}

// buildServiceSpec constructs the Swarm service specification
func (e *SwarmExecutor) buildServiceSpec(spec executor.RunSpec, serviceName, outputPath, networkID string) swarm.ServiceSpec {
	// Parse resource limits using shared utilities
	resources := &swarm.ResourceRequirements{}
	if spec.CPU != "" || spec.Memory != "" || spec.GPUCount > 0 {
		resources.Limits = &swarm.Limit{}
		if spec.CPU != "" {
			resources.Limits.NanoCPUs = executor.ParseCPU(spec.CPU)
		}
		if spec.Memory != "" {
			resources.Limits.MemoryBytes = executor.ParseMemory(spec.Memory)
		}
	}
	if spec.GPUCount > 0 {
		resources.Reservations = &swarm.Resources{
			GenericResources: []swarm.GenericResource{
				{
					DiscreteResourceSpec: &swarm.DiscreteGenericResource{
						Kind:  "gpu",
						Value: int64(spec.GPUCount),
					},
				},
			},
		}
	}

	// Build environment variables with system-managed SWITCHYARD_* vars
	env := executor.BuildSystemEnv(spec)

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
	if spec.NodeID != "" {
		taskTemplate.Placement = &swarm.Placement{
			Constraints: []string{fmt.Sprintf("node.id==%s", spec.NodeID)},
		}
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
	service, _, err := e.Client.ServiceInspectWithRaw(ctx, ref.Reference, types.ServiceInspectOptions{})
	if err != nil {
		return executor.StatusUnknown, err
	}

	// Get tasks for this service
	tasks, err := e.Client.TaskList(ctx, types.TaskListOptions{
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
	logs, err := e.Client.ServiceLogs(ctx, ref.Reference, container.LogsOptions{
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
	// Get service to extract job ID from labels
	service, _, err := e.Client.ServiceInspectWithRaw(ctx, ref.Reference, types.ServiceInspectOptions{})
	if err != nil {
		return nil, err
	}

	jobID := executor.ExtractJobIDFromLabels(service.Spec.Labels)
	if jobID == "" {
		return nil, fmt.Errorf("missing jobrunner.job_id label on service")
	}

	// Walk NFS output directory and upload to S3
	jobOutputPath := filepath.Join(e.NFSBasePath, "jobs", jobID, "outputs")
	return executor.WalkAndUploadOutputs(ctx, jobOutputPath, spec)
}

// Cancel stops a running job
func (e *SwarmExecutor) Cancel(ctx context.Context, ref executor.RunRef) error {
	return e.Client.ServiceRemove(ctx, ref.Reference)
}

// Cleanup removes all resources for a job
func (e *SwarmExecutor) Cleanup(ctx context.Context, ref executor.RunRef) error {
	// Get service to find network
	service, _, err := e.Client.ServiceInspectWithRaw(ctx, ref.Reference, types.ServiceInspectOptions{})
	if err == nil {
		// Remove service
		e.Client.ServiceRemove(ctx, ref.Reference)

		// Remove network
		for _, network := range service.Spec.TaskTemplate.Networks {
			e.Client.NetworkRemove(ctx, network.Target)
		}
	}

	return nil
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}
