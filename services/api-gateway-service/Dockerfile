# TWINUP API Gateway Service - Multi-stage Docker build
# Optimized for high-performance REST API with caching

# ============================================================================
# Build Stage
# ============================================================================
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version information
ARG VERSION=1.0.0
ARG BUILD_TIME
ARG GIT_COMMIT

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -a -installsuffix cgo \
    -o api-gateway \
    ./cmd/main.go

# ============================================================================
# Runtime Stage
# ============================================================================
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1001 -S twinup && \
    adduser -u 1001 -S twinup -G twinup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/api-gateway .

# Copy configuration files (if any)
COPY --from=builder /build/configs/ ./configs/

# Set ownership
RUN chown -R twinup:twinup /app

# Switch to non-root user
USER twinup

# Expose HTTP port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/api/v1/health || exit 1

# Set default environment variables
ENV TWINUP_HTTP_HOST=0.0.0.0
ENV TWINUP_HTTP_PORT=8081
ENV TWINUP_REDIS_HOST=redis:6379
ENV TWINUP_REDIS_DATABASE=0
ENV TWINUP_CLICKHOUSE_HOST=clickhouse:9000
ENV TWINUP_CLICKHOUSE_DATABASE=sensor_data
ENV TWINUP_CLICKHOUSE_USERNAME=twinup
ENV TWINUP_CLICKHOUSE_PASSWORD=twinup123
ENV TWINUP_GATEWAY_ENABLE_CACHING=true
ENV TWINUP_GATEWAY_CACHE_DEFAULT_TTL=5m
ENV TWINUP_GATEWAY_FALLBACK_TO_CLICKHOUSE=true
ENV TWINUP_GATEWAY_ENABLE_PAGINATION=true
ENV TWINUP_GATEWAY_DEFAULT_PAGE_SIZE=100
ENV TWINUP_GATEWAY_MAX_PAGE_SIZE=1000
ENV TWINUP_AUTH_ENABLE_AUTH=true
ENV TWINUP_LOGGER_LEVEL=info
ENV TWINUP_METRICS_ENABLED=true
ENV TWINUP_METRICS_PORT=8081

# Run the application
ENTRYPOINT ["./api-gateway"]
CMD ["--config", "configs/config.yaml"]
