# 🚀 TWINUP Sensor System - Complete Setup Guide

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [System Requirements](#system-requirements)
3. [Docker Installation](#docker-installation)
4. [Project Setup](#project-setup)
5. [Environment Configuration](#environment-configuration)
6. [Building the System](#building-the-system)
7. [Running the System](#running-the-system)
8. [Frontend Setup](#frontend-setup)
9. [Testing the System](#testing-the-system)
10. [Monitoring & Metrics](#monitoring--metrics)
11. [Troubleshooting](#troubleshooting)
12. [Development Guide](#development-guide)

---

## 🛠️ Prerequisites

### Required Software
- **Docker** (v20.0+)
- **Docker Compose** (v2.0+)
- **Git** (v2.20+)
- **Go** (v1.21+) - for development
- **Node.js** (v18+) - for frontend
- **npm** (v9+) - for frontend

### Operating System Support
- ✅ **macOS** (Intel & Apple Silicon)
- ✅ **Linux** (Ubuntu 20.04+, CentOS 8+)
- ✅ **Windows** (with WSL2)

---

## 💻 System Requirements

### Minimum Requirements
- **CPU**: 2 cores
- **RAM**: 4GB
- **Storage**: 10GB free space
- **Network**: Internet connection for dependencies

### Recommended Requirements
- **CPU**: 4+ cores
- **RAM**: 8GB+
- **Storage**: 20GB+ free space
- **Network**: Stable broadband connection

---

## 🐳 Docker Installation

### macOS Installation

1. **Download Docker Desktop:**
   ```bash
   # Visit https://docs.docker.com/desktop/mac/install/
   # Or install via Homebrew
   brew install --cask docker
   ```

2. **Start Docker Desktop:**
   ```bash
   open /Applications/Docker.app
   ```

3. **Verify Installation:**
   ```bash
   docker --version
   docker-compose --version
   ```

### Linux Installation (Ubuntu)

1. **Update Package Index:**
   ```bash
   sudo apt-get update
   ```

2. **Install Docker:**
   ```bash
   # Install required packages
   sudo apt-get install ca-certificates curl gnupg lsb-release
   
   # Add Docker's official GPG key
   sudo mkdir -m 0755 -p /etc/apt/keyrings
   curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
   
   # Add Docker repository
   echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
   
   # Install Docker Engine
   sudo apt-get update
   sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
   ```

3. **Start Docker Service:**
   ```bash
   sudo systemctl start docker
   sudo systemctl enable docker
   ```

4. **Add User to Docker Group:**
   ```bash
   sudo usermod -aG docker $USER
   newgrp docker
   ```

### Windows Installation

1. **Enable WSL2:**
   ```powershell
   # Run as Administrator
   wsl --install
   ```

2. **Download Docker Desktop:**
   - Visit: https://docs.docker.com/desktop/windows/install/
   - Download and install Docker Desktop for Windows

3. **Configure WSL2 Backend:**
   - Open Docker Desktop
   - Go to Settings → General
   - Check "Use the WSL 2 based engine"

---

## 📁 Project Setup

### 1. Clone Repository

```bash
# Clone the project
git clone <repository-url>
cd twinup-sensor-system

# Verify project structure
ls -la
```

### 2. Project Structure Overview

```
twinup-sensor-system/
├── services/                    # Microservices
│   ├── alert-handler-service/   # gRPC Alert Processing
│   ├── authentication-service/  # JWT Authentication
│   ├── api-gateway-service/     # HTTP API Gateway
│   ├── sensor-consumer-service/ # Kafka Consumer
│   ├── sensor-producer-service/ # Kafka Producer
│   └── real-time-analytics-service/ # MQTT Analytics
├── frontend/                    # React Frontend
├── configs/                     # Configuration files
├── scripts/                     # Utility scripts
├── docker-compose.yml           # Docker services
├── Makefile                     # Build automation
└── README.md                    # Project documentation
```

---

## ⚙️ Environment Configuration

### 1. Create Environment Files

```bash
# Copy example environment files
cp .env.example .env
```

### 2. Configure Services

#### Authentication Service Configuration
```bash
# Edit configs/auth-config.yaml
vim configs/auth-config.yaml
```

```yaml
http:
  host: "0.0.0.0"
  port: 8080

jwt:
  secret_key: "twinup-super-secret-jwt-key-2023"
  access_token_ttl: "15m"
  refresh_token_ttl: "24h"

redis:
  host: "twinup-redis"
  port: 6379
  password: ""
  database: 0
```

#### API Gateway Configuration
```bash
# Edit configs/gateway-config.yaml
vim configs/gateway-config.yaml
```

```yaml
http:
  host: "0.0.0.0"
  port: 8081

redis:
  host: "twinup-redis"
  port: 6379

clickhouse:
  host: "twinup-clickhouse"
  port: 9000
  database: "sensor_data"
  username: "twinup"
  password: "twinup123"
```

#### Alert Handler Configuration
```bash
# Edit configs/alert-config.yaml
vim configs/alert-config.yaml
```

```yaml
grpc:
  host: "0.0.0.0"
  port: 50051

email:
  enable_dummy: true
  from_email: "alerts@twinup.com"
  from_name: "TWINUP Alert System"

alert:
  temperature_threshold: 90.0
  cooldown_period: "1m"
```

---

## 🏗️ Building the System

## make build
## cd TWİNUPtask/twinup-sensor-system && docker-compose -f deployments/docker/docker-compose.yml up -d


### 1. Build All Services

```bash
# Build all microservices
make build

# Or build individual services
make build-auth
make build-gateway
make build-alert-handler
make build-producer
make build-consumer
make build-analytics
```

### 2. Build Docker Images

```bash
# Build all Docker images
docker-compose build

# Or build specific services
docker-compose build authentication
docker-compose build api-gateway
docker-compose build alert-handler
```

### 3. Verify Builds

```bash
# Check built binaries
ls -la bin/

# Check Docker images
docker images | grep twinup
```

---

## 🚀 Running the System

### 1. Start Infrastructure Services

```bash
# Start databases and message queues first
docker-compose up -d redis clickhouse zookeeper kafka prometheus grafana
```

### 2. Wait for Services to be Ready

```bash
# Check service health
docker-compose ps

# Wait for ClickHouse to be ready
docker-compose logs clickhouse

# Wait for Kafka to be ready
docker-compose logs kafka
```

### 3. Initialize Database

```bash
# Create ClickHouse database and tables
docker exec twinup-clickhouse clickhouse-client --user=twinup --password=twinup123 --query="CREATE DATABASE IF NOT EXISTS sensor_data"

# Create sensor_readings table
docker exec twinup-clickhouse clickhouse-client --user=twinup --password=twinup123 --database=sensor_data --query="
CREATE TABLE IF NOT EXISTS sensor_readings (
    id UUID DEFAULT generateUUIDv4(),
    device_id String,
    timestamp DateTime64(3),
    temperature Float64,
    humidity Float64,
    pressure Float64,
    location_lat Float64,
    location_lng Float64,
    status String DEFAULT 'active',
    created_at DateTime64(3) DEFAULT now64()
) ENGINE = MergeTree()
ORDER BY (device_id, timestamp)
PARTITION BY toYYYYMM(timestamp)
"

# Insert sample data
docker exec twinup-clickhouse clickhouse-client --user=twinup --password=twinup123 --database=sensor_data --query="
INSERT INTO sensor_readings (device_id, timestamp, temperature, humidity, pressure, location_lat, location_lng, status) VALUES 
('sensor-001', now() - INTERVAL 1 HOUR, 25.5, 60.2, 1013.25, 41.0082, 28.9784, 'active'),
('sensor-002', now() - INTERVAL 30 MINUTE, 28.3, 65.1, 1015.30, 41.0083, 28.9785, 'active'),
('test_device_001', now() - INTERVAL 15 MINUTE, 22.1, 55.8, 1012.10, 41.0084, 28.9786, 'active')
"
```

### 4. Start Application Services

```bash
# Start all application services
docker-compose up -d

# Or start services individually
docker-compose up -d authentication
docker-compose up -d api-gateway
docker-compose up -d alert-handler
docker-compose up -d sensor-producer
docker-compose up -d sensor-consumer
docker-compose up -d analytics
```

### 5. Verify All Services

```bash
# Check all services are running
docker-compose ps

# Check service logs
docker-compose logs authentication
docker-compose logs api-gateway
docker-compose logs alert-handler
```

---

## 🌐 Frontend Setup

### 1. Install Frontend Dependencies

```bash
# Navigate to frontend directory
cd frontend

# Install dependencies
npm install

# Verify installation
npm list
```

### 2. Configure Frontend Environment

```bash
# Create environment file
cat > .env.local << EOF
VITE_API_BASE_URL=http://localhost:8081
VITE_AUTH_BASE_URL=http://localhost:8080
VITE_PROMETHEUS_URL=http://localhost:9090
VITE_GRAFANA_URL=http://localhost:3000
EOF
```

### 3. Start Frontend Development Server

```bash
# Start development server
npm run dev

# Frontend will be available at: http://localhost:5173
```

### 4. Build Frontend for Production

```bash
# Build for production
npm run build

# Preview production build
npm run preview
```

---

## 🧪 Testing the System

### 1. Health Checks

```bash
# Test Authentication Service
curl http://localhost:8080/auth/health

# Test API Gateway
curl http://localhost:8081/api/v1/health

# Test Prometheus
curl http://localhost:9090/-/healthy

# Test Grafana
curl http://localhost:3000/api/health
```

### 2. Authentication Flow

```bash
# Test user login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'

# Extract JWT token from response and test verification
TOKEN="<jwt-token-from-login>"
curl -X POST http://localhost:8080/auth/verify \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN"
```

### 3. API Gateway Endpoints

```bash
# Test device list
curl http://localhost:8081/api/v1/devices?page=1&page_size=10

# Test device metrics
curl http://localhost:8081/api/v1/devices/sensor-001/metrics

# Test device analytics
curl http://localhost:8081/api/v1/devices/sensor-001/analytics?type=trend

# Test system stats
curl http://localhost:8081/api/v1/stats
```

### 4. Run Unit Tests

```bash
# Test Authentication Service
cd services/authentication-service
go test ./...

# Test API Gateway Service
cd ../api-gateway-service
go test ./...

# Test Alert Handler Service
cd ../alert-handler-service
go test ./...
```

### 5. Load Testing

```bash
# Install k6 (if not already installed)
# macOS
brew install k6

# Linux
sudo apt-get install k6

# Run load tests
k6 run scripts/load-test.js
```

---

## 📊 Monitoring & Metrics

### 1. Access Monitoring Dashboards

#### Prometheus Metrics
- **URL**: http://localhost:9090
- **Features**: Raw metrics, PromQL queries, targets monitoring

#### Grafana Dashboards
- **URL**: http://localhost:3000
- **Default Login**: admin/admin
- **Features**: Visual dashboards, alerts, data exploration

### 2. Key Metrics to Monitor

#### Application Metrics
```promql
# Authentication metrics
auth_login_success_total
auth_tokens_issued_total
auth_token_validation_latency

# Gateway metrics
gateway_http_requests_total
gateway_http_latency
gateway_cache_requests_total

# Alert handler metrics
alert_handler_alerts_received_total
alert_handler_emails_sent_total
alert_handler_processing_latency
```

#### System Metrics
```promql
# Container metrics
container_cpu_usage_seconds_total
container_memory_usage_bytes

# Database metrics
clickhouse_query_duration_seconds
redis_connected_clients
```

### 3. Setting Up Alerts

```yaml
# Example alert rules (prometheus/alerts.yml)
groups:
- name: twinup_alerts
  rules:
  - alert: HighErrorRate
    expr: rate(gateway_http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      description: "Error rate is {{ $value }} errors per second"
```

---

## 🔧 Troubleshooting

### Common Issues

#### 1. Docker Issues

**Problem**: Container fails to start
```bash
# Check container logs
docker-compose logs <service-name>

# Check container status
docker-compose ps

# Restart specific service
docker-compose restart <service-name>
```

**Problem**: Port conflicts
```bash
# Check port usage
lsof -i :8080
netstat -tulpn | grep :8080

# Kill process using port
kill -9 <PID>
```

#### 2. Database Connection Issues

**Problem**: ClickHouse connection failed
```bash
# Check ClickHouse container
docker-compose logs clickhouse

# Test ClickHouse connection
docker exec twinup-clickhouse clickhouse-client --query="SELECT 1"

# Check network connectivity
docker network ls
docker network inspect docker_twinup-network
```

**Problem**: Redis connection failed
```bash
# Check Redis container
docker-compose logs redis

# Test Redis connection
docker exec twinup-redis redis-cli ping
```

#### 3. Service-Specific Issues

**Problem**: Authentication service not responding
```bash
# Check service logs
docker-compose logs authentication

# Verify configuration
docker exec twinup-auth cat /app/configs/config.yaml

# Test health endpoint
curl http://localhost:8080/auth/health
```

**Problem**: Frontend can't connect to API
```bash
# Check CORS configuration
curl -H "Origin: http://localhost:5173" \
     -H "Access-Control-Request-Method: GET" \
     -H "Access-Control-Request-Headers: X-Requested-With" \
     -X OPTIONS \
     http://localhost:8081/api/v1/health

# Check frontend environment variables
cat frontend/.env.local
```

### 4. Performance Issues

**Problem**: High memory usage
```bash
# Check container resource usage
docker stats

# Check Go memory profiles
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Problem**: Slow response times
```bash
# Check Prometheus metrics
curl http://localhost:9090/api/v1/query?query=gateway_http_latency

# Enable debug logging
# Edit config files to set log level to "debug"
```

---

## 👨‍💻 Development Guide

### 1. Local Development Setup

```bash
# Start only infrastructure services
docker-compose up -d redis clickhouse kafka zookeeper prometheus grafana

# Run services locally for development
cd services/authentication-service
go run cmd/main.go --config ../../configs/auth-config.yaml

cd services/api-gateway-service
go run cmd/main.go --config ../../configs/gateway-config.yaml
```

### 2. Code Style and Standards

```bash
# Format Go code
gofmt -w .

# Run linters
golangci-lint run

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 3. Adding New Services

1. **Create service directory:**
   ```bash
   mkdir services/new-service
   cd services/new-service
   ```

2. **Initialize Go module:**
   ```bash
   go mod init github.com/twinup/sensor-system/services/new-service
   ```

3. **Create basic structure:**
   ```bash
   mkdir -p cmd internal/config internal/handler pkg
   ```

4. **Add to Makefile:**
   ```makefile
   build-new-service:
   	cd services/new-service && go build -o ../../bin/new-service ./cmd/
   ```

5. **Add to docker-compose.yml:**
   ```yaml
   new-service:
     build:
       context: .
       dockerfile: services/new-service/Dockerfile
     ports:
       - "8082:8082"
     networks:
       - twinup-network
   ```

### 4. Database Migrations

```bash
# Create migration files
mkdir -p migrations

# Example migration
cat > migrations/001_create_sensor_readings.sql << EOF
CREATE TABLE IF NOT EXISTS sensor_readings (
    id UUID DEFAULT generateUUIDv4(),
    device_id String,
    timestamp DateTime64(3),
    -- ... other fields
) ENGINE = MergeTree()
ORDER BY (device_id, timestamp);
EOF
```

### 5. Testing Strategies

#### Unit Tests
```go
func TestAuthenticationService(t *testing.T) {
    // Setup test dependencies
    // Run tests
    // Assert results
}
```

#### Integration Tests
```go
func TestAPIGatewayIntegration(t *testing.T) {
    // Start test containers
    // Test API endpoints
    // Verify database state
}
```

#### End-to-End Tests
```bash
# Use Cypress or similar for frontend E2E tests
npm run test:e2e
```

---

## 🎯 Quick Start Commands

### Complete System Startup
```bash
# Clone and setup
git clone <repo-url> && cd twinup-sensor-system

# Start all services
docker-compose up -d

# Initialize database
./scripts/init-database.sh

# Start frontend
cd frontend && npm install && npm run dev
```

### Development Mode
```bash
# Infrastructure only
docker-compose up -d redis clickhouse kafka prometheus grafana

# Run services locally
make run-local
```

### Production Deployment
```bash
# Build production images
docker-compose -f docker-compose.prod.yml build

# Deploy to production
docker-compose -f docker-compose.prod.yml up -d
```

---

## 📚 Additional Resources

### Documentation
- [API Documentation](./docs/api.md)
- [Architecture Overview](./docs/architecture.md)
- [Deployment Guide](./docs/deployment.md)

### External Links
- [Docker Documentation](https://docs.docker.com/)
- [Go Documentation](https://golang.org/doc/)
- [React Documentation](https://reactjs.org/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)

### Support
- **Issues**: Create GitHub issues for bugs
- **Discussions**: Use GitHub discussions for questions
- **Wiki**: Check project wiki for additional guides

---

## 🏆 Success Criteria

After completing this setup, you should have:

✅ **All services running and healthy**  
✅ **Frontend accessible at http://localhost:5173**  
✅ **API endpoints responding correctly**  
✅ **Monitoring dashboards available**  
✅ **Database with sample data**  
✅ **Authentication flow working**  
✅ **Real-time metrics collection**  

**🎉 Congratulations! Your TWINUP Sensor System is now fully operational!**

---

*Version: 1.0.0*


deneme
