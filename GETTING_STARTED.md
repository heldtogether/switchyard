# Getting Started with Switchyard

This guide will help you get Switchyard running locally for testing.

## Prerequisites

- Docker and Docker Compose installed
- Go 1.22+ (if building from source)
- curl and jq (for testing)
- Port 8080 available for the API

## Quick Start (Local Development)

### 1. Start the Infrastructure

Start Postgres and Redis using Docker Compose:

```bash
cd deployments
docker-compose up -d
```

Verify services are running:
```bash
docker-compose ps
```

You should see both postgres and redis with status "running" and healthy.

### 2. Create a Local Config

Copy the example config:
```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` and set these for local development:

```yaml
api:
  auth:
    api_key: "test-api-key"  # Use this for testing

database:
  url: "postgres://jobrunner:password@localhost:5432/jobrunner?sslmode=disable"

queue:
  url: "redis://localhost:6379/0"

storage:
  endpoint: "https://s3.amazonaws.com"  # Your S3 endpoint
  access_key: "YOUR_S3_KEY"
  secret_key: "YOUR_S3_SECRET"
  bucket: "jobrunner-test"
  create_bucket: true  # Auto-create if using MinIO

executor:
  swarm:
    nfs_base_path: "/tmp/jobrunner"  # Local temp for testing
```

**Note**: For the NFS path, create a local directory:
```bash
mkdir -p /tmp/jobrunner
```

### 3. Run Database Migrations

Apply the database schema:

```bash
# Using the migrations directly with psql
PGPASSWORD=password psql -h localhost -U jobrunner -d jobrunner -f migrations/000001_initial_schema.up.sql
```

Or use a migration tool:
```bash
migrate -path migrations -database "postgres://jobrunner:password@localhost:5432/jobrunner?sslmode=disable" up
```

### 4. Build and Run the Services

Build the binaries:
```bash
make build
```

You should see:
```
Building binaries...
go build -ldflags="-X main.Version=dev" -o bin/api ./cmd/api
go build -ldflags="-X main.Version=dev" -o bin/worker ./cmd/worker
```

Run the API:
```bash
./bin/api -config config.yaml
```

In another terminal, run the Worker:
```bash
./bin/worker -config config.yaml
```

You should see logs indicating both services started successfully.

### 5. Build the Example Job Image

```bash
docker build -t switchyard-example-job:latest -f build/example-job/Dockerfile build/example-job/
```

### 6. Submit a Test Job

Set your API key:
```bash
export API_KEY="test-api-key"
```

Submit a job:
```bash
cd examples/scripts
./submit-job.sh
```

You should see output like:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "PENDING",
  "created_at": "2026-02-18T13:00:00Z",
  "created_by": "api-key-user",
  ...
}
```

### 7. Check Job Status

Use the job ID from the previous step:
```bash
./check-status.sh 550e8400-e29b-41d4-a716-446655440000
```

Watch it progress: PENDING → RUNNING → SUCCEEDED

### 8. View Logs

```bash
./fetch-logs.sh 550e8400-e29b-41d4-a716-446655440000
```

You should see:
```
================================
Switchyard Example Job
================================
Started at: 2026-02-18T13:00:05+00:00
...
```

### 9. List and Download Artefacts

```bash
./list-artefacts.sh 550e8400-e29b-41d4-a716-446655440000
```

Download a specific artefact:
```bash
curl -H "X-API-Key: test-api-key" \
  -o result.txt \
  "http://localhost:8080/v1/jobs/550e8400-e29b-41d4-a716-446655440000/artefacts/result.txt"

cat result.txt
```

## Testing Without Docker Swarm

For local testing, the system will try to create Docker services on your local Docker daemon. If you don't have Swarm initialized:

```bash
docker swarm init
```

This creates a single-node Swarm on your machine.

## Common Issues

### "NFS path not found"

The worker checks that `executor.swarm.nfs_base_path` exists and is writable.

**Solution**: Create the directory:
```bash
mkdir -p /tmp/jobrunner
chmod 777 /tmp/jobrunner
```

Or update your `config.yaml`:
```yaml
executor:
  swarm:
    nfs_base_path: "/tmp/jobrunner"  # Or any local path
```

### "failed to connect to database"

**Solution**: Ensure Postgres is running:
```bash
cd deployments
docker-compose ps postgres
```

If not running:
```bash
docker-compose up -d postgres
```

### "failed to connect to redis"

**Solution**: Ensure Redis is running:
```bash
docker-compose ps redis
docker-compose up -d redis
```

### "Job stuck in PENDING"

**Checklist**:
1. Is the worker running? Check logs: `./bin/worker`
2. Is Redis accessible? Test: `redis-cli -h localhost ping`
3. Is the Docker socket accessible? Test: `docker ps`
4. Is the job image available? Test: `docker images | grep switchyard-example`

### "failed to upload logs to S3"

**Solution**: Check your S3 credentials in `config.yaml`:
```yaml
storage:
  endpoint: "https://s3.amazonaws.com"
  access_key: "YOUR_KEY"
  secret_key: "YOUR_SECRET"
  bucket: "your-bucket"
```

Test S3 access:
```bash
aws s3 ls s3://your-bucket/
```

## Next Steps

### Running on Docker Swarm

For production deployment on a real Swarm cluster:

1. Setup NFS on all nodes
2. Create Docker secrets
3. Deploy with stack.yml (coming soon)

See README.md for full production deployment guide.

### Customizing Jobs

Create your own job images:

```dockerfile
FROM ubuntu:22.04

# Install dependencies
RUN apt-get update && apt-get install -y your-tools

# Your job logic
COPY my-script.sh /app/
RUN chmod +x /app/my-script.sh

# Write to /outputs
CMD ["/app/my-script.sh"]
```

Submit with:
```json
{
  "image": "my-job:v1.0",
  "command": ["/app/my-script.sh"],
  "outputs": ["/outputs"],
  "resources": {
    "cpu": "2.0",
    "memory": "4g"
  },
  "timeout_seconds": 3600
}
```

### Monitoring

Watch logs:
```bash
# API logs
./bin/api -config config.yaml 2>&1 | jq .

# Worker logs
./bin/worker -config config.yaml 2>&1 | jq .
```

Check health:
```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Development Workflow

1. Make code changes
2. Rebuild: `make build`
3. Restart services (Ctrl+C and rerun)
4. Test with example scripts

## Cleanup

Stop services:
```bash
# Stop API and Worker (Ctrl+C in their terminals)

# Stop infrastructure
cd deployments
docker-compose down -v  # -v removes volumes (deletes data)
```

Clean up Docker:
```bash
docker system prune -a  # Removes all unused containers and images
```

## Summary

You now have a working Switchyard installation! You can:

✅ Submit jobs via HTTP API
✅ Execute containerized workloads
✅ Collect logs and artefacts
✅ Query job status

For production deployment with Docker Swarm, see the main README.md.
