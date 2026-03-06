package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestGetAllocationCapacity(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	api := &API{store: store, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/allocations/capacity", api.HandleGetAllocationCapacity)

	// No nodes -> 0
	req := httptest.NewRequest(http.MethodGet, "/v1/allocations/capacity", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp AllocationCapacityResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, 0, resp.MaxGPUPerNode)

	// Add nodes and expect max
	ctx := context.Background()
	nodeA := &domain.Node{ID: "node-a", Hostname: "node-a", Executor: domain.ExecutorType("docker"), GPUTotal: 2}
	nodeB := &domain.Node{ID: "node-b", Hostname: "node-b", Executor: domain.ExecutorType("docker"), GPUTotal: 4}
	require.NoError(t, store.UpsertNode(ctx, nodeA))
	require.NoError(t, store.UpsertNode(ctx, nodeB))

	req = httptest.NewRequest(http.MethodGet, "/v1/allocations/capacity", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	resp = AllocationCapacityResponse{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, 4, resp.MaxGPUPerNode)
}
