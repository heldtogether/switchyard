package dockerexec

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"

	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
)

type DockerExecutor struct {
	*executor.BaseExecutor
}

func New(dockerHost, nfsBasePath string, networkIsolated bool) (*DockerExecutor, error) {
	base, err := executor.NewBaseExecutor(dockerHost, nfsBasePath, networkIsolated)
	if err != nil {
		return nil, err
	}
	return &DockerExecutor{BaseExecutor: base}, nil
}

func (e *DockerExecutor) CreateRun(ctx context.Context, spec executor.RunSpec) (executor.RunRef, error) {
	containerName := fmt.Sprintf("job-%s", spec.JobID)
	networkName := fmt.Sprintf("%s-net", containerName)

	// Prepare job output directory on NFS
	jobOutputPath, err := e.PrepareOutputDirectory(spec.JobID)
	if err != nil {
		return executor.RunRef{}, err
	}

	// Create a per-job network (bridge)
	netID, err := e.CreateNetwork(ctx, networkName, spec.JobID, "bridge")
	if err != nil {
		return executor.RunRef{}, fmt.Errorf("create network: %w", err)
	}

	if err := e.pullImage(ctx, spec.Image, spec.RegistryAuth); err != nil {
		_ = e.Client.NetworkRemove(ctx, netID)
		return executor.RunRef{}, err
	}

	// Build environment variables with system-managed SWITCHYARD_* vars
	env := executor.BuildSystemEnv(spec)

	cfg := &container.Config{
		Image: spec.Image,
		Env:   env,
		Cmd:   spec.Command,
		Labels: map[string]string{
			"jobrunner.job_id":  spec.JobID,
			"jobrunner.managed": "true",
		},
	}

	// Configure resource limits
	resources := container.Resources{}
	if spec.CPU != "" {
		resources.NanoCPUs = executor.ParseCPU(spec.CPU)
	}
	if spec.Memory != "" {
		resources.Memory = executor.ParseMemory(spec.Memory)
	}
	if spec.GPUCount > 0 {
		request := container.DeviceRequest{
			Driver:       "nvidia",
			Capabilities: [][]string{{"gpu"}},
		}
		if len(spec.GPUDeviceIDs) > 0 {
			if len(spec.GPUDeviceIDs) != spec.GPUCount {
				return executor.RunRef{}, fmt.Errorf("gpu device id count mismatch: gpu_count=%d device_ids=%d", spec.GPUCount, len(spec.GPUDeviceIDs))
			}
			request.DeviceIDs = append([]string(nil), spec.GPUDeviceIDs...)
		} else {
			request.Count = spec.GPUCount
		}
		resources.DeviceRequests = []container.DeviceRequest{request}
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "no"},
		Resources:     resources,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: jobOutputPath,
				Target: "/outputs",
			},
		},
	}

	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	// Create container
	createResp, err := e.Client.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, containerName)
	if err != nil {
		_ = e.Client.NetworkRemove(ctx, netID)
		return executor.RunRef{}, fmt.Errorf("container create: %w", err)
	}

	// Start container
	if err := e.Client.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		_ = e.Client.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		_ = e.Client.NetworkRemove(ctx, netID)
		return executor.RunRef{}, fmt.Errorf("container start: %w", err)
	}

	return executor.RunRef{
		ExecutorType: "docker",
		Reference:    createResp.ID,
	}, nil
}

func (e *DockerExecutor) pullImage(ctx context.Context, imageRef string, auth *domain.RegistryAuth) error {
	opts := image.PullOptions{}
	if auth != nil {
		encoded, err := executor.BuildRegistryAuthString(auth)
		if err != nil {
			return fmt.Errorf("build registry auth: %w", err)
		}
		opts.RegistryAuth = encoded
	}

	reader, err := e.Client.ImagePull(ctx, imageRef, opts)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	defer reader.Close()

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	return nil
}

func (e *DockerExecutor) Wait(ctx context.Context, ref executor.RunRef) (executor.Result, error) {
	var res executor.Result

	waitC, errC := e.Client.ContainerWait(ctx, ref.Reference, container.WaitConditionNotRunning)

	select {
	case <-ctx.Done():
		return res, ctx.Err()
	case err := <-errC:
		if err != nil {
			return res, err
		}
		return res, fmt.Errorf("container wait error (nil)")
	case status := <-waitC:
		res.FinishedAt = time.Now()
		res.ExitCode = int(status.StatusCode)
		if status.StatusCode == 0 {
			res.Status = executor.StatusSuccess
			return res, nil
		}
		res.Status = executor.StatusFailed
		res.Error = fmt.Errorf("job failed with exit code %d", status.StatusCode)
		return res, nil
	}
}

func (e *DockerExecutor) Status(ctx context.Context, ref executor.RunRef) (executor.ExecutorStatus, error) {
	inspect, err := e.Client.ContainerInspect(ctx, ref.Reference)
	if err != nil {
		return executor.StatusUnknown, err
	}

	switch inspect.State.Status {
	case "created":
		return executor.StatusPending, nil
	case "running":
		return executor.StatusRunning, nil
	case "exited":
		if inspect.State.ExitCode == 0 {
			return executor.StatusSuccess, nil
		}
		return executor.StatusFailed, nil
	default:
		return executor.StatusUnknown, nil
	}
}

func (e *DockerExecutor) GetLogs(ctx context.Context, ref executor.RunRef, w io.Writer) error {
	rc, err := e.Client.ContainerLogs(ctx, ref.Reference, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Details:    false,
	})
	if err != nil {
		return err
	}
	defer rc.Close()

	// Note: Docker multiplexes stdout/stderr in a binary stream unless TTY=true.
	// If you need clean text, use stdcopy.StdCopy(w, w, rc).
	_, err = io.Copy(w, rc)
	return err
}

func (e *DockerExecutor) CollectOutputs(ctx context.Context, ref executor.RunRef, spec executor.OutputSpec) ([]domain.Artefact, error) {
	// Inspect container to get job ID from labels
	inspect, err := e.Client.ContainerInspect(ctx, ref.Reference)
	if err != nil {
		return nil, err
	}

	jobID := executor.ExtractJobIDFromLabels(inspect.Config.Labels)
	if jobID == "" {
		return nil, fmt.Errorf("missing jobrunner.job_id label on container")
	}

	// Walk NFS output directory and upload to S3
	jobOutputPath := filepath.Join(e.NFSBasePath, "jobs", jobID, "outputs")
	return executor.WalkAndUploadOutputs(ctx, jobOutputPath, spec)
}

func (e *DockerExecutor) Cancel(ctx context.Context, ref executor.RunRef) error {
	timeout := 10 * time.Second
	return e.Client.ContainerStop(ctx, ref.Reference, container.StopOptions{Timeout: ptr(int(timeout.Seconds()))})
}

func (e *DockerExecutor) Cleanup(ctx context.Context, ref executor.RunRef) error {
	// Get container info to find network before removing
	inspect, err := e.Client.ContainerInspect(ctx, ref.Reference)
	var networkIDs []string
	if err == nil {
		for netName := range inspect.NetworkSettings.Networks {
			// Store network IDs for cleanup
			if net, err := e.Client.NetworkInspect(ctx, netName, network.InspectOptions{}); err == nil {
				// Only remove networks we created (with our labels)
				if net.Labels["jobrunner.managed"] == "true" {
					networkIDs = append(networkIDs, net.ID)
				}
			}
		}
	}

	// Remove container
	_ = e.Client.ContainerRemove(ctx, ref.Reference, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	// Remove networks
	for _, netID := range networkIDs {
		_ = e.Client.NetworkRemove(ctx, netID)
	}

	return nil
}

func ptr[T any](v T) *T { return &v }
