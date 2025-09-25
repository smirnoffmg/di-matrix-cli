# syntax=docker/dockerfile:1

# Build stage: Compile Go application
FROM golang:1.25-alpine AS builder

# Set build arguments for cross-compilation
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Set environment variables for Go build
ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GO111MODULE=on

# Install git and ca-certificates for dependency fetching
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -a -ldflags="-s -w" -installsuffix cgo -o di-matrix-cli ./cmd

# Test stage: Run tests
FROM builder AS test-stage
RUN go test -v ./...

# Lint stage: Run golangci-lint
FROM builder AS lint-stage
RUN apk add --no-cache curl
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
RUN golangci-lint run

# Final stage: Create minimal runtime image
FROM alpine:3.21 AS final

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Environment variables for runtime configuration
# These can be overridden at runtime
ENV GITLAB_BASE_URL="https://gitlab.com"
ENV GITLAB_TOKEN=""
ENV OUTPUT_HTML_FILE="dependency-matrix.html"
ENV OUTPUT_TITLE="Dependency Matrix Report"
ENV ANALYSIS_TIMEOUT_MINUTES="10"

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/di-matrix-cli /app/di-matrix-cli

# Copy example config file
COPY --from=builder /app/config.example.yaml /app/config.example.yaml

# Create config directory for runtime mounting
RUN mkdir -p /app/config

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Set the binary as executable
RUN chmod +x /app/di-matrix-cli

# Default command with config file
ENTRYPOINT ["/app/di-matrix-cli", "analyze", "--config", "/app/config/config.yaml"]
