# Switchyard - Docker Job Runner Platform

A job execution platform for running containerized workloads on Docker Swarm (with Kubernetes support planned).

## Overview
Switchyard accepts container jobs over an HTTP API, executes them on a Swarm cluster, and stores logs and artefacts in S3-compatible storage. Jobs are organized hierarchically: Workspace → Project → Run → Jobs.

## Key Features
- Hierarchical job organization (workspace/project/run/job)
- Swarm and Docker executors with shared utilities
- Per-job resource limits (CPU, memory, GPU) and timeouts
- GPU-aware scheduling with per-job allocations and node capacity tracking
- RabbitMQ or Redis queueing (RabbitMQ recommended for GPU routing)
- Postgres metadata store + S3-compatible log and artefact storage
- Hybrid authentication (API key + OIDC SSO)

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
Then open `http://localhost:5173`.

### UI (Docker)
```bash
docker build -f build/ui.Dockerfile -t switchyard-ui:latest .
docker run -p 3000:80 \
  -e UI_API_BASE_URL=http://localhost:8080 \
  -e UI_AUTH_LOGIN_URL=http://localhost:8080/v1/auth/login \
  -e UI_AUTH_LOGOUT_URL=http://localhost:8080/v1/auth/logout \
  -e UI_WORKSPACE_SLUG=default \
  -e UI_USE_MOCKS=false \
  switchyard-ui:latest
```
For deployment and ops, see `deployments/README.md` and `deployments/DEPLOYMENT.md`.

## Repository Structure
```
cmd/            # Service entrypoints
internal/       # Core packages (api, worker, executor, storage, config, domain)
migrations/     # SQL migrations
deployments/    # Swarm/Compose configs and docs
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
- RBAC is configurable under `api.rbac` (workspace + project memberships, token invites, and service-account allowlists for API key auth).

## Documentation Map
- `AGENTS.md`: contributor guide and repo conventions
- `ARCHITECTURE.md`: system and component overview
- `TESTING.md`: how to run tests and current expectations
- `deployments/README.md`: deployment file index and quick ops notes
- `deployments/DEPLOYMENT.md`: full Swarm deployment guide
- `build/README.md`: Docker image build details
