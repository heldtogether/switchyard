package worker

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/client"
)

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
