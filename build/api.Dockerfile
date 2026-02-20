# Multi-stage Dockerfile for Switchyard API service
# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the API binary
# CGO_ENABLED=0 for static binary, GOOS=linux for Linux target
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -o api \
    ./cmd/api

# Build the migrate binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o migrate \
    ./cmd/migrate

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 switchyard && \
    adduser -D -u 1000 -G switchyard switchyard

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/api /app/api
COPY --from=builder /build/migrate /app/migrate

# Copy migrations (needed if we want to run migrations from the container)
COPY --from=builder /build/migrations /app/migrations

# Copy example config (can be overridden with volume mount)
COPY --from=builder /build/config.example.yaml /app/config.example.yaml

# Change ownership
RUN chown -R switchyard:switchyard /app

# Switch to non-root user
USER switchyard

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the API server
ENTRYPOINT ["/app/api"]
CMD ["-config", "/app/config.yaml"]
