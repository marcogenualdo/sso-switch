.PHONY: help build test clean run docker docker-run lint fmt vet check deps

BINARY_NAME=sso-proxy
DOCKER_IMAGE=sso-proxy:latest
CONFIG_PATH=./examples/config.yaml

help:
	@echo "SSO Proxy - Makefile commands:"
	@echo "  make build       - Build the binary"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make run         - Run the proxy locally"
	@echo "  make docker      - Build Docker image"
	@echo "  make docker-run  - Run with docker-compose"
	@echo "  make lint        - Run golangci-lint"
	@echo "  make fmt         - Format code"
	@echo "  make vet         - Run go vet"
	@echo "  make check       - Run all checks (fmt, vet, test)"
	@echo "  make deps        - Download dependencies"

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) ./cmd/sso-proxy
	@echo "Build complete: $(BINARY_NAME)"

build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o $(BINARY_NAME) ./cmd/sso-proxy
	@echo "Build complete: $(BINARY_NAME)"

test:
	@echo "Running tests..."
	@go test -v -race -cover ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@go clean
	@echo "Clean complete"

run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME) --config $(CONFIG_PATH)

docker:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-run:
	@echo "Starting services with docker-compose..."
	@docker-compose up -d
	@echo "Services started. Proxy available at http://localhost:8080"
	@echo "Run 'docker-compose logs -f' to view logs"

docker-stop:
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped"

docker-logs:
	@docker-compose logs -f

lint:
	@echo "Running golangci-lint..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Install: https://golangci-lint.run/usage/install/"; exit 1; }
	@golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Vet complete"

check: fmt vet test
	@echo "All checks passed!"

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

install:
	@echo "Installing $(BINARY_NAME)..."
	@go install ./cmd/sso-proxy
	@echo "Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

.DEFAULT_GOAL := help
