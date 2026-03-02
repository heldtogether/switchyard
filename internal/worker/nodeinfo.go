package worker

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const defaultGPUDetectImage = "nvidia/cuda:12.4.1-runtime-ubuntu22.04"

// DetectNodeInfo attempts to discover node ID and hostname.
func DetectNodeInfo(ctx context.Context, dockerHost string) (string, string, error) {
	hostname, _ := os.Hostname()

	cli, err := client.NewClientWithOpts(client.WithHost(dockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		return "", hostname, err
	}
	defer cli.Close()

	info, err := cli.Info(ctx)
	if err != nil {
		return "", hostname, err
	}

	nodeID := info.Swarm.NodeID
	if hostname == "" {
		hostname = info.Name
	}

	return nodeID, hostname, nil
}

// DetectGPUCount returns the number of GPUs via nvidia-smi, or 0 if unavailable.
func DetectGPUCount(ctx context.Context) int {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "-L")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// DetectGPUCountViaDocker runs nvidia-smi in a short-lived container and counts GPUs.
// Returns 0 on failure.
func DetectGPUCountViaDocker(ctx context.Context, dockerHost string, detectImage string) int {
	if detectImage == "" {
		detectImage = defaultGPUDetectImage
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.WithHost(dockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		return 0
	}
	defer cli.Close()

	if err := ensureImagePresent(ctx, cli, detectImage); err != nil {
		return 0
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      detectImage,
		Entrypoint: []string{"nvidia-smi"},
		Cmd:        []string{"-L"},
		Tty:        false,
	}, &container.HostConfig{
		Resources: container.Resources{
			DeviceRequests: []container.DeviceRequest{
				{
					Driver:       "nvidia",
					Count:        -1,
					Capabilities: [][]string{{"gpu"}},
				},
			},
		},
	}, nil, nil, "")
	if err != nil {
		return 0
	}
	defer cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return 0
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return 0
		}
	case <-statusCh:
	case <-ctx.Done():
		return 0
	}

	logs, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return 0
	}
	defer logs.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, logs); err != nil {
		return 0
	}

	return countGPULines(stdout.String())
}

func countGPULines(output string) int {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func ensureImagePresent(ctx context.Context, cli *client.Client, imageRef string) error {
	// Check if image exists locally
	_, err := cli.ImageInspect(ctx, imageRef)
	if err == nil {
		return nil
	}

	// If it's not a "not found" error, return it
	if !errdefs.IsNotFound(err) {
		return err
	}

	// Pull image if missing
	reader, err := cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Drain response to complete pull
	_, err = io.Copy(io.Discard, reader)
	return err
}
