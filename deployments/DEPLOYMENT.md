# Switchyard Deployment Guide

This guide covers deploying Switchyard to a Docker Swarm cluster. For file inventory and quick commands, see `deployments/README.md`.

## Prerequisites
- **Docker Swarm** initialized with manager/worker nodes.
- **Postgres 12+** reachable by the swarm.
- **NFS share** mounted at the same path on all nodes (for job outputs).
- **S3-compatible storage** for logs and artefacts.

## Deployment Steps

### 1. Create Secrets
```bash
echo "your-secure-api-key" | docker secret create switchyard_api_key -
echo "your-s3-access-key" | docker secret create switchyard_s3_access_key -
echo "your-s3-secret-key" | docker secret create switchyard_s3_secret_key -
```

### 2. Configure Environment
```bash
cd deployments
cp .env.example .env
# edit .env with database and S3 values
```

Required `.env` keys:
```
VERSION=your-tag
DATABASE_URL=postgres://jobrunner:password@db-host:5432/jobrunner?sslmode=require
S3_ENDPOINT=https://s3.amazonaws.com
S3_BUCKET=jobrunner
S3_REGION=us-east-1
```

### 3. Validate `deployments/config.yaml`
Ensure:
- `executor.swarm.nfs_base_path` matches your NFS mount
- `worker.concurrency` matches capacity
- `executor.swarm.defaults.constraints` match your node labels

### 4. Build/Push Images (if self-hosting)
```bash
make docker-build VERSION=your-tag
make docker-push VERSION=your-tag
```

### 5. Deploy
```bash
docker stack deploy -c deployments/stack.yml switchyard
```

### 6. Verify
```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Configuration Reference (Environment Variables)
Key `.env` variables (see `.env.example` for full list):

| Variable | Description | Default |
|----------|-------------|---------|
| `VERSION` | Image tag | `latest` |
| `DATABASE_URL` | Postgres connection | Required |
| `S3_ENDPOINT` | S3 endpoint | Required |
| `S3_BUCKET` | S3 bucket name | `jobrunner` |
| `S3_REGION` | S3 region | `us-east-1` |
| `API_REPLICAS` | API replicas | `2` |
| `WORKER_REPLICAS` | Worker replicas | `2` |
| `WORKER_CONCURRENCY` | Jobs per worker | `5` |
| `LOGGING_LEVEL` | Log level | `info` |

## Common Ops
```bash
# Scale
API_REPLICAS=3 WORKER_REPLICAS=6 docker stack deploy -c deployments/stack.yml switchyard

# Rolling update
VERSION=your-tag docker stack deploy -c deployments/stack.yml switchyard

# Logs
docker service logs -f switchyard_api
docker service logs -f switchyard_worker
```

## Troubleshooting (Short)
- **Jobs stuck in PENDING**: check worker logs and Redis connectivity.
- **NFS errors**: confirm the NFS mount exists on every node and matches `executor.swarm.nfs_base_path`.
- **API not ready**: validate `DATABASE_URL` and Redis reachability.
