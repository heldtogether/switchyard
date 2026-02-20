package dockerexec

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"

	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
)

type DockerExecutor struct {
	client          *client.Client
	nfsBasePath     string
	networkIsolated bool
}

func New(dockerHost, nfsBasePath string, networkIsolated bool) (*DockerExecutor, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &DockerExecutor{
		client:          cli,
		nfsBasePath:     nfsBasePath,
		networkIsolated: networkIsolated,
	}, nil
}

func (e *DockerExecutor) CreateRun(ctx context.Context, spec executor.RunSpec) (executor.RunRef, error) {
	containerName := fmt.Sprintf("job-%s", spec.JobID)
	networkName := fmt.Sprintf("%s-net", containerName)

	// Prepare job output directory on NFS
	jobOutputPath := filepath.Join(e.nfsBasePath, "jobs", spec.JobID, "outputs")
	if err := os.MkdirAll(jobOutputPath, 0o755); err != nil {
		return executor.RunRef{}, fmt.Errorf("mkdir outputs: %w", err)
	}

	// Create a per-job network (bridge). For true isolation you can also run with NetworkDisabled.
	netID, err := e.createNetwork(ctx, networkName, spec.JobID)
	if err != nil {
		return executor.RunRef{}, fmt.Errorf("create network: %w", err)
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
		// If you want hard isolation from the start:
		// NetworkDisabled: e.networkIsolated,
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "no"},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: jobOutputPath,
				Target: "/outputs",
			},
		},
		// Optional resource limits if you want parity with swarm:
		// Resources: container.Resources{NanoCPUs: ..., Memory: ...},
	}

	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	// Pull image (optional: only if you want to ensure it exists before create)
	// _, _ = e.client.ImagePull(ctx, spec.Image, types.ImagePullOptions{RegistryAuth: ...})

	createResp, err := e.client.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, containerName)
	if err != nil {
		_ = e.client.NetworkRemove(ctx, netID)
		return executor.RunRef{}, fmt.Errorf("container create: %w", err)
	}

	// Start
	if err := e.client.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		_ = e.client.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		_ = e.client.NetworkRemove(ctx, netID)
		return executor.RunRef{}, fmt.Errorf("container start: %w", err)
	}

	return executor.RunRef{
		ExecutorType: "docker",
		Reference:    createResp.ID,
		// Strongly consider extending RunRef to include JobID and NetworkID:
		// Metadata: map[string]string{"job_id": spec.JobID, "network_id": netID},
	}, nil
}

func (e *DockerExecutor) Wait(ctx context.Context, ref executor.RunRef) (executor.Result, error) {
	var res executor.Result

	waitC, errC := e.client.ContainerWait(ctx, ref.Reference, container.WaitConditionNotRunning)

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
	inspect, err := e.client.ContainerInspect(ctx, ref.Reference)
	if err != nil {
		// If container is gone, treat as unknown; you may want special-case NotFound.
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
	rc, err := e.client.ContainerLogs(ctx, ref.Reference, container.LogsOptions{
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
	// Don’t attempt to derive jobID from ref.Reference (container ID).
	// Either:
	// 1) include JobID in RunRef, OR
	// 2) inspect container labels here:
	inspect, err := e.client.ContainerInspect(ctx, ref.Reference)
	if err != nil {
		return nil, err
	}
	jobID := inspect.Config.Labels["jobrunner.job_id"]
	if jobID == "" {
		return nil, fmt.Errorf("missing jobrunner.job_id label on container")
	}

	// Same NFS walk/upload logic you already have:
	jobOutputPath := filepath.Join(e.nfsBasePath, "jobs", jobID, "outputs")
	return walkAndUpload(jobOutputPath, spec)
}

func (e *DockerExecutor) Cancel(ctx context.Context, ref executor.RunRef) error {
	timeout := 10 * time.Second
	return e.client.ContainerStop(ctx, ref.Reference, container.StopOptions{Timeout: ptr(int(timeout.Seconds()))})
}

func (e *DockerExecutor) Cleanup(ctx context.Context, ref executor.RunRef) error {
	// Remove container
	_ = e.client.ContainerRemove(ctx, ref.Reference, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	// Remove network if we can infer it
	inspect, err := e.client.ContainerInspect(ctx, ref.Reference)
	if err == nil {
		for netName := range inspect.NetworkSettings.Networks {
			// best-effort; you might instead store the network ID in RunRef metadata
			_ = e.client.NetworkRemove(ctx, netName)
		}
	}
	return nil
}

func (e *DockerExecutor) createNetwork(ctx context.Context, name, jobID string) (string, error) {
	resp, err := e.client.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:   "bridge",
		Internal: e.networkIsolated, // internal bridge network (no external access)
		Labels: map[string]string{
			"jobrunner.job_id":  jobID,
			"jobrunner.managed": "true",
		},
	})
	return resp.ID, err
}

func buildRegistryAuth(auth *domain.RegistryAuth) (string, error) {
	ac := registry.AuthConfig{Username: auth.Username, Password: auth.Password}
	b, err := json.Marshal(ac)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func ptr[T any](v T) *T { return &v }

// walkAndUpload: reuse your existing Walk + Upload logic
func walkAndUpload(jobOutputPath string, spec executor.OutputSpec) ([]domain.Artefact, error) {
	// paste your existing filepath.Walk + upload here
	return nil, fmt.Errorf("not implemented")
}
