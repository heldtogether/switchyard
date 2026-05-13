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
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/service-accounts", s.api.HandleCreateServiceAccount)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/service-accounts", s.api.HandleListServiceAccounts)
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/service-accounts/{service_account_id}/keys", s.api.HandleCreateServiceAccountKey)
	mux.HandleFunc("DELETE /v1/workspaces/{workspace_slug}/service-accounts/{service_account_id}/keys/{key_id}", s.api.HandleRevokeServiceAccountKey)
	mux.HandleFunc("DELETE /v1/workspaces/{workspace_slug}/service-accounts/{service_account_id}", s.api.HandleDisableServiceAccount)
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

// OpenAPI operation annotations
// @Summary Start OIDC login
// @Description Starts OIDC login flow.
// @Tags auth
// @Success 302 {string} string "Redirect"
// @Failure 404 {object} ErrorResponse
// @Router /v1/auth/login [get]
func openapiAuthLogin() {}

// @Summary Complete OIDC login callback
// @Description Completes OIDC login flow.
// @Description Default behavior sets session cookie and redirects.
// @Description Use `format=json` to receive a bearer token payload for Authorization header usage.
// @Tags auth
// @Param format query string false "Set to `json` to return bearer token response instead of redirect"
// @Success 200 {object} AuthCallbackTokenResponse
// @Success 302 {string} string "Redirect"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /v1/auth/callback [get]
func openapiAuthCallback() {}

// @Summary Logout current session
// @Tags auth
// @Success 302 {string} string "Redirect"
// @Router /v1/auth/logout [get]
func openapiAuthLogout() {}

// @Summary Get current principal
// @Tags auth
// @Success 200 {object} AuthMeResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/auth/me [get]
func openapiAuthMe() {}

// @Summary Create workspace
// @Tags workspaces
// @Description Creates a workspace and returns its persisted record.
// @Description Slug must be lowercase letters, numbers, and hyphens.
// @Description Reserved slugs are rejected.
// @Accept json
// @Param body body CreateWorkspaceRequest true "Workspace"
// @Success 201 {object} WorkspaceResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces [post]
func openapiCreateWorkspace() {}

// @Summary List workspaces
// @Tags workspaces
// @Param limit query int false "Limit (default 50, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} WorkspacesListResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces [get]
func openapiListWorkspaces() {}

// @Summary Get workspace
// @Tags workspaces
// @Param slug path string true "Workspace slug"
// @Success 200 {object} WorkspaceResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{slug} [get]
func openapiGetWorkspace() {}

// @Summary Create workspace invite
// @Tags access
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param body body CreateInviteRequest true "Invite"
// @Success 201 {object} CreateInviteResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/invites [post]
func openapiCreateWorkspaceInvite() {}

// @Summary Accept workspace invite
// @Tags access
// @Accept json
// @Param body body AcceptInviteRequest true "Invite acceptance"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspace-invites/accept [post]
func openapiAcceptWorkspaceInvite() {}

// @Summary List workspace members
// @Tags access
// @Param workspace_slug path string true "Workspace slug"
// @Success 200 {object} MembersListResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/members [get]
func openapiListWorkspaceMembers() {}

// @Summary Create registry secret
// @Tags registry-secrets
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param body body CreateRegistrySecretRequest true "Registry secret"
// @Success 201 {object} RegistrySecretResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/registry-secrets [post]
func openapiCreateRegistrySecret() {}

// @Summary List registry secrets
// @Tags registry-secrets
// @Param workspace_slug path string true "Workspace slug"
// @Success 200 {object} RegistrySecretsListResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/registry-secrets [get]
func openapiListRegistrySecrets() {}

// @Summary Deactivate registry secret
// @Tags registry-secrets
// @Param workspace_slug path string true "Workspace slug"
// @Param secret_id path string true "Secret ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id} [delete]
func openapiDeleteRegistrySecret() {}

// @Summary Rotate registry secret
// @Tags registry-secrets
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param secret_id path string true "Secret ID"
// @Param body body RotateRegistrySecretRequest true "Rotation payload"
// @Success 200 {object} RegistrySecretResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/registry-secrets/{secret_id}/rotate [post]
func openapiRotateRegistrySecret() {}

// @Summary Create project
// @Tags projects
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param body body CreateProjectRequest true "Project"
// @Success 201 {object} ProjectResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects [post]
func openapiCreateProject() {}

// @Summary List projects
// @Tags projects
// @Param workspace_slug path string true "Workspace slug"
// @Param limit query int false "Limit (default 50, max 100)"
// @Param offset query int false "Offset"
// @Param include_archived query bool false "Include archived projects"
// @Success 200 {object} ProjectsListResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects [get]
func openapiListProjects() {}

// @Summary Get project
// @Tags projects
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Success 200 {object} ProjectResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug} [get]
func openapiGetProject() {}

// @Summary Update project
// @Tags projects
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param body body UpdateProjectRequest true "Project patch"
// @Success 200 {object} ProjectResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug} [put]
func openapiUpdateProject() {}

// @Summary Archive project
// @Tags projects
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Success 200 {object} MessageResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/archive [post]
func openapiArchiveProject() {}

// @Summary Create project invite
// @Tags access
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param body body CreateInviteRequest true "Invite"
// @Success 201 {object} CreateInviteResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/invites [post]
func openapiCreateProjectInvite() {}

// @Summary Accept project invite
// @Tags access
// @Accept json
// @Param body body AcceptInviteRequest true "Invite acceptance"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/project-invites/accept [post]
func openapiAcceptProjectInvite() {}

// @Summary List project members
// @Tags access
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Success 200 {object} MembersListResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/members [get]
func openapiListProjectMembers() {}

// @Summary Create run
// @Tags runs
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param body body CreateRunRequest true "Run"
// @Success 201 {object} RunResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs [post]
func openapiCreateRun() {}

// @Summary List runs
// @Tags runs
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param limit query int false "Limit (default 50, max 100)"
// @Param offset query int false "Offset"
// @Param status query string false "Run status"
// @Success 200 {object} RunsListResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs [get]
func openapiListRuns() {}

// @Summary Get run
// @Tags runs
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Success 200 {object} RunResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug} [get]
func openapiGetRun() {}

// @Summary Rerun run
// @Tags runs
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param body body RerunRunRequest true "Rerun request"
// @Success 201 {object} RerunRunResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/rerun [post]
func openapiRerunRun() {}

// @Summary Cancel run
// @Tags runs
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Success 200 {object} CancelRunResponse
// @Success 202 {object} CancelRunResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/cancel [post]
func openapiCancelRun() {}

// @Summary Get run billing breakdown
// @Tags billing
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Success 200 {object} RunBillingBreakdownResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/billing [get]
func openapiGetRunBillingBreakdown() {}

// @Summary Create promotion
// @Tags promotions
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param body body CreatePromotionRequest true "Promotion"
// @Success 201 {object} PromotionEventResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions [post]
func openapiCreatePromotion() {}

// @Summary List current promotions
// @Tags promotions
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Success 200 {object} ListCurrentPromotionsResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions [get]
func openapiListCurrentPromotions() {}

// @Summary List promotion history
// @Tags promotions
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param limit query int false "Limit (default 50, max 200)"
// @Param offset query int false "Offset"
// @Param channel query string false "Promotion channel filter"
// @Success 200 {object} PromotionHistoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions/history [get]
func openapiListPromotionHistory() {}

// @Summary Get current promotion by channel
// @Tags promotions
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param channel path string true "Promotion channel"
// @Success 200 {object} CurrentPromotionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions/{channel} [get]
func openapiGetCurrentPromotionByChannel() {}

// @Summary Resolve promoted artefact
// @Tags promotions
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param channel path string true "Promotion channel"
// @Param logical_key path string true "Logical artefact key"
// @Success 200 {object} ResolvedPromotedArtefactResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions/{channel}/artefacts/{logical_key} [get]
func openapiResolvePromotedArtefact() {}

// @Summary Create job
// @Tags jobs
// @Accept json
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param body body CreateJobRequest true "Job"
// @Success 201 {object} JobResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs [post]
func openapiCreateJob() {}

// @Summary List jobs
// @Tags jobs
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param limit query int false "Limit (default 50, max 100)"
// @Param offset query int false "Offset"
// @Param status query string false "Job status"
// @Success 200 {object} JobsListResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs [get]
func openapiListJobs() {}

// @Summary Get job
// @Tags jobs
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param job_id path string true "Job ID"
// @Success 200 {object} JobResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id} [get]
func openapiGetJob() {}

// @Summary Get job logs
// @Tags jobs
// @Produce plain
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param job_id path string true "Job ID"
// @Success 200 {string} string "Logs"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/logs [get]
func openapiGetJobLogs() {}

// @Summary Cancel job
// @Tags jobs
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param job_id path string true "Job ID"
// @Success 200 {object} JobResponse
// @Success 202 {object} JobResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/cancel [post]
func openapiCancelJob() {}

// @Summary List job artefacts
// @Tags artefacts
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param job_id path string true "Job ID"
// @Success 200 {object} ListArtefactsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts [get]
func openapiListArtefacts() {}

// @Summary Download job artefact
// @Tags artefacts
// @Produce application/octet-stream
// @Param workspace_slug path string true "Workspace slug"
// @Param project_slug path string true "Project slug"
// @Param run_slug path string true "Run slug"
// @Param job_id path string true "Job ID"
// @Param path path string true "Artefact path"
// @Success 200 {string} string "Artefact content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/jobs/{job_id}/artefacts/{path} [get]
func openapiDownloadArtefact() {}

// @Summary Get workspace month-to-date billing
// @Tags billing
// @Param workspace_slug path string true "Workspace slug"
// @Success 200 {object} WorkspaceMonthToDateBillingResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/workspaces/{workspace_slug}/billing/month-to-date [get]
func openapiGetWorkspaceMonthToDateBilling() {}
