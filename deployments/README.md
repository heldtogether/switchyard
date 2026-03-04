# Switchyard Deployment Files

This directory contains Swarm deployment configs and supporting docs.

## Files
- `stack.yml`: Swarm stack definition (API, Worker, Redis, networks, secrets)
- `docker-compose.yml`: Local dev Postgres + Redis
- `config.yaml`: Production config template
- `DEPLOYMENT.md`: Full deployment guide

## Quick Start (Swarm)
```bash
cd deployments

# Secrets
printf "%s" "your-api-key" | docker secret create switchyard_api_key -
printf "%s" "your-s3-access-key" | docker secret create switchyard_s3_access_key -
printf "%s" "your-s3-secret-key" | docker secret create switchyard_s3_secret_key -
# Optional for OIDC modes:
# export AUTH_MODE=hybrid
# export OIDC_ISSUER_URL=...
# export OIDC_CLIENT_ID=...
# export OIDC_CLIENT_SECRET=...
# export OIDC_REDIRECT_URL=https://api.example.com/v1/auth/callback
# export OIDC_SESSION_SIGNING_KEY=... # strong random string

# Env
cp .env.example .env
# edit .env with database and S3 values

# Deploy
docker stack deploy -c stack.yml switchyard

# Verify
curl http://localhost:8080/healthz
```
UI will be available at `http://localhost:3000`.

## Local Dev Services
```bash
make dev-up
make dev-down
```

For full details, prerequisites, and ops guidance, see `DEPLOYMENT.md`.
