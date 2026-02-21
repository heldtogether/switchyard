# Switchyard - Docker Job Runner Platform

A job execution platform for running containerized workloads on Docker Swarm (with Kubernetes support planned).

## Overview
Switchyard accepts container jobs over an HTTP API, executes them on a Swarm cluster, and stores logs and artefacts in S3-compatible storage. Jobs are organized hierarchically: Workspace → Project → Run → Jobs.

## Key Features
- Hierarchical job organization (workspace/project/run/job)
- Swarm and Docker executors with shared utilities
- Resource limits and timeouts per job
- Redis-backed queue and Postgres metadata store
- S3-compatible log and artefact storage
- API key authentication

## Quick Start (Local Dev)
```bash
cp config.example.yaml config.yaml
make dev-up
make migrate-up
make build
./bin/api -config config.yaml
./bin/worker -config config.yaml
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
- Avoid committing secrets; use env vars or Docker secrets for credentials.

## Documentation Map
- `AGENTS.md`: contributor guide and repo conventions
- `ARCHITECTURE.md`: system and component overview
- `TESTING.md`: how to run tests and current expectations
- `deployments/README.md`: deployment file index and quick ops notes
- `deployments/DEPLOYMENT.md`: full Swarm deployment guide
- `build/README.md`: Docker image build details
