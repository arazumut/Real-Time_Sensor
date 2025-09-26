# TWINUP Sensor Producer Service - Multi-stage Docker build
# Optimized for production deployment with minimal image size

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
    -o sensor-producer \
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
COPY --from=builder /build/sensor-producer .

# Copy configuration files (if any)
COPY --from=builder /build/configs/ ./configs/

# Set ownership
RUN chown -R twinup:twinup /app

# Switch to non-root user
USER twinup

# Expose metrics port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set default environment variables
ENV TWINUP_KAFKA_BROKERS=kafka:29092
ENV TWINUP_KAFKA_TOPIC=sensor-data
ENV TWINUP_PRODUCER_DEVICE_COUNT=10000
ENV TWINUP_PRODUCER_BATCH_SIZE=1000
ENV TWINUP_PRODUCER_PRODUCE_INTERVAL=1s
ENV TWINUP_LOGGER_LEVEL=info
ENV TWINUP_METRICS_ENABLED=true
ENV TWINUP_METRICS_PORT=8080

# Run the application
ENTRYPOINT ["./sensor-producer"]
CMD ["--config", "configs/config.yaml"]
