# Multi-stage Dockerfile for Switchyard Worker service
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

# Build the Worker binary
# CGO_ENABLED=0 for static binary, GOOS=linux for Linux target
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -o worker \
    ./cmd/worker

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd

# Create non-root user
RUN addgroup -g 1000 switchyard && \
    adduser -D -u 1000 -G switchyard switchyard

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/worker /app/worker

# Copy example config (can be overridden with volume mount)
COPY --from=builder /build/config.example.yaml /app/config.example.yaml

# Copy entrypoint
COPY --from=builder /build/build/worker-entrypoint.sh /app/entrypoint.sh

# Change ownership
RUN chown -R switchyard:switchyard /app

# Ensure entrypoint is executable
RUN chmod +x /app/entrypoint.sh

# Switch to non-root user
USER switchyard

# No ports to expose (worker connects to queue)

# Run the worker
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["-config", "/app/config.yaml"]
