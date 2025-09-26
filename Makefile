# TWINUP Sensor System - Makefile
# Mükemmel bir şekilde tüm operasyonları yönetir

.PHONY: help build test clean docker-build docker-up docker-down proto lint fmt vet deps check-deps

# Default target
.DEFAULT_GOAL := help

# Variables
DOCKER_COMPOSE_FILE := deployments/docker/docker-compose.yml
SERVICES := sensor-producer-service sensor-consumer-service real-time-analytics-service alert-handler-service authentication-service api-gateway-service
GO_VERSION := 1.21
GOLANGCI_LINT_VERSION := v1.54.2

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

## Help
help: ## Show this help message
	@echo "$(BLUE)TWINUP Sensor System - Available Commands$(NC)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

## Dependencies
deps: ## Install all dependencies
	@echo "$(YELLOW)Installing dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)Dependencies installed successfully$(NC)"

check-deps: ## Check if all required tools are installed
	@echo "$(YELLOW)Checking dependencies...$(NC)"
	@command -v go >/dev/null 2>&1 || { echo "$(RED)Go is not installed$(NC)"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "$(RED)Docker is not installed$(NC)"; exit 1; }
	@command -v docker-compose >/dev/null 2>&1 || { echo "$(RED)Docker Compose is not installed$(NC)"; exit 1; }
	@echo "$(GREEN)All dependencies are available$(NC)"

## Development
fmt: ## Format Go code
	@echo "$(YELLOW)Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)Code formatted$(NC)"

vet: ## Run go vet
	@echo "$(YELLOW)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)go vet completed$(NC)"

lint: ## Run golangci-lint
	@echo "$(YELLOW)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "$(RED)golangci-lint not found. Installing...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
		golangci-lint run ./...; \
	fi
	@echo "$(GREEN)Linting completed$(NC)"

## Testing
test: ## Run all tests
	@echo "$(YELLOW)Running tests...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)Tests completed$(NC)"

test-coverage: test ## Run tests with coverage report
	@echo "$(YELLOW)Generating coverage report...$(NC)"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

test-bench: ## Run benchmark tests
	@echo "$(YELLOW)Running benchmark tests...$(NC)"
	@go test -bench=. -benchmem ./...
	@echo "$(GREEN)Benchmark tests completed$(NC)"

## Building
build: ## Build all services
	@echo "$(YELLOW)Building all services...$(NC)"
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		cd services/$$service && go build -o ../../bin/$$service ./cmd/main.go && cd ../..; \
	done
	@echo "$(GREEN)All services built successfully$(NC)"

build-producer: ## Build only sensor producer service
	@echo "$(YELLOW)Building sensor producer service...$(NC)"
	@mkdir -p bin
	@cd services/sensor-producer-service && go build -o ../../bin/sensor-producer ./cmd/main.go
	@echo "$(GREEN)Sensor producer built successfully$(NC)"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@go clean ./...
	@echo "$(GREEN)Clean completed$(NC)"

## Protocol Buffers
proto: ## Generate gRPC code from proto files
	@echo "$(YELLOW)Generating gRPC code...$(NC)"
	@if command -v protoc >/dev/null 2>&1; then \
		protoc --go_out=. --go_opt=paths=source_relative \
		       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		       proto/common/*.proto proto/alert/*.proto; \
	else \
		echo "$(RED)protoc not found. Please install Protocol Buffers compiler$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)gRPC code generated$(NC)"

## Docker Operations
docker-build: ## Build all Docker images
	@echo "$(YELLOW)Building Docker images...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) build
	@echo "$(GREEN)Docker images built successfully$(NC)"

docker-build-producer: ## Build only producer Docker image
	@echo "$(YELLOW)Building producer Docker image...$(NC)"
	@cd services/sensor-producer-service && docker build -t twinup/sensor-producer:latest .
	@echo "$(GREEN)Producer Docker image built$(NC)"

docker-up: ## Start all services with Docker Compose
	@echo "$(YELLOW)Starting services with Docker Compose...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) up -d
	@echo "$(GREEN)All services started$(NC)"
	@echo "$(BLUE)Services available at:$(NC)"
	@echo "  - Grafana: http://localhost:3000 (admin/twinup123)"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - ClickHouse: http://localhost:8123"
	@echo "  - API Gateway: http://localhost:8081"

docker-up-infra: ## Start only infrastructure services (Kafka, Redis, ClickHouse, etc.)
	@echo "$(YELLOW)Starting infrastructure services...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) up -d zookeeper kafka redis clickhouse prometheus grafana mosquitto
	@echo "$(GREEN)Infrastructure services started$(NC)"

docker-down: ## Stop all services
	@echo "$(YELLOW)Stopping all services...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "$(GREEN)All services stopped$(NC)"

docker-down-volumes: ## Stop all services and remove volumes
	@echo "$(YELLOW)Stopping services and removing volumes...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) down -v
	@echo "$(GREEN)Services stopped and volumes removed$(NC)"

docker-logs: ## Show logs for all services
	@docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f

docker-logs-producer: ## Show logs for producer service
	@docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f sensor-producer

## Development Helpers
run-producer: build-producer ## Run producer service locally
	@echo "$(YELLOW)Running sensor producer locally...$(NC)"
	@./bin/sensor-producer --config configs/producer-config.yaml

dev-setup: check-deps docker-up-infra ## Setup development environment
	@echo "$(YELLOW)Setting up development environment...$(NC)"
	@sleep 10 # Wait for services to start
	@echo "$(GREEN)Development environment ready$(NC)"
	@echo "$(BLUE)You can now run services locally or with Docker$(NC)"

## Monitoring
monitoring-up: ## Start only monitoring services
	@echo "$(YELLOW)Starting monitoring services...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) up -d prometheus grafana
	@echo "$(GREEN)Monitoring services started$(NC)"

kafka-topics: ## List Kafka topics
	@echo "$(YELLOW)Listing Kafka topics...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) exec kafka kafka-topics --bootstrap-server localhost:9092 --list

kafka-create-topic: ## Create sensor-data topic
	@echo "$(YELLOW)Creating sensor-data topic...$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) exec kafka kafka-topics --bootstrap-server localhost:9092 --create --topic sensor-data --partitions 10 --replication-factor 1

## Database
clickhouse-client: ## Connect to ClickHouse client
	@docker-compose -f $(DOCKER_COMPOSE_FILE) exec clickhouse clickhouse-client --user twinup --password twinup123 --database sensor_data

redis-cli: ## Connect to Redis CLI
	@docker-compose -f $(DOCKER_COMPOSE_FILE) exec redis redis-cli

## Quality Assurance
qa: fmt vet lint test ## Run all quality assurance checks
	@echo "$(GREEN)All QA checks completed successfully$(NC)"

ci: check-deps deps proto qa build ## Run CI pipeline locally
	@echo "$(GREEN)CI pipeline completed successfully$(NC)"

## Utilities
status: ## Show status of all services
	@echo "$(BLUE)Service Status:$(NC)"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) ps

health-check: ## Check health of all services
	@echo "$(YELLOW)Checking service health...$(NC)"
	@echo "Kafka:"
	@curl -f http://localhost:9092 >/dev/null 2>&1 && echo "  $(GREEN)✓ Healthy$(NC)" || echo "  $(RED)✗ Unhealthy$(NC)"
	@echo "Redis:"
	@docker-compose -f $(DOCKER_COMPOSE_FILE) exec redis redis-cli ping >/dev/null 2>&1 && echo "  $(GREEN)✓ Healthy$(NC)" || echo "  $(RED)✗ Unhealthy$(NC)"
	@echo "ClickHouse:"
	@curl -f http://localhost:8123/ping >/dev/null 2>&1 && echo "  $(GREEN)✓ Healthy$(NC)" || echo "  $(RED)✗ Unhealthy$(NC)"
	@echo "Prometheus:"
	@curl -f http://localhost:9090/-/healthy >/dev/null 2>&1 && echo "  $(GREEN)✓ Healthy$(NC)" || echo "  $(RED)✗ Unhealthy$(NC)"
	@echo "Grafana:"
	@curl -f http://localhost:3000/api/health >/dev/null 2>&1 && echo "  $(GREEN)✓ Healthy$(NC)" || echo "  $(RED)✗ Unhealthy$(NC)"

load-test: ## Run load test on the system
	@echo "$(YELLOW)Running load test...$(NC)"
	@echo "$(RED)Load test implementation needed$(NC)"

## Documentation
docs: ## Generate documentation
	@echo "$(YELLOW)Generating documentation...$(NC)"
	@go doc -all ./... > docs/api.md
	@echo "$(GREEN)Documentation generated$(NC)"

## Version
version: ## Show version information
	@echo "$(BLUE)TWINUP Sensor System$(NC)"
	@echo "Version: 1.0.0"
	@echo "Go Version: $(shell go version)"
	@echo "Docker Version: $(shell docker --version)"
	@echo "Docker Compose Version: $(shell docker-compose --version)"
