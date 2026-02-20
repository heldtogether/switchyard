# Switchyard Architecture

## System Overview

Switchyard is a job execution platform that runs containerized workloads on Docker Swarm with plans for Kubernetes support.

```
┌─────────────┐
│   Client    │
│  (curl/CLI) │
└──────┬──────┘
       │ HTTP + API Key
       ▼
┌─────────────────────────────────────────────┐
│            API Service (2 replicas)          │
│  - Validates requests                        │
│  - Inserts job to Postgres                   │
│  - Pushes job ID to Redis queue              │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
          ┌────────────────┐
          │  Redis Queue   │
          │  (job IDs)     │
          └────────┬───────┘
                   │ BRPOP
                   ▼
┌─────────────────────────────────────────────┐
│          Worker Service (2 replicas)         │
│  1. Pop job ID from Redis                    │
│  2. Fetch job from Postgres                  │
│  3. Create Swarm service with:               │
│     - Isolated network                       │
│     - NFS volume mount                       │
│     - Resource limits                        │
│  4. Wait for completion (poll status)        │
│  5. Collect logs → S3                        │
│  6. Collect outputs from NFS → S3            │
│  7. Update job status in Postgres            │
│  8. Cleanup Swarm resources                  │
└─────────────────────────────────────────────┘
                   │
                   ▼
          ┌────────────────┐
          │ Docker Swarm   │
          │  Job Services  │
          │                │
          │  ┌──────────┐  │
          │  │  Job     │  │
          │  │Container │  │
          │  │          │  │
          │  │ /outputs │◄─┼─── NFS Mount
          │  └──────────┘  │
          └────────────────┘
                   │
                   ▼
          ┌────────────────┐
          │   NFS Share    │
          │  /mnt/jobrunner│
          │  ├── jobs/     │
          │  │   └── {id}/ │
          │  │       └──outputs/│
          └────────────────┘
                   │
                   │ Worker uploads
                   ▼
          ┌────────────────┐
          │  S3 Storage    │
          │  bucket:       │
          │  jobrunner     │
          │  ├── jobs/{id}/│
          │  │   ├── logs.txt│
          │  │   └── outputs/│
          └────────────────┘
```

## Component Responsibilities

### API Service
- **Purpose**: Accept job submissions, provide query interface
- **Language**: Go
- **HTTP Server**: net/http with custom routing
- **Authentication**: API key (X-API-Key header)
- **Endpoints**:
  - `POST /v1/jobs` - Submit new job
  - `GET /v1/jobs` - List jobs (paginated, filtered)
  - `GET /v1/jobs/{id}` - Get job details
  - `GET /v1/jobs/{id}/logs` - Download/stream logs
  - `GET /v1/jobs/{id}/artefacts` - List artefacts
  - `GET /v1/jobs/{id}/artefacts/{path}` - Download artefact
  - `POST /v1/jobs/{id}/cancel` - Cancel running job
  - `GET /healthz` - Health check
  - `GET /readyz` - Readiness check

### Worker Service
- **Purpose**: Execute jobs, collect outputs
- **Language**: Go
- **Main Loop**: BRPOP from Redis (blocking wait)
- **Concurrency**: Configurable (default: 5 parallel jobs)
- **Job Lifecycle**:
  1. Pop job ID from queue
  2. Fetch job details from Postgres (SELECT ... FOR UPDATE)
  3. Update status to RUNNING
  4. Call Executor.CreateRun()
  5. Wait for completion (with timeout)
  6. Collect logs and upload to S3
  7. Walk NFS outputs directory, upload to S3
  8. Update job status (SUCCEEDED/FAILED/TIMEOUT)
  9. Cleanup executor resources

### Executor Layer
- **Interface**: `internal/executor/executor.go`
- **Implementations**:
  - **SwarmExecutor**: Docker Swarm (complete)
  - **KubeExecutor**: Kubernetes (stub for future)

**Swarm Executor Details**:
- Creates one-shot service per job
- Service naming: `job-{jobID}`
- Labels: `jobrunner.job_id`, `jobrunner.managed=true`
- Network: Creates isolated overlay network per job
- Mounts: Binds `/mnt/jobrunner/jobs/{jobID}/outputs` → `/outputs`
- Resources: Enforces CPU/memory limits from config or job spec
- Timeout: Worker enforces via context.WithTimeout
- Logs: Collected via Docker ServiceLogs API
- Cleanup: Removes service and network

### Storage Layer

**Postgres**:
- **Tables**: jobs, artefacts, registry_secrets
- **Connection Pool**: Configurable (default: 25 max open, 5 idle)
- **Jobs Table**:
  - Stores full job specification
  - Status tracking with timestamps
  - JSONB for flexible fields (env, command, outputs, metadata)
  - Foreign key to registry_secrets (optional)
- **Artefacts Table**:
  - One row per output file
  - Links job_id → S3 object_key
  - Tracks size and content type

**Redis**:
- **Purpose**: Job queue
- **Data Structure**: List (LPUSH/BRPOP)
- **Queue Name**: `jobrunner:jobs` (configurable)
- **Why Redis**: Better for multiple workers than Postgres polling

**S3 (or compatible)**:
- **Purpose**: Logs and artefact storage
- **Bucket**: `jobrunner` (configurable)
- **Layout**:
  ```
  jobs/
    {job_id}/
      logs.txt
      metadata.json (future)
      outputs/
        result.txt
        data/
          metrics.json
  ```
- **Operations**:
  - Upload with streaming (logs, artefacts)
  - Presigned URLs (for downloads without proxying)
  - List with prefix (for artefact enumeration)

**NFS**:
- **Purpose**: Shared filesystem for job outputs
- **Mount Point**: `/mnt/jobrunner` (configurable via `executor.swarm.nfs_base_path`)
- **Layout**:
  ```
  /mnt/jobrunner/
    jobs/
      {job_id}/
        outputs/  ← Job containers write here
  ```
- **Critical**: Must be mounted on ALL Swarm nodes at the SAME path

## Data Flow

### Job Submission Flow
1. Client → `POST /v1/jobs` → API
2. API validates request (image, outputs, limits)
3. API generates UUID for job
4. API inserts job to Postgres (status=PENDING)
5. API pushes job UUID to Redis queue
6. API returns 201 Created with job ID

### Job Execution Flow
1. Worker BRPOPs from Redis (blocking)
2. Worker receives job UUID
3. Worker fetches job from Postgres with row lock
4. Worker updates status → RUNNING, started_at = NOW()
5. Worker creates NFS output directory: `/mnt/jobrunner/jobs/{id}/outputs`
6. Worker calls Executor.CreateRun():
   - Creates Swarm service with spec
   - Mounts NFS path → `/outputs` in container
   - Attaches to isolated network
   - Sets resource limits
   - Returns service ID (RunRef)
7. Worker calls Executor.Wait(RunRef) with timeout context
   - Polls service status every 2 seconds
   - Returns when service completes or context times out
8. On completion:
   - Worker calls Executor.GetLogs() → uploads to S3
   - Worker calls Executor.CollectOutputs():
     - Walks `/mnt/jobrunner/jobs/{id}/outputs`
     - Uploads each file to S3
     - Returns list of artefacts
   - Worker saves artefacts to Postgres
   - Worker updates job status (SUCCEEDED/FAILED/TIMEOUT)
   - Worker calls Executor.Cleanup() (removes service, network)

### Log/Artefact Retrieval Flow
1. Client → `GET /v1/jobs/{id}/logs` → API
2. API fetches job from Postgres
3. API checks if `log_object_key` is set
4. API generates presigned S3 URL
5. API returns 302 redirect or proxies content

## Configuration

All configuration is centralized in `config.yaml` with environment variable overrides.

**Critical Configuration Points**:

1. **NFS Path** (`executor.swarm.nfs_base_path`):
   ```yaml
   executor:
     swarm:
       nfs_base_path: "/mnt/jobrunner"  # ← Must match NFS mount
   ```
   
2. **Resource Defaults** (`executor.swarm.defaults.resources`):
   ```yaml
   executor:
     swarm:
       defaults:
         resources:
           cpu: "1.0"
           memory: "2g"
         timeout: "1h"
   ```

3. **S3 Credentials** (`storage`):
   ```yaml
   storage:
     endpoint: "https://s3.amazonaws.com"
     access_key: "${S3_ACCESS_KEY}"
     secret_key: "${S3_SECRET_KEY}"
   ```

## Security Model

**MVP Security**:
- API key authentication (Bearer token or X-API-Key header)
- Network isolation (jobs cannot reach internet)
- Resource limits (prevent resource exhaustion)
- Docker socket access limited to worker containers

**Extension Points for Production**:
- SSO/OIDC: Replace API key middleware in `internal/api/middleware.go`
- RBAC: Add `permissions` JSONB column to jobs table
- Image scanning: Hook into CreateRun() to scan images before execution
- Secrets management: Integrate Vault for registry credentials
- Audit logging: Add audit table and middleware
- mTLS: Add TLS config to API server

## Scalability Considerations

**Current Limits**:
- Worker concurrency: 5 jobs/worker (configurable)
- API replicas: 2 (stateless, can scale horizontally)
- Worker replicas: 2 (can scale horizontally)
- Postgres connection pool: 25 connections

**Scaling Strategies**:
- **More jobs/sec**: Increase worker replicas + concurrency
- **Larger jobs**: Adjust per-job resource limits
- **More storage**: Use S3 lifecycle policies for old artefacts
- **Geographic distribution**: Deploy multiple clusters, federate via API gateway

**Bottlenecks to Watch**:
- NFS throughput (many jobs writing outputs concurrently)
- Redis memory (long queue buildup)
- Postgres connections (many workers + API instances)
- Swarm manager capacity (service creation rate)

## Recovery & Resilience

**Worker Crash Recovery**:
- On startup, worker queries `SELECT * FROM jobs WHERE status = 'RUNNING'`
- For each orphaned job:
  - Query executor status via RunRef
  - If service no longer exists → mark job FAILED
  - If service still running → continue monitoring

**Job Timeout Handling**:
- Worker creates context with timeout from job spec
- Context cancellation triggers:
  - Executor.Cancel() (removes service)
  - Job status → TIMEOUT
  - Logs still collected (partial)

**Failed Job Cleanup**:
- Config option: `cleanup.keep_failed_services`
- If false: Remove service immediately
- If true: Keep service for debugging (manual cleanup required)

## Monitoring & Observability

**Structured Logging**:
- Format: JSON (configurable to console for dev)
- Fields: `job_id`, `service`, `action`, `error`
- Output: stdout (captured by Docker)

**Health Checks**:
- `/healthz`: Basic liveness (always returns 200)
- `/readyz`: Checks DB + Redis connectivity

**Metrics** (future):
- Jobs submitted/sec
- Jobs completed/sec
- Job duration histogram
- Queue depth
- Worker utilization
- Executor errors

## Future Enhancements

**Phase 2 (Kubernetes)**:
- Implement KubeExecutor
- Use PersistentVolumeClaims instead of NFS
- Pod-based execution with sidecars for output collection

**Phase 3 (Workflows)**:
- DAG support (job dependencies)
- Conditional execution
- Retries with backoff

**Phase 4 (Advanced Features)**:
- Job templates
- Scheduled/cron jobs
- Result caching (content-addressable)
- GPU support
- Multi-region job dispatch

## Design Decisions & Rationale

**Why Redis queue instead of Postgres SKIP LOCKED?**
- Better performance for high-throughput job dispatch
- Atomic pop semantics
- Multiple workers without lock contention

**Why NFS for artefacts?**
- Simplest shared storage for Swarm
- Worker can directly access job outputs without sidecars
- Alternative: Use Docker volumes + sidecar containers (more complex)

**Why Swarm services instead of `docker run`?**
- Better scheduling (constraints, placement)
- Native log collection via ServiceLogs API
- Health checks and restart policies
- Labels for resource tracking

**Why Postgres for job state?**
- ACID guarantees for status transitions
- Rich querying (filter by status, user, date)
- Foreign keys for data integrity
- JSON support for flexible schemas

**Why presigned URLs instead of proxying S3?**
- Offload bandwidth from API service
- Client directly downloads from S3
- Lower latency

**Why separate API and Worker services?**
- Different scaling characteristics (API = CPU, Worker = I/O)
- Worker needs Docker socket access (security boundary)
- Can deploy workers closer to Swarm managers
