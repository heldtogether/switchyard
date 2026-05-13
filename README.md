# Switchyard - Docker Job Runner Platform

A job execution platform for running containerized workloads on Docker (with Kubernetes support planned).

## Overview
Switchyard accepts container jobs over an HTTP API, executes them via Docker, and stores logs and artefacts in S3-compatible storage. Jobs are organized hierarchically: Workspace → Project → Run → Jobs.

## Key Features
- Hierarchical job organization (workspace/project/run/job)
- Docker executor with shared runtime utilities
- Per-job resource limits (CPU, memory, GPU) and timeouts
- GPU-aware scheduling with per-job allocations and node capacity tracking
- RabbitMQ or Redis queueing (RabbitMQ recommended for GPU routing)
- Postgres metadata store + S3-compatible log and artefact storage
- Hybrid authentication (API key + OIDC SSO)
- RBAC memberships with workspace/project invites and UI invite acceptance (`/accept-invite`)
- API-managed service account keys for CI/CD and machine-to-machine job submission
- Workspace switcher with in-app workspace creation for authenticated users
- Workspace-scoped registry secret lifecycle (create/list/deactivate/rotate) surfaced in workspace settings UI
- Registry secrets encrypted at rest with AES-256-GCM (decrypt only at worker pull time)

## Quick Start (Local Dev)
```bash
cp config.example.yaml config.yaml
make dev-up
make migrate-up
make build
./bin/api -config config.yaml
./bin/worker -config config.yaml
```
### UI (Local Dev)
```bash
cd ui
npm install
npm run dev
```
Then open `http://localhost:5173`. The sidebar version defaults to the nearest release tag plus the current short Git SHA, appending `-dirty` when the worktree has uncommitted changes. Clean tagged commits show the release tag only.

### UI (Docker)
```bash
docker build -f build/ui.Dockerfile -t switchyard-ui:latest .
docker run -p 3000:80 \
  -e UI_API_BASE_URL=http://localhost:8080 \
  -e UI_AUTH_LOGIN_URL=http://localhost:8080/v1/auth/login \
  -e UI_AUTH_LOGOUT_URL=http://localhost:8080/v1/auth/logout \
  -e UI_WORKSPACE_SLUG=default \
  -e UI_USE_MOCKS=false \
  -e UI_VERSION=v1.2.3 \
  switchyard-ui:latest
```
For deployment and ops, see `deployments/README.md` and `deployments/DEPLOYMENT.md`.

## Repository Structure
```
cmd/            # Service entrypoints
internal/       # Core packages (api, worker, executor, storage, config, domain)
migrations/     # SQL migrations
deployments/    # Deployment configs and docs
build/          # Dockerfiles and image docs
examples/       # Example jobs and helper scripts
```

## Configuration
- `config.example.yaml` documents available settings.
- GPU-aware scheduling requires workers to register and heartbeat (automatic on startup).
- RabbitMQ is the recommended queue for GPU routing via `gpu.N` topic keys.
- Avoid committing secrets; use env vars or Docker secrets for credentials.
- For OIDC with separate UI/API origins, set `api.auth.oidc.post_login_redirect` and `post_logout_redirect` to absolute UI URLs (for example `http://localhost:5173/` and `http://localhost:5173/login`).
- `api.auth.oidc.logout_url` is optional. When set, `GET /v1/auth/logout` clears the local session and redirects to that IdP logout URL.
- Preferred API auth is bearer token via `Authorization: Bearer <token>`. Obtain token from `GET /v1/auth/callback?format=json` after OIDC login; session cookie auth remains supported.
- RBAC is configurable under `api.rbac` (workspace + project memberships, token invites, legacy service-account allowlists for config API key auth, and database-backed service accounts).
- Create CI service account keys with `POST /v1/workspaces/{workspace_slug}/service-accounts`; store the one-time returned `key` as a GitHub Actions secret and send it as `Authorization: Bearer <key>`.
- Registry secret encryption is configured under `api.registry_secrets.encryption` (`REGISTRY_SECRETS_*` env vars, including `*_FILE` forms).
- To backfill legacy plaintext secrets after enabling encryption metadata migration, run: `go run ./cmd/secretmigrate -config config.yaml` (use `-dry-run` first).

### GitHub Actions Job Submission
Create a workspace-scoped service account as a workspace owner. `expires_at` is required, and `project_slugs` limits which projects the key can use:
```bash
curl -X POST "$API_URL/v1/workspaces/default/service-accounts" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"github-actions","expires_at":"2026-12-31T23:59:59Z","project_slugs":["test-project"]}'
```
Save the one-time `key` response as `SWITCHYARD_SERVICE_ACCOUNT_KEY` in GitHub Actions, then submit jobs with:
```yaml
- name: Submit Switchyard job
  env:
    API_URL: https://switchyard.example.com
    SWITCHYARD_SERVICE_ACCOUNT_KEY: ${{ secrets.SWITCHYARD_SERVICE_ACCOUNT_KEY }}
    WORKSPACE_SLUG: default
    PROJECT_SLUG: test-project
    RUN_SLUG: ci-${{ github.run_id }}
  run: ./examples/scripts/submit-job.sh
```

## Documentation Map
- `AGENTS.md`: contributor guide and repo conventions
- `ARCHITECTURE.md`: system and component overview
- `TESTING.md`: how to run tests and current expectations
- `build/README.md`: Docker image build details
