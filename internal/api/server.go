package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

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

	// Health check routes
	mux.HandleFunc("/healthz", s.handleHealthCheck)
	mux.HandleFunc("/readyz", s.handleHealthCheck)

	// Workspace routes
	mux.HandleFunc("POST /v1/workspaces", s.api.HandleCreateWorkspace)
	mux.HandleFunc("GET /v1/workspaces", s.api.HandleListWorkspaces)
	mux.HandleFunc("GET /v1/workspaces/{slug}", s.api.HandleGetWorkspace)

	// Project routes
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects", s.api.HandleCreateProject)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects", s.api.HandleListProjects)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}", s.api.HandleGetProject)
	mux.HandleFunc("PUT /v1/workspaces/{workspace_slug}/projects/{project_slug}", s.api.HandleUpdateProject)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/archive", s.api.HandleArchiveProject)

	// Run routes
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs", s.api.HandleCreateRun)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs", s.api.HandleListRuns)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}", s.api.HandleGetRun)

	// Job routes
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs", s.api.HandleCreateJob)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs", s.api.HandleListJobs)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}", s.api.HandleGetJob)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/logs", s.api.HandleGetJobLogs)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/cancel", s.api.HandleCancelJob)

	// Artefact routes
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts", s.api.HandleListArtefacts)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts/{path...}", s.api.HandleDownloadArtefact)

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

// handleHealthCheck returns a simple health check
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
