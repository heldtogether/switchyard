# Switchyard Docker Images

This directory contains Dockerfiles for building the Switchyard services.

## Available Images

### 1. API Service (`api.Dockerfile`)
**Image:** `ghcr.io/heldtogether/switchyard-api`

Multi-stage Docker image for the Switchyard API server.

**Features:**
- Built with Go 1.24-alpine
- Multi-stage build for minimal size (~43MB)
- Non-root user (switchyard:1000)
- Includes migrations directory
- Built-in health check on `/healthz`

**Usage:**
```bash
docker build -f build/api.Dockerfile -t ghcr.io/heldtogether/switchyard-api:latest .
```

### 2. Worker Service (`worker.Dockerfile`)
**Image:** `ghcr.io/heldtogether/switchyard-worker`

Multi-stage Docker image for the Switchyard Worker service.

**Features:**
- Built with Go 1.24-alpine
- Multi-stage build for minimal size (~44MB)
- Non-root user (switchyard:1000)
- Requires Docker socket mount for job execution

**Usage:**
```bash
docker build -f build/worker.Dockerfile -t ghcr.io/heldtogether/switchyard-worker:latest .
```

### 3. Example Job (`example-job/Dockerfile`)
**Image:** `ghcr.io/heldtogether/switchyard-example-job`

Minimal Alpine-based example job that writes test output.

**Features:**
- Alpine 3.19 base (~20MB)
- Simple bash script that writes to `/outputs`
- Demonstrates job execution

**Usage:**
```bash
docker build -f build/example-job/Dockerfile -t ghcr.io/heldtogether/switchyard-example-job:latest ./build/example-job
```

## Building All Images

Use the Makefile to build all images at once:

```bash
# Build with default registry (ghcr.io/heldtogether)
make docker-build VERSION=v1.0.0

# Build with custom registry
make docker-build VERSION=v1.0.0 DOCKER_REGISTRY=myregistry.io/myorg

# Build development version
make docker-build
```

## Image Details

### Base Images
- **Builder stage:** `golang:1.24-alpine`
- **Runtime stage:** `alpine:3.19`

### Security Features
- Non-root user (UID/GID 1000)
- Static binaries (CGO_ENABLED=0)
- Minimal runtime dependencies
- No shell in runtime (except Alpine sh)

### Configuration

Both API and Worker images expect:

1. **Config file** at `/app/config.yaml` (can be mounted as volume)
2. **Environment variables** for overrides (see config.example.yaml)
3. **Docker secrets** mounted at `/run/secrets/*` (optional)

Example volume mounts:
```bash
docker run -v ./config.yaml:/app/config.yaml \
  ghcr.io/heldtogether/switchyard-api:latest
```

### Worker-Specific Requirements

The Worker needs access to the Docker socket to create jobs:

```bash
docker run -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./config.yaml:/app/config.yaml \
  ghcr.io/heldtogether/switchyard-worker:latest
```

## Pushing Images

```bash
# Push all images
make docker-push VERSION=v1.0.0

# Login to registry first
docker login ghcr.io

# Or manually push individual images
docker push ghcr.io/heldtogether/switchyard-api:v1.0.0
docker push ghcr.io/heldtogether/switchyard-worker:v1.0.0
docker push ghcr.io/heldtogether/switchyard-example-job:v1.0.0
```

## Development

For local testing:

```bash
# Build test images
docker build -f build/api.Dockerfile -t switchyard-api:test .
docker build -f build/worker.Dockerfile -t switchyard-worker:test .

# Test API
docker run --rm -p 8080:8080 \
  -e DATABASE_URL=postgres://... \
  -e QUEUE_URL=redis://... \
  switchyard-api:test

# Test Worker
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e DATABASE_URL=postgres://... \
  -e QUEUE_URL=redis://... \
  switchyard-worker:test
```

## Build Arguments

Both Dockerfiles support the `VERSION` build argument:

```bash
docker build --build-arg VERSION=v1.0.0 -f build/api.Dockerfile .
```

The version is embedded in the binary and visible via logs on startup.

## Size Optimization

The multi-stage builds result in minimal image sizes:

| Image | Size |
|-------|------|
| switchyard-api | ~43MB |
| switchyard-worker | ~44MB |
| switchyard-example-job | ~20MB |

This is achieved by:
- Using Alpine base images
- Static linking (CGO_ENABLED=0)
- Build-time stripping (`-ldflags="-w -s"`)
- Separate build and runtime stages
- Only copying necessary files to runtime

## Troubleshooting

### Image won't build
- Ensure you're in the repository root
- Check Go version in go.mod matches Dockerfile (currently 1.24)
- Run `go mod tidy` first

### Worker can't create jobs
- Verify Docker socket is mounted: `-v /var/run/docker.sock:/var/run/docker.sock`
- Check Worker has permission to access socket
- In Swarm mode, socket must be available on worker nodes

### Config not found
- Default config path is `/app/config.yaml`
- Mount your config: `-v $(pwd)/config.yaml:/app/config.yaml`
- Or use environment variables to override settings
