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
	mux.HandleFunc("GET /v1/auth/login", s.api.HandleAuthLogin)
	mux.HandleFunc("GET /v1/auth/callback", s.api.HandleAuthCallback)
	mux.HandleFunc("GET /v1/auth/logout", s.api.HandleAuthLogoutRedirect)
	mux.HandleFunc("GET /v1/auth/me", s.api.HandleAuthMe)

	// Workspace routes
	mux.HandleFunc("POST /v1/workspaces", s.api.HandleCreateWorkspace)
	mux.HandleFunc("GET /v1/workspaces", s.api.HandleListWorkspaces)
	mux.HandleFunc("GET /v1/workspaces/{slug}", s.api.HandleGetWorkspace)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/invites", s.api.HandleCreateWorkspaceInvite)
	mux.HandleFunc("POST /v1/workspace-invites/accept", s.api.HandleAcceptWorkspaceInvite)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/members", s.api.HandleListWorkspaceMembers)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/registry-secrets", s.api.HandleCreateRegistrySecret)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/registry-secrets", s.api.HandleListRegistrySecrets)
	mux.HandleFunc("DELETE /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id}", s.api.HandleDeleteRegistrySecret)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id}/rotate", s.api.HandleRotateRegistrySecret)

	// Project routes
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects", s.api.HandleCreateProject)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects", s.api.HandleListProjects)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}", s.api.HandleGetProject)
	mux.HandleFunc("PUT /v1/workspaces/{workspace_slug}/projects/{project_slug}", s.api.HandleUpdateProject)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/archive", s.api.HandleArchiveProject)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/invites", s.api.HandleCreateProjectInvite)
	mux.HandleFunc("POST /v1/project-invites/accept", s.api.HandleAcceptProjectInvite)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/members", s.api.HandleListProjectMembers)

	// Run routes
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs", s.api.HandleCreateRun)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs", s.api.HandleListRuns)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}", s.api.HandleGetRun)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/rerun", s.api.HandleRerunRun)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/cancel", s.api.HandleCancelRun)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/billing", s.api.HandleRunBillingBreakdown)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions", s.api.HandleCreatePromotion)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions", s.api.HandleListCurrentPromotions)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions/history", s.api.HandleListPromotionHistory)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions/{channel}", s.api.HandleGetCurrentPromotionByChannel)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions/{channel}/artefacts/{logical_key}", s.api.HandleResolvePromotedArtefact)

	// Job routes
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs", s.api.HandleCreateJob)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs", s.api.HandleListJobs)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}", s.api.HandleGetJob)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/logs", s.api.HandleGetJobLogs)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/cancel", s.api.HandleCancelJob)

	// Worker routes
	mux.HandleFunc("POST /v1/workers/register", s.api.HandleRegisterWorker)
	mux.HandleFunc("POST /v1/workers/heartbeat", s.api.HandleWorkerHeartbeat)

	// Allocation routes
	mux.HandleFunc("POST /v1/allocations/claim", s.api.HandleClaimAllocation)
	mux.HandleFunc("POST /v1/allocations/release", s.api.HandleReleaseAllocation)
	mux.HandleFunc("GET /v1/allocations/capacity", s.api.HandleGetAllocationCapacity)

	// Artefact routes
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts", s.api.HandleListArtefacts)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts/{path...}", s.api.HandleDownloadArtefact)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/billing/month-to-date", s.api.HandleWorkspaceMonthToDateBilling)

	// Wrap with middlewares
	var handler http.Handler = mux

	// Apply middlewares (in reverse order of execution)
	handler = RequestIDMiddleware()(handler)
	handler = LoggingMiddleware(s.logger)(handler)
	handler = RecoveryMiddleware(s.logger)(handler)

	authManager, err := NewAuthManager(s.cfg, s.logger)
	if err != nil {
		return fmt.Errorf("initialize auth manager: %w", err)
	}
	s.api.SetAuthManager(authManager)

	// Add auth middleware if enabled
	if authManager.Enabled() {
		handler = authManager.Middleware(handler)
	}

	// Add CORS middleware (wrap outermost so OPTIONS preflight is handled)
	handler = CORSMiddleware(s.cfg.API.CORSAllowedOrigins)(handler)

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
