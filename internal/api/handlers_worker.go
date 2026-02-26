package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
)

// HandleRegisterWorker handles POST /v1/workers/register
func (a *API) HandleRegisterWorker(w http.ResponseWriter, r *http.Request) {
	var req RegisterWorkerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.logger.Warn("invalid worker register request", "error", err, "remote", r.RemoteAddr)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.NodeID == "" {
		a.logger.Warn("worker register missing node_id", "remote", r.RemoteAddr, "hostname", req.Hostname, "executor", req.Executor)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "node_id is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.GPUTotal < 0 {
		a.logger.Warn("worker register invalid gpu_total", "remote", r.RemoteAddr, "node_id", req.NodeID, "gpu_total", req.GPUTotal)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "gpu_total must be >= 0",
			Code:    http.StatusBadRequest,
		})
		return
	}

	node := &domain.Node{
		ID:            req.NodeID,
		Hostname:      req.Hostname,
		Executor:      domain.ExecutorType(req.Executor),
		GPUTotal:      req.GPUTotal,
		LastHeartbeat: time.Now(),
	}

	if err := a.store.UpsertNode(r.Context(), node); err != nil {
		a.logger.Error("failed to register worker", "error", err, "node_id", req.NodeID)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to register worker",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, RegisterWorkerResponse{
		NodeID:       node.ID,
		RegisteredAt: node.CreatedAt,
	})
}

// HandleWorkerHeartbeat handles POST /v1/workers/heartbeat
func (a *API) HandleWorkerHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req WorkerHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.logger.Warn("invalid worker heartbeat request", "error", err, "remote", r.RemoteAddr)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.NodeID == "" {
		a.logger.Warn("worker heartbeat missing node_id", "remote", r.RemoteAddr)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "node_id is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.GPUTotal < 0 {
		a.logger.Warn("worker heartbeat invalid gpu_total", "remote", r.RemoteAddr, "node_id", req.NodeID, "gpu_total", req.GPUTotal)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "gpu_total must be >= 0",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if err := a.store.UpdateNodeHeartbeat(r.Context(), req.NodeID, req.GPUTotal); err != nil {
		if err == postgres.ErrNodeNotFound {
			a.logger.Warn("worker heartbeat node not found", "node_id", req.NodeID)
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: "Node not found",
				Code:    http.StatusNotFound,
			})
			return
		}
		a.logger.Error("failed to update worker heartbeat", "error", err, "node_id", req.NodeID)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to update heartbeat",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
