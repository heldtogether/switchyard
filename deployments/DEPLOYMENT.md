# Switchyard Deployment Guide

This guide covers deploying Switchyard to a Docker Swarm cluster.

## Prerequisites

### 1. Docker Swarm Cluster
Ensure you have a Swarm cluster initialized:
```bash
# On manager node
docker swarm init

# On worker nodes (use token from manager)
docker swarm join --token SWMTKN-... manager-ip:2377
```

### 2. External Postgres Database
Switchyard requires a Postgres 12+ database. The stack expects an external Postgres instance.

**Setup database:**
```sql
CREATE DATABASE jobrunner;
CREATE USER jobrunner WITH PASSWORD 'your-secure-password';
GRANT ALL PRIVILEGES ON DATABASE jobrunner TO jobrunner;
```

**Run migrations:**
```bash
# From a machine with access to the database
DATABASE_URL="postgres://jobrunner:password@db-host:5432/jobrunner" \
  make migrate-up
```

### 3. NFS Share (Critical!)
All Swarm nodes **must** mount the same NFS share at the same path.

**On NFS server:**
```bash
# Export directory
sudo mkdir -p /export/jobrunner
sudo chown nobody:nogroup /export/jobrunner
sudo chmod 777 /export/jobrunner

# Add to /etc/exports
echo "/export/jobrunner *(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
sudo exportfs -ra
```

**On all Swarm nodes:**
```bash
# Mount NFS
sudo mkdir -p /mnt/jobrunner
sudo mount -t nfs nfs-server:/export/jobrunner /mnt/jobrunner

# Make permanent in /etc/fstab
echo "nfs-server:/export/jobrunner /mnt/jobrunner nfs defaults 0 0" | sudo tee -a /etc/fstab

# Verify
df -h | grep jobrunner
touch /mnt/jobrunner/test && rm /mnt/jobrunner/test
```

### 4. S3-Compatible Storage
Configure S3 or MinIO for logs and artefacts. Ensure:
- Bucket exists (or set `create_bucket: true` in config)
- Access credentials are valid
- Endpoint is reachable from Swarm nodes

## Deployment Steps

### Step 1: Create Docker Secrets

Create secrets for sensitive credentials:

```bash
# API Key (for authentication)
echo "your-secure-api-key-here" | docker secret create switchyard_api_key -

# S3 Credentials
echo "your-s3-access-key" | docker secret create switchyard_s3_access_key -
echo "your-s3-secret-key" | docker secret create switchyard_s3_secret_key -

# Verify secrets
docker secret ls
```

### Step 2: Configure Environment

Create `.env` file from template:

```bash
cd deployments
cp .env.example .env
nano .env  # Edit with your settings
```

**Required variables:**
```bash
VERSION=v1.0.0
DATABASE_URL=postgres://jobrunner:password@db-host:5432/jobrunner?sslmode=require
S3_ENDPOINT=https://s3.amazonaws.com
S3_BUCKET=jobrunner
S3_REGION=us-east-1
```

### Step 3: Prepare Configuration

Ensure `deployments/config.yaml` is properly configured:

```bash
# Review configuration
cat deployments/config.yaml

# Key settings to verify:
# - executor.swarm.nfs_base_path matches your NFS mount (/mnt/jobrunner)
# - worker.concurrency matches your capacity
# - executor.swarm.defaults.constraints matches your node labels
```

### Step 4: Build and Push Images

```bash
# Build images
make docker-build VERSION=v1.0.0

# Push to registry
make docker-push VERSION=v1.0.0

# Or push individually
docker push ghcr.io/heldtogether/switchyard-api:v1.0.0
docker push ghcr.io/heldtogether/switchyard-worker:v1.0.0
docker push ghcr.io/heldtogether/switchyard-example-job:v1.0.0
```

### Step 5: Deploy Stack

```bash
# Deploy the stack
docker stack deploy -c deployments/stack.yml switchyard

# Check deployment status
docker stack services switchyard
docker stack ps switchyard

# View logs
docker service logs -f switchyard_api
docker service logs -f switchyard_worker
docker service logs -f switchyard_redis
```

### Step 6: Verify Deployment

```bash
# Check API health
curl -s http://localhost:8080/healthz | jq .
curl -s http://localhost:8080/readyz | jq .

# Submit a test job
./examples/scripts/submit-job.sh

# Check job status
./examples/scripts/check-status.sh <job-id>
```

## Configuration Reference

### Environment Variables

The stack supports these environment variables (see `.env.example`):

| Variable | Description | Default |
|----------|-------------|---------|
| `VERSION` | Image tag to deploy | `latest` |
| `DATABASE_URL` | Postgres connection string | Required |
| `S3_ENDPOINT` | S3-compatible endpoint | Required |
| `S3_BUCKET` | S3 bucket name | `jobrunner` |
| `S3_REGION` | S3 region | `us-east-1` |
| `API_REPLICAS` | Number of API replicas | `2` |
| `WORKER_REPLICAS` | Number of Worker replicas | `2` |
| `WORKER_CONCURRENCY` | Jobs per worker | `5` |
| `LOGGING_LEVEL` | Log level (debug/info/warn/error) | `info` |

### Service Placement

**API Service:**
- Runs on any node
- Default: 2 replicas
- Exposes port 8080

**Worker Service:**
- Constrained to worker nodes by default
- Requires Docker socket access
- Requires NFS mount access
- Default: 2 replicas

**Redis Service:**
- Constrained to manager node
- Single replica (can be scaled if using Redis Cluster)
- Persists data to volume

## Scaling

Scale services dynamically:

```bash
# Scale API
docker service scale switchyard_api=5

# Scale Workers
docker service scale switchyard_worker=10

# Scale with env vars (redeploy)
API_REPLICAS=5 WORKER_REPLICAS=10 docker stack deploy -c deployments/stack.yml switchyard
```

## Updates and Rollbacks

### Rolling Update

```bash
# Update to new version
VERSION=v1.1.0 docker stack deploy -c deployments/stack.yml switchyard

# Monitor update
docker service ps switchyard_api
docker service ps switchyard_worker
```

### Rollback

```bash
# Rollback a service
docker service rollback switchyard_api
docker service rollback switchyard_worker

# Or redeploy previous version
VERSION=v1.0.0 docker stack deploy -c deployments/stack.yml switchyard
```

## Monitoring

### Service Status

```bash
# List all services
docker stack services switchyard

# View service details
docker service inspect switchyard_api --pretty
docker service inspect switchyard_worker --pretty

# View running tasks
docker service ps switchyard_api
docker service ps switchyard_worker
```

### Logs

```bash
# Follow API logs
docker service logs -f --tail 100 switchyard_api

# Follow Worker logs
docker service logs -f --tail 100 switchyard_worker

# Follow all services
docker service logs -f switchyard_api switchyard_worker switchyard_redis

# Filter by time
docker service logs --since 1h switchyard_worker
```

### Resource Usage

```bash
# View resource stats
docker stats $(docker ps --filter "label=com.docker.swarm.service.name=switchyard_api" -q)
docker stats $(docker ps --filter "label=com.docker.swarm.service.name=switchyard_worker" -q)
```

## Troubleshooting

### Common Issues

#### 1. Workers Can't Create Jobs
**Symptom:** Jobs stuck in PENDING, worker logs show "failed to create service"

**Solutions:**
- Verify Docker socket is mounted: `docker service inspect switchyard_worker`
- Check worker has permissions: `docker exec -it <worker-container> ls -la /var/run/docker.sock`
- Ensure worker nodes have access to NFS

#### 2. Jobs Fail with "NFS path not found"
**Symptom:** Jobs fail immediately with "failed to create NFS directory"

**Solutions:**
- Verify NFS is mounted on ALL nodes: `docker node ls` then check each
- Check mount path matches config: `executor.swarm.nfs_base_path`
- Test write access: `docker exec -it <worker-container> touch /mnt/jobrunner/test`

#### 3. API Returns 503 (Service Unavailable)
**Symptom:** API health check fails, readyz returns not ready

**Solutions:**
- Check Redis connection: `docker exec -it <redis-container> redis-cli ping`
- Check database connection: Verify `DATABASE_URL` is correct
- View API logs: `docker service logs switchyard_api`

#### 4. Jobs Don't Start
**Symptom:** Jobs stuck in PENDING state indefinitely

**Solutions:**
- Check Redis queue: `docker exec -it <redis-container> redis-cli LLEN jobrunner:jobs`
- Verify workers are running: `docker service ps switchyard_worker`
- Check worker logs: `docker service logs switchyard_worker`
- Verify job was enqueued: Check API logs after submission

#### 5. Images Not Found
**Symptom:** Service fails to start with "image not found"

**Solutions:**
- Ensure images are pushed: `docker image ls | grep switchyard`
- Check registry authentication: `docker login ghcr.io`
- Verify VERSION env var: `echo $VERSION`

### Debug Commands

```bash
# Check network connectivity
docker run --rm --network switchyard_internal alpine ping -c 3 redis

# Inspect service details
docker service inspect switchyard_worker --pretty

# Check node resources
docker node inspect self --pretty

# View service constraints
docker service inspect switchyard_worker --format '{{.Spec.TaskTemplate.Placement}}'

# Test NFS access
docker run --rm -v /mnt/jobrunner:/test alpine sh -c "echo test > /test/debug && cat /test/debug && rm /test/debug"
```

## Backup and Recovery

### Redis Data

Redis data is persisted in a volume:

```bash
# Backup
docker run --rm -v switchyard_redis_data:/data -v $(pwd):/backup alpine \
  tar czf /backup/redis-backup-$(date +%Y%m%d).tar.gz -C /data .

# Restore
docker run --rm -v switchyard_redis_data:/data -v $(pwd):/backup alpine \
  tar xzf /backup/redis-backup-YYYYMMDD.tar.gz -C /data
```

### Configuration

```bash
# Backup secrets
docker secret inspect switchyard_api_key > secrets-backup.json

# Backup configs
docker config inspect switchyard_config > config-backup.json
```

## Uninstall

To completely remove Switchyard:

```bash
# Remove stack
docker stack rm switchyard

# Wait for cleanup
watch docker ps

# Remove secrets (optional)
docker secret rm switchyard_api_key
docker secret rm switchyard_s3_access_key
docker secret rm switchyard_s3_secret_key

# Remove volumes (optional - DESTROYS REDIS DATA)
docker volume rm switchyard_redis_data

# Remove networks (automatic after stack removal)
```

## Production Best Practices

1. **High Availability:**
   - Run multiple manager nodes (3 or 5)
   - Scale API replicas across multiple nodes
   - Use Redis Cluster for queue HA

2. **Resource Management:**
   - Set appropriate resource limits
   - Monitor memory usage
   - Scale workers based on job volume

3. **Security:**
   - Use TLS for external connections
   - Rotate API keys regularly
   - Use Docker secrets for all credentials
   - Enable Postgres SSL mode

4. **Monitoring:**
   - Implement health checks
   - Monitor service logs
   - Track job success/failure rates
   - Monitor NFS performance

5. **Backups:**
   - Regular Redis backups
   - Database backups
   - Configuration version control

## Support

For issues or questions:
- Check logs: `docker service logs <service>`
- Review configuration: `deployments/config.yaml`
- See main README: `../README.md`
