package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RedisContainer holds references to a test Redis container.
type RedisContainer struct {
	URL       string
	container testcontainers.Container
}

// Cleanup terminates the container and cleans up resources.
func (rc *RedisContainer) Cleanup(t *testing.T) {
	if err := testcontainers.TerminateContainer(rc.container); err != nil {
		t.Logf("failed to terminate redis container: %v", err)
	}
}

// SetupTestRedis creates a Redis testcontainer and returns a connection URL.
func SetupTestRedis(t *testing.T) *RedisContainer {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start redis container")

	host, err := container.Host(ctx)
	require.NoError(t, err, "failed to get redis host")

	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err, "failed to get redis port")

	url := fmt.Sprintf("redis://%s:%s/0", host, port.Port())

	return &RedisContainer{
		URL:       url,
		container: container,
	}
}
