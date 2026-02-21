# Switchyard Architecture

## System Overview
Switchyard is a job execution platform that runs containerized workloads on Docker Swarm with plans for Kubernetes support. Jobs are submitted to the API, queued in Redis, executed by workers, and stored in Postgres/S3.

```
Client -> API -> Redis queue -> Worker -> Executor -> Docker Swarm
                         \-> Postgres (metadata)
                         \-> S3 (logs/artefacts)
                         \-> NFS (shared outputs)
```

## Component Responsibilities

### API Service
- Accepts job submissions and query requests
- Validates input (including reserved `SWITCHYARD_` env prefix)
- Persists job metadata to Postgres
- Enqueues jobs in Redis

### Worker Service
- Dequeues job IDs from Redis
- Loads job + hierarchy metadata (workspace/project/run)
- Creates executor runs and waits for completion
- Uploads logs and artefacts to S3
- Updates job status in Postgres

### Executor Layer
- Interface: `internal/executor/executor.go`
- Shared utilities in `internal/executor/common.go`
- Implementations:
  - `internal/executor/docker/` (Docker containers)
  - `internal/executor/swarm/` (Swarm services)
  - `internal/executor/kube/` (stub)

### Storage Layer
- **Postgres**: job metadata, artefacts, registry secrets
- **Redis**: job queue (LPUSH/BRPOP)
- **S3**: logs and artefacts storage
- **NFS**: shared output directory mounted into job containers

## Interfaces and Contracts
- Jobs are identified by UUID and stored in Postgres as the source of truth.
- Redis stores only job IDs; workers rehydrate full specs from Postgres.
- Logs and artefacts are uploaded under predictable S3 prefixes per job.

For endpoint details and request/response payloads, see the API handlers in `internal/api/`.
