# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` contains service entrypoints: `cmd/api`, `cmd/worker`, `cmd/migrate`.
- `internal/` holds core packages: `api`, `worker`, `executor`, `storage`, `config`, `domain`.
- `migrations/` stores SQL migration files.
- `deployments/` includes Docker Compose/Swarm configs and deployment docs.
- `build/` holds Dockerfiles and example-job assets.
- `examples/` includes helper scripts for API usage.
- Configuration lives in `config.yaml`; use `config.example.yaml` as a template.

## Build, Test, and Development Commands
- `make build` builds binaries into `bin/`.
- `make test` runs `go test -race -coverprofile=coverage.out ./...`.
- `make dev-up` starts local services with Docker Compose and applies migrations.
- `make dev-down` stops local services and removes volumes.
- `make migrate-up` / `make migrate-down` apply or roll back migrations.
- `make fmt` runs `go fmt ./...` and `gofmt -s -w .`.
- `make lint` runs `golangci-lint run ./...` (non-blocking).
- `make docker-build` builds API/worker/example images.

## Coding Style & Naming Conventions
- Use standard Go formatting (`gofmt -s`) and `go fmt`.
- Package names are lowercase; exported identifiers use `CamelCase`, unexported use `lowerCamel`.
- Match existing file naming patterns like `handlers_job.go` and `processor_test.go`.

## Testing Guidelines
- Frameworks: Go `testing`, `testify`, and `testcontainers-go` (see `TESTING.md`).
- Test files use the `*_test.go` naming convention.
- Run all tests with `go test ./...` or `make test`.
- For coverage reports: `go test ./... -coverprofile=coverage.out` then `go tool cover -html=coverage.out`.
- Integration tests live under `internal/storage/postgres/` and may require Docker.

## Commit & Pull Request Guidelines
- Follow Conventional Commits as seen in history: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`.
- PRs should include a concise summary and the tests you ran (or why not).
- Call out any config or migration changes explicitly in the PR description.

## Security & Configuration Tips
- Do not commit secrets. Use env vars or Docker secrets for API keys and credentials.
- Keep `config.example.yaml` updated when new config fields are added.
