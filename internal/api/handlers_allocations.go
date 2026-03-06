package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
)

// HandleClaimAllocation handles POST /v1/allocations/claim
func (a *API) HandleClaimAllocation(w http.ResponseWriter, r *http.Request) {
	var req AllocationClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.JobID == uuid.Nil || req.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "job_id and node_id are required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	allocation, err := a.store.ClaimGPUAllocation(r.Context(), req.JobID, req.NodeID)
	if err != nil {
		a.logger.Error("failed to claim allocation", "error", err, "job_id", req.JobID, "node_id", req.NodeID)
		switch err {
		case postgres.ErrNodeNotFound, postgres.ErrJobNotFound:
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
				Code:    http.StatusNotFound,
			})
			return
		case postgres.ErrInsufficientGPU:
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error:   "insufficient_capacity",
				Message: "insufficient GPU capacity on node",
				Code:    http.StatusConflict,
			})
			return
		default:
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to claim allocation",
				Code:    http.StatusInternalServerError,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, AllocationClaimResponse{
		AllocationID: allocation.ID,
		NodeID:       allocation.NodeID,
		GPUCount:     allocation.GPUCount,
		DeviceIDs:    allocation.DeviceIDs,
	})
}

// HandleReleaseAllocation handles POST /v1/allocations/release
func (a *API) HandleReleaseAllocation(w http.ResponseWriter, r *http.Request) {
	var req AllocationReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if req.JobID == uuid.Nil || req.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "job_id and node_id are required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if err := a.store.ReleaseGPUAllocation(r.Context(), req.JobID, req.NodeID); err != nil {
		if err == postgres.ErrNodeNotFound || err == postgres.ErrJobNotFound {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
				Code:    http.StatusNotFound,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to release allocation",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleGetAllocationCapacity handles GET /v1/allocations/capacity
func (a *API) HandleGetAllocationCapacity(w http.ResponseWriter, r *http.Request) {
	maxGPU, err := a.store.MaxGPUPerNode(r.Context())
	if err != nil {
		a.logger.Error("failed to fetch max gpu per node", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to fetch allocation capacity",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, AllocationCapacityResponse{
		MaxGPUPerNode: maxGPU,
	})
}
