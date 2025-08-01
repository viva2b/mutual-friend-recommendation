# Makefile for Mutual Friend System

.PHONY: help build test clean docker-up docker-down proto install-deps

# Variables
APP_NAME := mutual-friend
DOCKER_COMPOSE := docker-compose
GO_FILES := $(shell find . -name "*.go" -type f)

# Default target
help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-deps: ## Install Go dependencies
	go mod download
	go mod tidy

build: ## Build the application
	go build -o bin/server cmd/server/main.go
	go build -o bin/batch cmd/batch/main.go

test: ## Run tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-up: ## Start all services with Docker Compose
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop all services
	$(DOCKER_COMPOSE) down

docker-restart: docker-down docker-up ## Restart all services

docker-logs: ## Show logs for all services
	$(DOCKER_COMPOSE) logs -f

docker-clean: ## Clean Docker containers and volumes
	$(DOCKER_COMPOSE) down -v --remove-orphans
	docker system prune -f

proto-install: ## Install protoc and Go plugins
	@echo "Installing protoc..."
	# macOS
	brew install protobuf
	# Install Go plugins
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto: ## Generate protobuf files
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/*.proto

dev: docker-up ## Start development environment
	@echo "Development environment started!"
	@echo "Services:"
	@echo "  - DynamoDB: http://localhost:8000"
	@echo "  - DynamoDB Admin: http://localhost:8001"
	@echo "  - RabbitMQ Management: http://localhost:15672 (admin/admin123)"
	@echo "  - Elasticsearch: http://localhost:9200"
	@echo "  - Redis: localhost:6379"
	@echo "  - Redis Commander: http://localhost:8081"

lint: ## Run linters
	golangci-lint run

fmt: ## Format Go code
	go fmt ./...

run-server: ## Run the gRPC server
	go run cmd/server/main.go

run-batch: ## Run the batch processor
	go run cmd/batch/main.go

test-events: ## Test RabbitMQ event publishing
	go run cmd/test-events/main.go

test-db: ## Test DynamoDB setup and data
	go run cmd/setup/main.go

test-query: ## Query and analyze DynamoDB data
	go run cmd/query/main.go

.DEFAULT_GOAL := help