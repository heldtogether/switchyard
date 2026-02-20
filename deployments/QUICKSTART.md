# Switchyard Quick Start - Docker Swarm

Fast deployment guide for getting Switchyard running on Docker Swarm.

## 🚀 5-Minute Deploy

### 1. Prerequisites Check

```bash
# Verify Swarm is initialized
docker node ls

# Verify NFS is mounted on all nodes
ssh <each-node> "df -h | grep jobrunner"

# Verify external Postgres is accessible
psql "$DATABASE_URL" -c "SELECT version();"
```

### 2. Create Secrets

```bash
# Create all required secrets at once
echo "your-secure-api-key" | docker secret create switchyard_api_key -
echo "your-s3-access-key" | docker secret create switchyard_s3_access_key -
echo "your-s3-secret-key" | docker secret create switchyard_s3_secret_key -

# Verify
docker secret ls | grep switchyard
```

### 3. Configure Environment

```bash
cd deployments

# Create .env file
cat > .env <<EOF
VERSION=latest
DATABASE_URL=postgres://jobrunner:password@db-host:5432/jobrunner?sslmode=require
S3_ENDPOINT=https://s3.amazonaws.com
S3_BUCKET=jobrunner
S3_REGION=us-east-1
API_REPLICAS=2
WORKER_REPLICAS=2
LOGGING_LEVEL=info
EOF
```

### 4. Deploy

```bash
# Deploy the stack
docker stack deploy -c stack.yml switchyard

# Wait for services to start
watch docker stack ps switchyard

# Check logs
docker service logs -f switchyard_api
```

### 5. Verify

```bash
# Health check
curl http://localhost:8080/healthz

# Submit test job
curl -X POST http://localhost:8080/v1/jobs \
  -H "X-API-Key: your-secure-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "image": "alpine:latest",
    "command": ["sh", "-c", "echo Hello > /outputs/test.txt"],
    "outputs": ["/outputs"]
  }'
```

## 📋 Common Commands

### Service Management

```bash
# List services
docker stack services switchyard

# Scale services
docker service scale switchyard_api=5
docker service scale switchyard_worker=10

# Update service
docker service update --image ghcr.io/heldtogether/switchyard-api:v1.1.0 switchyard_api

# Restart service
docker service update --force switchyard_worker

# Remove stack
docker stack rm switchyard
```

### Logs and Debugging

```bash
# Follow logs
docker service logs -f switchyard_api
docker service logs -f switchyard_worker
docker service logs -f --tail 100 switchyard_redis

# View service details
docker service inspect switchyard_worker --pretty

# Check running containers
docker ps --filter "label=com.docker.stack.namespace=switchyard"

# Exec into container
docker exec -it $(docker ps -qf "label=com.docker.swarm.service.name=switchyard_worker") sh
```

### Updates and Rollbacks

```bash
# Deploy new version
VERSION=v1.1.0 docker stack deploy -c stack.yml switchyard

# Rollback
docker service rollback switchyard_api
docker service rollback switchyard_worker

# Force update (restart)
docker service update --force switchyard_api
```

### Monitoring

```bash
# Service status
docker stack ps switchyard

# Resource usage
docker stats $(docker ps -qf "label=com.docker.stack.namespace=switchyard")

# Check Redis queue
docker exec -it $(docker ps -qf "name=switchyard_redis") redis-cli LLEN jobrunner:jobs
```

## 🔧 Troubleshooting Quick Fixes

### Jobs Not Starting
```bash
# Check worker logs
docker service logs switchyard_worker | grep ERROR

# Check Redis
docker exec -it $(docker ps -qf "name=switchyard_redis") redis-cli ping
docker exec -it $(docker ps -qf "name=switchyard_redis") redis-cli LLEN jobrunner:jobs

# Restart workers
docker service update --force switchyard_worker
```

### API Not Responding
```bash
# Check API logs
docker service logs switchyard_api | tail -50

# Test connectivity
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# Restart API
docker service update --force switchyard_api
```

### NFS Issues
```bash
# On each Swarm node
ssh node-1 "df -h | grep jobrunner"
ssh node-1 "touch /mnt/jobrunner/test && rm /mnt/jobrunner/test"

# Check worker mount
docker exec -it $(docker ps -qf "name=switchyard_worker") ls -la /mnt/jobrunner
```

## 🎯 Environment Variables Reference

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `VERSION` | Image tag | No | `latest` |
| `DATABASE_URL` | Postgres connection | Yes | - |
| `S3_ENDPOINT` | S3 endpoint | Yes | - |
| `S3_BUCKET` | S3 bucket name | No | `jobrunner` |
| `S3_REGION` | S3 region | No | `us-east-1` |
| `API_REPLICAS` | API instances | No | `2` |
| `WORKER_REPLICAS` | Worker instances | No | `2` |
| `WORKER_CONCURRENCY` | Jobs per worker | No | `5` |
| `LOGGING_LEVEL` | Log level | No | `info` |

## 📝 Pre-Deployment Checklist

- [ ] Docker Swarm initialized
- [ ] NFS mounted on all nodes at `/mnt/jobrunner`
- [ ] External Postgres accessible
- [ ] Database migrations applied (`make migrate-up`)
- [ ] S3 bucket created and accessible
- [ ] Docker secrets created (3 secrets)
- [ ] `.env` file configured
- [ ] `config.yaml` reviewed
- [ ] Images built and pushed
- [ ] Firewall allows port 8080 (API)

## 🆘 Emergency Commands

### Complete Reset
```bash
# Remove stack
docker stack rm switchyard

# Wait for cleanup
sleep 30

# Remove data (⚠️  destroys Redis data)
docker volume rm switchyard_redis_data

# Redeploy
docker stack deploy -c stack.yml switchyard
```

### Force Restart All
```bash
docker service update --force switchyard_api
docker service update --force switchyard_worker
docker service update --force switchyard_redis
```

### Check All Health
```bash
echo "=== Services ==="
docker stack services switchyard

echo -e "\n=== API Health ==="
curl -s http://localhost:8080/healthz | jq .

echo -e "\n=== Redis ==="
docker exec $(docker ps -qf "name=switchyard_redis") redis-cli ping

echo -e "\n=== Database ==="
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM jobs;"
```

## 📚 Further Reading

- Full deployment guide: `DEPLOYMENT.md`
- Configuration reference: `../config.example.yaml`
- API documentation: `../README.md`
- Troubleshooting: `DEPLOYMENT.md` (Troubleshooting section)
