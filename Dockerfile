# syntax=docker/dockerfile:1

# Build stage: Compile Go application
FROM golang:1.25-alpine AS builder

# Set build arguments for cross-compilation and version info
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Set environment variables for Go build
ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GO111MODULE=on

# Install git, ca-certificates, curl, and make for dependency fetching, linting, and building
RUN apk add --no-cache git ca-certificates tzdata curl make

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Install golangci-lint for linting (latest version compatible with Go 1.25)
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Install gotestsum for better test output
RUN go install gotest.tools/gotestsum@latest

# Build the application using Makefile to ensure quality gates
RUN make build VERSION=${VERSION} COMMIT=${COMMIT} BUILD_TIME=${BUILD_TIME}

# Test stage: Run tests using Makefile
FROM builder AS test-stage
RUN make coverage

# Lint stage: Run golangci-lint using Makefile
FROM builder AS lint-stage
RUN make lint

# Final stage: Create minimal runtime image
FROM alpine:3.21 AS final

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Environment variables for runtime configuration
# These can be overridden at runtime
ENV GITLAB_BASE_URL="https://gitlab.com"
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

# Default command - can be overridden
ENTRYPOINT ["/app/di-matrix-cli"]
CMD ["analyze", "--config", "/app/config/config.yaml"]
