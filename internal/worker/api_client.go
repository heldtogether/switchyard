package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInsufficientGPU = fmt.Errorf("insufficient gpu capacity")
	ErrNotFound        = fmt.Errorf("not found")
)

// APIClient is a minimal client for worker -> API calls.
type APIClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewAPIClient creates a new API client.
func NewAPIClient(baseURL, apiKey string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterWorker registers a worker node with the API.
func (c *APIClient) RegisterWorker(ctx context.Context, req RegisterWorkerRequest) error {
	return c.post(ctx, "/v1/workers/register", req, nil)
}

// Heartbeat sends a worker heartbeat to the API.
func (c *APIClient) Heartbeat(ctx context.Context, req WorkerHeartbeatRequest) error {
	return c.post(ctx, "/v1/workers/heartbeat", req, nil)
}

// ClaimAllocation attempts to claim a GPU allocation for a job on a node.
func (c *APIClient) ClaimAllocation(ctx context.Context, req AllocationClaimRequest) (*AllocationClaimResponse, error) {
	var resp AllocationClaimResponse
	status, err := c.postWithStatus(ctx, "/v1/allocations/claim", req, &resp)
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict {
		return nil, ErrInsufficientGPU
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if status >= 300 {
		return nil, fmt.Errorf("claim failed with status %d", status)
	}
	return &resp, nil
}

// ReleaseAllocation releases a GPU allocation for a job on a node.
func (c *APIClient) ReleaseAllocation(ctx context.Context, req AllocationReleaseRequest) error {
	status, err := c.postWithStatus(ctx, "/v1/allocations/release", req, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return ErrNotFound
	}
	if status >= 300 {
		return fmt.Errorf("release failed with status %d", status)
	}
	return nil
}

func (c *APIClient) post(ctx context.Context, path string, payload any, out any) error {
	_, err := c.postWithStatus(ctx, path, payload, out)
	return err
}

func (c *APIClient) postWithStatus(ctx context.Context, path string, payload any, out any) (int, error) {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, buf)
	if err != nil {
		return 0, err
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
			Code    int    `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Message != "" {
			return resp.StatusCode, fmt.Errorf("%s (status %d)", errResp.Message, resp.StatusCode)
		}
		return resp.StatusCode, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, err
		}
	}

	return resp.StatusCode, nil
}

// Local copies of API DTOs

type RegisterWorkerRequest struct {
	NodeID   string            `json:"node_id"`
	Hostname string            `json:"hostname"`
	Executor string            `json:"executor"`
	GPUTotal int               `json:"gpu_total"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type WorkerHeartbeatRequest struct {
	NodeID   string `json:"node_id"`
	GPUTotal int    `json:"gpu_total"`
}

type AllocationClaimRequest struct {
	JobID  uuid.UUID `json:"job_id"`
	NodeID string    `json:"node_id"`
}

type AllocationClaimResponse struct {
	AllocationID uuid.UUID `json:"allocation_id"`
	NodeID       string    `json:"node_id"`
	GPUCount     int       `json:"gpu_count"`
}

type AllocationReleaseRequest struct {
	JobID  uuid.UUID `json:"job_id"`
	NodeID string    `json:"node_id"`
}
