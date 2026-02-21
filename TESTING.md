# Switchyard Testing Guide

## Frameworks
- Go `testing` package
- `stretchr/testify` for assertions
- `testcontainers-go` for integration tests (Postgres)

## Running Tests
```bash
# All tests
make test

# Standard Go test
go test ./...

# With coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Race detector
go test -race ./...
```

## Integration Tests
- Postgres integration tests live under `internal/storage/postgres/`.
- These tests require Docker and will spin up containers via `testcontainers-go`.

## Conventions
- Test files use the `*_test.go` suffix.
- Prefer table-driven tests for parsing/validation logic.
