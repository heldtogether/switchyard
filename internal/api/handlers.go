package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/storage/objectstore"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/storage/queue"
)

// API holds the API dependencies
type API struct {
	cfg     *config.Config
	store   *postgres.Store
	queue   *queue.RedisQueue
	storage *objectstore.S3Store
	logger  *slog.Logger
	baseURL string
}

// New creates a new API instance
func New(cfg *config.Config, store *postgres.Store, q *queue.RedisQueue, storage *objectstore.S3Store, logger *slog.Logger, baseURL string) *API {
	return &API{
		cfg:     cfg,
		store:   store,
		queue:   q,
		storage: storage,
		logger:  logger,
		baseURL: baseURL,
	}
}

// HandleCreateJob handles POST /v1/jobs
func (a *API) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Validate request
	if req.Image == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "image is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if len(req.Outputs) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "at least one output path is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Validate environment variables
	if err := validateEnvVars(req.Env); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Create job
	job := &domain.Job{
		ID:          uuid.New(),
		CreatedBy:   "api-key-user", // TODO: Get from auth context
		Status:      domain.JobStatusPending,
		Image:       req.Image,
		Command:     req.Command,
		Env:         req.Env,
		Outputs:     req.Outputs,
		TimeoutSecs: int(a.cfg.Executor.Swarm.Defaults.Timeout.Seconds()),
		Executor:    domain.ExecutorTypeSwarm,
		Metadata:    req.Metadata,
	}

	// Set timeout if provided
	if req.TimeoutSecs != nil {
		job.TimeoutSecs = *req.TimeoutSecs
	}

	// Set resources if provided
	if req.Resources != nil {
		if req.Resources.CPU != "" {
			job.CPULimit = &req.Resources.CPU
		}
		if req.Resources.Memory != "" {
			job.MemoryLimit = &req.Resources.Memory
		}
	} else {
		// Use defaults from config
		cpu := a.cfg.Executor.Swarm.Defaults.Resources.CPU
		mem := a.cfg.Executor.Swarm.Defaults.Resources.Memory
		job.CPULimit = &cpu
		job.MemoryLimit = &mem
	}

	// Insert job into database
	if err := a.store.CreateJob(r.Context(), job); err != nil {
		a.logger.Error("failed to create job", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create job",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Push to queue
	if err := a.queue.Push(r.Context(), job.ID.String()); err != nil {
		a.logger.Error("failed to push job to queue", "job_id", job.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to queue job",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	a.logger.Info("job created", "job_id", job.ID, "image", job.Image)

	// Return response
	writeJSON(w, http.StatusCreated, toJobResponse(job, a.baseURL))
}

// HandleGetJob handles GET /v1/jobs/{id}
func (a *API) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := uuid.Parse(strings.TrimPrefix(r.URL.Path, "/v1/jobs/"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	job, err := a.store.GetJob(r.Context(), jobID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Job not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	writeJSON(w, http.StatusOK, toJobResponse(job, a.baseURL))
}

// HandleListJobs handles GET /v1/jobs
func (a *API) HandleListJobs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	var status *domain.JobStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		s := domain.JobStatus(statusStr)
		status = &s
	}

	var createdBy *string
	if cb := r.URL.Query().Get("created_by"); cb != "" {
		createdBy = &cb
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get jobs
	jobs, err := a.store.ListJobs(r.Context(), status, createdBy, limit, offset)
	if err != nil {
		a.logger.Error("failed to list jobs", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to list jobs",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Convert to response
	jobResponses := make([]JobResponse, len(jobs))
	for i, job := range jobs {
		jobResponses[i] = toJobResponse(job, a.baseURL)
	}

	writeJSON(w, http.StatusOK, ListJobsResponse{
		Jobs:   jobResponses,
		Total:  len(jobResponses),
		Limit:  limit,
		Offset: offset,
	})
}

// HandleGetLogs handles GET /v1/jobs/{id}/logs
func (a *API) HandleGetLogs(w http.ResponseWriter, r *http.Request) {
	jobID, err := uuid.Parse(getJobIDFromPath(r.URL.Path))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	job, err := a.store.GetJob(r.Context(), jobID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Job not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	if job.LogObjectKey == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Logs not yet available",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Check if presigned URL is requested
	if r.URL.Query().Get("presigned") == "true" {
		url, err := a.storage.PresignedURL(r.Context(), *job.LogObjectKey, 15*time.Minute)
		if err != nil {
			a.logger.Error("failed to generate presigned URL", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to generate download URL",
				Code:    http.StatusInternalServerError,
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"url": url})
		return
	}

	// Stream logs
	reader, err := a.storage.Download(r.Context(), *job.LogObjectKey)
	if err != nil {
		a.logger.Error("failed to download logs", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to download logs",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

// HandleListArtefacts handles GET /v1/jobs/{id}/artefacts
func (a *API) HandleListArtefacts(w http.ResponseWriter, r *http.Request) {
	jobID, err := uuid.Parse(getJobIDFromPath(r.URL.Path))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	artefacts, err := a.store.GetArtefacts(r.Context(), jobID)
	if err != nil {
		a.logger.Error("failed to get artefacts", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to get artefacts",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Convert to response
	artefactResponses := make([]ArtefactResponse, len(artefacts))
	for i, art := range artefacts {
		downloadURL := fmt.Sprintf("%s/v1/jobs/%s/artefacts/%s", a.baseURL, jobID, art.Path)
		artefactResponses[i] = ArtefactResponse{
			Path:        art.Path,
			SizeBytes:   art.SizeBytes,
			ContentType: art.ContentType,
			DownloadURL: downloadURL,
		}
	}

	writeJSON(w, http.StatusOK, ListArtefactsResponse{
		JobID:     jobID,
		Artefacts: artefactResponses,
	})
}

// HandleDownloadArtefact handles GET /v1/jobs/{id}/artefacts/{path}
func (a *API) HandleDownloadArtefact(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.Path, "/artefacts/", 2)
	if len(parts) != 2 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_path",
			Message: "Invalid artefact path",
			Code:    http.StatusBadRequest,
		})
		return
	}

	jobID, err := uuid.Parse(getJobIDFromPath(parts[0]))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid job ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	artefactPath := parts[1]

	// Get artefact metadata
	artefacts, err := a.store.GetArtefacts(r.Context(), jobID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to get artefacts",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	var targetArtefact *domain.Artefact
	for _, art := range artefacts {
		if art.Path == artefactPath {
			targetArtefact = &art
			break
		}
	}

	if targetArtefact == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Artefact not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	// Check if presigned URL is requested
	if r.URL.Query().Get("presigned") == "true" {
		url, err := a.storage.PresignedURL(r.Context(), targetArtefact.ObjectKey, 15*time.Minute)
		if err != nil {
			a.logger.Error("failed to generate presigned URL", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to generate download URL",
				Code:    http.StatusInternalServerError,
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"url": url})
		return
	}

	// Stream artefact
	reader, err := a.storage.Download(r.Context(), targetArtefact.ObjectKey)
	if err != nil {
		a.logger.Error("failed to download artefact", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to download artefact",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", targetArtefact.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(targetArtefact.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

// HandleHealthz handles GET /healthz
func (a *API) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleReadyz handles GET /readyz
func (a *API) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	// Check database
	if err := a.queue.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "redis unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getJobIDFromPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/v1/jobs/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// validateEnvVars checks that user-provided env vars don't use reserved names.
// Environment variables starting with "SWITCHYARD_" are reserved for system use
// and cannot be set by users.
func validateEnvVars(env map[string]string) error {
	const reservedPrefix = "SWITCHYARD_"

	for key := range env {
		if strings.HasPrefix(key, reservedPrefix) {
			return fmt.Errorf("environment variable '%s' is reserved (variables starting with '%s' are system-managed)", key, reservedPrefix)
		}
	}

	return nil
}
