# Switchyard - Docker Job Runner Platform

[![Build and Push Docker Images](https://github.com/heldtogether/switchyard/actions/workflows/docker-build.yml/badge.svg)](https://github.com/heldtogether/switchyard/actions/workflows/docker-build.yml)

A production-ready job execution platform for running containerized workloads on Docker Swarm (with Kubernetes support planned).

## 🎯 Overview

Switchyard is an internal platform that:
- Accepts Docker container jobs via HTTP API
- Executes them on a Docker Swarm cluster with resource limits and timeout enforcement
- Captures logs and collects output artefacts
- Stores everything in S3-compatible object storage
- Provides job status tracking and artefact download

**Key Features:**
- ✅ Clean executor abstraction (Docker and Swarm implemented, Kubernetes ready)
- ✅ Automatic system environment variable injection (job context, metadata)
- ✅ Resource limits (CPU, memory) and timeout enforcement
- ✅ Network isolation (jobs run in isolated networks)
- ✅ NFS-based artefact collection with S3 upload
- ✅ Redis job queue for reliable job dispatch
- ✅ Postgres for job metadata and history
- ✅ S3-compatible storage for logs and artefacts
- ✅ API key authentication (extensible to SSO/OIDC)
- ✅ Private registry support

## 📋 Prerequisites

### Infrastructure Requirements

1. **Docker Swarm Cluster**
   - Initialized Swarm with manager and worker nodes
   - Docker API accessible to worker containers

2. **NFS Share** (Critical!)
   ```bash
   # On all Swarm nodes, mount NFS at the same path:
   sudo mkdir -p /mnt/jobrunner
   sudo mount -t nfs nfs-server.local:/export/jobrunner /mnt/jobrunner
   
   # Make permanent in /etc/fstab:
   nfs-server.local:/export/jobrunner  /mnt/jobrunner  nfs  defaults  0  0
   ```
   
   **⚠️ IMPORTANT**: This path must be:
   - Accessible on ALL Swarm nodes
   - Mounted at the SAME path on every node
   - Writable by UID/GID used by job containers (typically root or specific user)
   
   **To change the NFS path**: Edit `executor.swarm.nfs_base_path` in `config.yaml`

3. **S3-Compatible Object Storage**
   - AWS S3, MinIO, or any S3-compatible service
   - Bucket created (or set `storage.create_bucket: true` in config)
   - Access credentials available

4. **Postgres Database**
   - Version 12+
   - Database created: `jobrunner`

5. **Redis**
   - Version 6+
   - For job queue

## 🚀 Quick Start

### 1. Configure

Copy the example config and customize:

```bash
cp config.example.yaml config.yaml
```

**Critical settings to customize:**

```yaml
# ⚠️ NFS mount path - MUST match your NFS mount point
executor:
  swarm:
    nfs_base_path: "/mnt/jobrunner"  # ← Change this if needed

# S3 storage credentials
storage:
  endpoint: "https://s3.amazonaws.com"  # Or your MinIO/S3 endpoint
  access_key: "YOUR_ACCESS_KEY"
  secret_key: "YOUR_SECRET_KEY"
  bucket: "jobrunner"

# Database connection
database:
  url: "postgres://jobrunner:password@postgres:5432/jobrunner"

# Redis connection  
queue:
  url: "redis://redis:6379/0"

# API authentication
api:
  auth:
    api_key: "your-secret-api-key"  # Or set API_KEY env var
```

### 2. Create Docker Secrets

```bash
# API key
echo "your-secret-api-key" | docker secret create switchyard_api_key -

# Database password
echo "your-db-password" | docker secret create switchyard_db_password -

# S3 credentials
echo "your-s3-access-key" | docker secret create switchyard_s3_access_key -
echo "your-s3-secret-key" | docker secret create switchyard_s3_secret_key -
```

### 3. Deploy the Stack

```bash
# Deploy infrastructure (Postgres + Redis)
docker stack deploy -c deployments/stack.yml switchyard

# Wait for Postgres to be ready
sleep 10

# Run migrations
docker run --rm \
  --network switchyard_internal \
  -e DATABASE_URL="postgres://jobrunner:password@postgres:5432/jobrunner" \
  ghcr.io/heldtogether/switchyard-api:latest \
  /app/migrate -dir /app/migrations -action up

# Check status
docker service ls | grep switchyard
```

### 4. Submit a Job

```bash
# Submit example job
curl -X POST http://localhost:8080/v1/jobs \
  -H "X-API-Key: your-secret-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "image": "ghcr.io/heldtogether/switchyard-example-job:latest",
    "command": ["/app/entrypoint.sh"],
    "env": {
      "CUSTOM_VAR": "my-value"
    },
    "outputs": ["/outputs"],
    "resources": {
      "cpu": "0.5",
      "memory": "512m"
    },
    "timeout_seconds": 300
  }'

# Response:
# {
#   "id": "550e8400-e29b-41d4-a716-446655440000",
#   "status": "PENDING",
#   "created_at": "2026-02-18T12:00:00Z"
# }
```

### 5. Check Job Status

```bash
JOB_ID="550e8400-e29b-41d4-a716-446655440000"

# Get job details
curl -H "X-API-Key: your-secret-api-key" \
  http://localhost:8080/v1/jobs/$JOB_ID

# Get logs
curl -H "X-API-Key: your-secret-api-key" \
  http://localhost:8080/v1/jobs/$JOB_ID/logs

# List artefacts
curl -H "X-API-Key: your-secret-api-key" \
  http://localhost:8080/v1/jobs/$JOB_ID/artefacts

# Download artefact
curl -H "X-API-Key: your-secret-api-key" \
  http://localhost:8080/v1/jobs/$JOB_ID/artefacts/result.txt
```

## 🔐 Environment Variables

### System-Managed Variables

Switchyard automatically injects environment variables into every job container to provide context and metadata:

```bash
# Your job containers automatically have access to:
SWITCHYARD_JOB_ID=550e8400-e29b-41d4-a716-446655440000
SWITCHYARD_JOB_CREATED_AT=2026-02-20T14:30:00Z
SWITCHYARD_JOB_TIMEOUT=3600
SWITCHYARD_EXECUTOR_TYPE=swarm
SWITCHYARD_IMAGE=myapp:v1.0
SWITCHYARD_OUTPUTS_DIR=/outputs
SWITCHYARD_BUCKET=jobrunner
SWITCHYARD_VERSION=v1.0.0
SWITCHYARD_API_URL=http://localhost:8080
# ... plus any custom variables you provide
```

**Important:** 
- The `SWITCHYARD_*` prefix is reserved for system use
- Attempting to set `SWITCHYARD_*` variables in your job submission will result in a 400 validation error
- System variables are injected at runtime and are NOT stored in the database
- The API returns only your custom environment variables, not system ones

**Example:**
```json
{
  "env": {
    "MY_VAR": "value",         // ✅ OK
    "DATABASE_URL": "...",     // ✅ OK
    "SWITCHYARD_FOO": "bar"    // ❌ Error: reserved prefix
  }
}
```

See [ARCHITECTURE.md](ARCHITECTURE.md#environment-variable-handling) for the complete list of system variables and their purposes.

## 📁 Repository Structure

```
switchyard/
├── cmd/
│   ├── api/        # HTTP API server
│   ├── worker/     # Job execution worker
│   └── migrate/    # Database migration tool
├── internal/
│   ├── api/        # HTTP handlers & routes
│   ├── domain/     # Domain models (Job, Artefact, etc.)
│   ├── config/     # Configuration loading
│   ├── executor/   # Execution backends (Docker, Swarm, shared utilities)
│   │   ├── common.go      # Shared BaseExecutor and utilities
│   │   ├── docker/        # Docker executor
│   │   └── swarm/         # Swarm executor
│   ├── storage/    # Postgres, Redis, S3 clients
│   └── worker/     # Job processing logic
├── migrations/     # Database migrations
├── deployments/    # Docker Stack & Compose files
├── build/          # Dockerfiles
├── examples/       # Example jobs & scripts
└── config.example.yaml  # Configuration template
```

## ⚙️ Configuration Reference

See `config.example.yaml` for full documentation. Key sections:

### API Configuration
```yaml
api:
  port: 8080
  auth:
    enabled: true
    api_key: "${API_KEY}"  # Override with env var
```

### Executor Configuration (Swarm)
```yaml
executor:
  type: swarm
  swarm:
    nfs_base_path: "/mnt/jobrunner"  # ⚠️ Must match NFS mount
    defaults:
      resources:
        cpu: "1.0"
        memory: "2g"
      timeout: "1h"
      constraints:
        - "node.role==worker"
```

### Environment Variable Overrides

Any config value can be overridden with environment variables:

```bash
export API_KEY="production-key"
export EXECUTOR_NFS_BASE="/mnt/custom-path"
export WORKER_CONCURRENCY=10
export S3_ENDPOINT="https://minio.company.com"
```

Or use Docker secrets (recommended):
```bash
export API_KEY_FILE="/run/secrets/api_key"
export S3_ACCESS_KEY_FILE="/run/secrets/s3_access_key"
```

## 🔧 Development

### Local Development

```bash
# Start local environment (Postgres + Redis)
make dev-up

# Run migrations
make migrate-up

# Run API locally
go run ./cmd/api

# Run worker locally
go run ./cmd/worker

# Stop local environment
make dev-down
```

### Building

```bash
# Build binaries
make build

# Build Docker images
make docker-build VERSION=v0.1.0

# Push images
make docker-push VERSION=v0.1.0
```

## 🧪 Testing

```bash
# Run tests
make test

# Integration test (requires running stack)
./examples/scripts/integration-test.sh
```

## 📊 API Reference

### POST /v1/jobs
Submit a new job.

**Request:**
```json
{
  "image": "alpine:latest",
  "command": ["sh", "-c", "echo 'Hello' > /outputs/result.txt"],
  "env": {
    "MY_VAR": "value",
    "DATABASE_URL": "postgres://..."
  },
  "outputs": ["/outputs"],
  "resources": {
    "cpu": "1.0",
    "memory": "2g"
  },
  "timeout_seconds": 3600,
  "registry_auth": {
    "username": "user",
    "password": "pass"
  }
}
```

**Note:** Environment variables with the `SWITCHYARD_` prefix are reserved and will be rejected. System variables are automatically injected at runtime.

**Response:**
```json
{
  "id": "uuid",
  "status": "PENDING",
  "created_at": "2026-02-18T12:00:00Z"
}
```

### GET /v1/jobs/{id}
Get job details.

### GET /v1/jobs/{id}/logs
Stream or download job logs.

### GET /v1/jobs/{id}/artefacts
List output artefacts.

### GET /v1/jobs/{id}/artefacts/{path}
Download a specific artefact.

### POST /v1/jobs/{id}/cancel
Cancel a running job.

### GET /v1/jobs
List jobs (supports filtering: `?status=RUNNING&created_by=user&limit=50`)

## 🔒 Security

**MVP Security:**
- API key authentication (`X-API-Key` header)
- Network isolation (jobs have no internet access)
- Resource limits enforced

**Future Enhancements:**
- SSO/OIDC integration (structure exists in `internal/api/middleware.go`)
- RBAC (add permissions to Postgres schema)
- Image scanning before execution
- Secrets management (Vault integration)
- Audit logging

## 🐛 Troubleshooting

### Jobs stuck in PENDING
- Check worker logs: `docker service logs switchyard_worker`
- Check Redis: `docker exec -it $(docker ps -q -f name=switchyard_redis) redis-cli LLEN jobrunner:jobs`

### Jobs fail with "NFS path not found"
- Verify NFS is mounted on all nodes: `df -h | grep jobrunner`
- Check config: `executor.swarm.nfs_base_path` matches mount point
- Test write access: `touch /mnt/jobrunner/test && rm /mnt/jobrunner/test`

### "failed to create service"
- Check Docker socket access: worker needs `/var/run/docker.sock`
- Check Swarm status: `docker node ls`
- View detailed error: `docker service logs switchyard_worker`

### Logs not appearing
- Check S3 credentials in config
- Verify bucket exists: `aws s3 ls s3://jobrunner/`
- Check object store logs

## 🛣️ Roadmap

- [ ] Kubernetes executor implementation
- [ ] Web UI for job management
- [ ] Workflow DAGs (job dependencies)
- [ ] Job templates and reusable specs
- [ ] Enhanced metrics and monitoring
- [ ] Job result caching
- [ ] GPU support
- [ ] Multi-region job dispatch

## 📝 License

Internal use - proprietary.

## 🤝 Contributing

This is an internal platform. For bugs or features, file an issue in the repository.

---

**Need Help?**
- Check logs: `docker service logs switchyard_api` / `switchyard_worker`
- Verify config: Review `config.yaml` against `config.example.yaml`
- Test connectivity: `docker exec -it <worker-container> ping postgres`
