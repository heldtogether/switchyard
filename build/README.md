# Switchyard Docker Images

This directory contains Dockerfiles for building the Switchyard services.

## Available Images
- **API Service**: `build/api.Dockerfile` → `switchyard-api`
- **Worker Service**: `build/worker.Dockerfile` → `switchyard-worker`
- **Example Job**: `build/example-job/Dockerfile` → `switchyard-example-job`

## Build Commands
```bash
# Build individual images
make docker-build

# Or build manually
docker build -f build/api.Dockerfile -t switchyard-api:latest .
docker build -f build/worker.Dockerfile -t switchyard-worker:latest .
docker build -f build/example-job/Dockerfile -t switchyard-example-job:latest ./build/example-job
```

## Runtime Expectations
- API and Worker images expect `/app/config.yaml` (mounted or baked in).
- Env vars can override config values.
- The Worker requires Docker socket access for job execution:
```bash
docker run -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./config.yaml:/app/config.yaml \
  switchyard-worker:latest
```
