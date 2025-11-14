.PHONY: help build test lint clean docker-build docker-up docker-down

help:
	@echo "1C Log Parser Service - Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build          - Build both parser and mcp binaries"
	@echo "  test           - Run all tests"
	@echo "  lint           - Run linters"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start Docker Compose stack"
	@echo "  docker-down    - Stop Docker Compose stack"
	@echo "  mod-tidy       - Run go mod tidy"

build:
	@echo "Building parser..."
	@go build -o bin/parser.exe ./cmd/parser
	@echo "Building MCP server..."
	@go build -o bin/mcp.exe ./cmd/mcp
	@echo "Building extract_mxl utility..."
	@go build -o bin/extract_mxl.exe ./cmd/extract_mxl
	@echo "Building compare utility..."
	@go build -o bin/compare.exe ./cmd/compare
	@echo "Done!"

test:
	@echo "Running tests..."
	@go test ./... -v -cover

lint:
	@echo "Running linters..."
	@go fmt ./...
	@go vet ./...
	@golangci-lint run

clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf build/
	@go clean

docker-build:
	@echo "Building Docker images..."
	@cd deploy/docker && docker-compose build

docker-up:
	@echo "Starting Docker Compose stack..."
	@cd deploy/docker && docker-compose up -d
	@echo "Services started:"
	@echo "  - ClickHouse: http://localhost:8123"
	@echo "  - Grafana: http://localhost:3000"
	@echo "  - MCP Server: http://localhost:8080"

docker-down:
	@echo "Stopping Docker Compose stack..."
	@cd deploy/docker && docker-compose down

mod-tidy:
	@echo "Running go mod tidy..."
	@go mod tidy

