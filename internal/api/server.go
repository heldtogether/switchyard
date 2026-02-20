package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/heldtogether/switchyard/internal/config"
)

// Server is the HTTP server
type Server struct {
	api        *API
	httpServer *http.Server
	logger     *slog.Logger
	cfg        *config.Config
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, api *API, logger *slog.Logger) *Server {
	return &Server{
		api:    api,
		cfg:    cfg,
		logger: logger,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/v1/jobs", s.handleJobs)
	mux.HandleFunc("/v1/jobs/", s.handleJobsWithID)
	mux.HandleFunc("/healthz", s.api.HandleHealthz)
	mux.HandleFunc("/readyz", s.api.HandleReadyz)

	// Wrap with middlewares
	var handler http.Handler = mux

	// Apply middlewares (in reverse order of execution)
	handler = RequestIDMiddleware()(handler)
	handler = LoggingMiddleware(s.logger)(handler)
	handler = RecoveryMiddleware(s.logger)(handler)

	// Add auth middleware if enabled
	if s.cfg.API.Auth.Enabled {
		handler = AuthMiddleware(s.cfg.API.Auth.APIKey, s.logger)(handler)
	}

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", s.cfg.API.Host, s.cfg.API.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  s.cfg.API.ReadTimeout,
		WriteTimeout: s.cfg.API.WriteTimeout,
	}

	s.logger.Info("starting http server", "addr", addr)

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping http server")
	return s.httpServer.Shutdown(ctx)
}

// handleJobs routes /v1/jobs based on method
func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.api.HandleCreateJob(w, r)
	case http.MethodGet:
		s.api.HandleListJobs(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error:   "method_not_allowed",
			Message: "Method not allowed",
			Code:    http.StatusMethodNotAllowed,
		})
	}
}

// handleJobsWithID routes /v1/jobs/{id}/* based on path and method
func (s *Server) handleJobsWithID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Check if it's cancel
	if strings.HasSuffix(path, "/cancel") {
		if r.Method == http.MethodPost {
			s.api.HandleCancelJob(w, r)
		} else {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
				Error:   "method_not_allowed",
				Message: "Method not allowed",
				Code:    http.StatusMethodNotAllowed,
			})
		}
		return
	}

	// Check if it's artefacts
	if strings.Contains(path, "/artefacts") {
		if strings.HasSuffix(path, "/artefacts") {
			// List artefacts
			s.api.HandleListArtefacts(w, r)
		} else {
			// Download specific artefact
			s.api.HandleDownloadArtefact(w, r)
		}
		return
	}

	// Check if it's logs
	if strings.HasSuffix(path, "/logs") {
		s.api.HandleGetLogs(w, r)
		return
	}

	// Just job ID - get job details
	if r.Method == http.MethodGet {
		s.api.HandleGetJob(w, r)
	} else {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error:   "method_not_allowed",
			Message: "Method not allowed",
			Code:    http.StatusMethodNotAllowed,
		})
	}
}
