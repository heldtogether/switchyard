# Switchyard Deployment Files

This directory contains all files needed to deploy Switchyard to Docker Swarm.

## 📁 Files Overview

### Configuration Files

- **`config.yaml`** - Production configuration template
  - Service ports, timeouts, resource limits
  - Executor settings (NFS path, constraints)
  - Storage configuration (S3, Redis, Postgres)

- **`.env.example`** - Environment variables template
  - Copy to `.env` and customize
  - Used by `stack.yml` for variable substitution

### Deployment Files

- **`stack.yml`** - Docker Swarm stack definition
  - Services: API, Worker, Redis
  - Networks: internal, public
  - Secrets: API key, S3 credentials
  - Complete production configuration

- **`docker-compose.yml`** - Local development
  - Postgres + Redis only
  - For local `go run` development
  - Used by `make dev-up`

### Documentation

- **`QUICKSTART.md`** - 5-minute deployment guide
  - Fast track to production
  - Common commands reference
  - Quick troubleshooting

- **`DEPLOYMENT.md`** - Complete deployment guide
  - Detailed prerequisites
  - Step-by-step instructions
  - Monitoring and maintenance
  - Troubleshooting guide
  - Backup and recovery

## 🚀 Quick Start

### First Time Deployment

```bash
# 1. Setup secrets
echo "your-api-key" | docker secret create switchyard_api_key -
echo "s3-access-key" | docker secret create switchyard_s3_access_key -
echo "s3-secret-key" | docker secret create switchyard_s3_secret_key -

# 2. Configure environment
cp .env.example .env
nano .env  # Edit with your settings

# 3. Deploy
docker stack deploy -c stack.yml switchyard

# 4. Verify
curl http://localhost:8080/healthz
```

See **`QUICKSTART.md`** for detailed quick start guide.

### Local Development

```bash
# Start local Postgres + Redis
make dev-up

# Run services locally
make build
./bin/api &
./bin/worker &

# Stop local environment
make dev-down
```

## 📊 Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Docker Swarm Cluster                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐        ┌──────────────┐                     │
│  │  API         │        │  API         │                     │
│  │  (replica 1) │        │  (replica 2) │                     │
│  └──────┬───────┘        └──────┬───────┘                     │
│         │                       │                              │
│         └───────────┬───────────┘                              │
│                     │                                          │
│         ┌───────────▼──────────┐                               │
│         │                      │                               │
│         │       Redis          │                               │
│         │    (Job Queue)       │                               │
│         │                      │                               │
│         └───────────┬──────────┘                               │
│                     │                                          │
│         ┌───────────▼───────────┐                              │
│         │                       │                              │
│  ┌──────┴────────┐      ┌──────┴────────┐                    │
│  │  Worker       │      │  Worker       │                    │
│  │  (replica 1)  │      │  (replica 2)  │                    │
│  └───┬───────┬───┘      └───┬───────┬───┘                    │
│      │       │              │       │                         │
│      │       └──────────────┴───────┘                         │
│      │              │                                          │
│      │       (Docker Socket)                                  │
│      │                                                         │
│      └──────────────────────────────────┐                     │
│                 │                        │                     │
│          ┌──────▼─────┐          ┌──────▼─────┐              │
│          │  NFS Mount │          │  Job       │              │
│          │  /mnt/     │          │  Container │              │
│          │  jobrunner │◄─────────┤  (Swarm    │              │
│          └────────────┘          │   Service) │              │
│                                  └────────────┘              │
└─────────────────────────────────────────────────────────────────┘

External Services:
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Postgres DB   │  │   S3 Storage    │  │   NFS Server    │
│   (External)    │  │   (External)    │  │   (External)    │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## 🔧 Configuration Overview

### Service Dependencies

```
API Service:
  ├── Redis (internal)
  ├── Postgres (external)
  └── S3 (external)

Worker Service:
  ├── Redis (internal)
  ├── Postgres (external)
  ├── S3 (external)
  ├── Docker Socket (for job execution)
  └── NFS Mount (for artefact collection)

Redis Service:
  └── Persistent Volume
```

### Required Secrets

```bash
switchyard_api_key          # API authentication
switchyard_s3_access_key    # S3 access credentials
switchyard_s3_secret_key    # S3 secret credentials
```

### Required Environment Variables

```bash
DATABASE_URL       # External Postgres connection string
S3_ENDPOINT        # S3-compatible storage endpoint
S3_BUCKET          # S3 bucket name
S3_REGION          # S3 region
```

## 📋 Deployment Checklist

### Infrastructure Prerequisites

- [ ] Docker Swarm cluster initialized
- [ ] External Postgres 12+ running
- [ ] Database migrations applied
- [ ] NFS share exported
- [ ] NFS mounted on all Swarm nodes at `/mnt/jobrunner`
- [ ] S3-compatible storage configured
- [ ] S3 bucket created

### Deployment Prerequisites

- [ ] Docker images built and pushed
- [ ] Docker secrets created (3 secrets)
- [ ] `.env` file configured
- [ ] `config.yaml` reviewed and customized
- [ ] Firewall allows port 8080 (API)

### Post-Deployment Verification

- [ ] All services running: `docker stack services switchyard`
- [ ] API health check passes: `curl http://localhost:8080/healthz`
- [ ] Redis accessible: `curl http://localhost:8080/readyz`
- [ ] Can submit job: `./examples/scripts/submit-job.sh`
- [ ] Job executes successfully
- [ ] Logs available: `./examples/scripts/fetch-logs.sh <job-id>`
- [ ] Artefacts retrievable: `./examples/scripts/list-artefacts.sh <job-id>`

## 🎯 Common Operations

### Scaling

```bash
# Scale API for more throughput
docker service scale switchyard_api=5

# Scale Workers for more job capacity
docker service scale switchyard_worker=10
```

### Updates

```bash
# Update to new version
VERSION=v1.1.0 docker stack deploy -c stack.yml switchyard

# Rollback if needed
docker service rollback switchyard_api
docker service rollback switchyard_worker
```

### Monitoring

```bash
# View all services
docker stack services switchyard

# Follow logs
docker service logs -f switchyard_api
docker service logs -f switchyard_worker

# Check queue depth
docker exec $(docker ps -qf "name=switchyard_redis") \
  redis-cli LLEN jobrunner:jobs
```

### Troubleshooting

```bash
# View detailed service info
docker service ps switchyard_worker --no-trunc

# Inspect service
docker service inspect switchyard_worker --pretty

# Exec into container
docker exec -it $(docker ps -qf "name=switchyard_worker") sh

# Check logs with errors only
docker service logs switchyard_worker | grep ERROR
```

## 📚 Documentation Index

1. **[QUICKSTART.md](./QUICKSTART.md)** - Fast deployment in 5 minutes
2. **[DEPLOYMENT.md](./DEPLOYMENT.md)** - Complete deployment guide
3. **[../README.md](../README.md)** - Main project documentation
4. **[../STATUS.md](../STATUS.md)** - Implementation status
5. **[config.yaml](./config.yaml)** - Configuration template with comments

## 🆘 Getting Help

### Common Issues

See the troubleshooting sections in:
- `QUICKSTART.md` - Quick fixes
- `DEPLOYMENT.md` - Detailed troubleshooting

### Debug Information

When reporting issues, include:
```bash
# Service status
docker stack services switchyard

# Recent logs
docker service logs --tail 100 switchyard_worker

# Configuration
cat deployments/.env
docker service inspect switchyard_worker --format '{{json .Spec}}'

# System info
docker version
docker node ls
```

## 🔐 Security Notes

1. **Secrets Management**
   - Use Docker secrets for all credentials
   - Never commit secrets to version control
   - Rotate secrets regularly

2. **Network Security**
   - API exposed on port 8080 only
   - Workers on internal network
   - Jobs isolated by default

3. **Access Control**
   - API key authentication required
   - Configure API_KEY via secret
   - Consider SSO/OIDC for production

## 📦 What's Included

### Services (stack.yml)
- ✅ API Server (2 replicas, load balanced)
- ✅ Worker Service (2 replicas, scalable)
- ✅ Redis (queue with persistence)

### Configuration
- ✅ Secrets (API key, S3 credentials)
- ✅ Configs (config.yaml mounted)
- ✅ Networks (internal + public)
- ✅ Volumes (Redis persistence)

### Features
- ✅ Rolling updates
- ✅ Automatic rollback on failure
- ✅ Health checks
- ✅ Resource limits
- ✅ Placement constraints
- ✅ Graceful shutdown

## 🎁 What's Not Included

These are managed externally:

- **Postgres Database** - Use your existing instance
- **S3 Storage** - Use AWS S3, MinIO, or compatible
- **NFS Server** - Setup separately and mount on nodes
- **Metrics/Monitoring** - Use Prometheus/Grafana
- **Ingress/Load Balancer** - Use Traefik, nginx, or cloud LB
- **TLS Certificates** - Terminate at load balancer

## 🚀 Next Steps

1. Review `QUICKSTART.md` for fast deployment
2. Read `DEPLOYMENT.md` for detailed guide
3. Customize `config.yaml` for your environment
4. Deploy and monitor via `docker stack`
5. Submit your first job via API!

---

**Ready to deploy?** Start with [QUICKSTART.md](./QUICKSTART.md)!
