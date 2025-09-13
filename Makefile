include .env
export

.PHONY: help build test clean lint fmt run install docker-build docker-test docker-lint docker-run

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the CLI application
	go build -o di-matrix-cli ./cmd

test: ## Run tests
	go test ./... -v

test-integration: ## Run integration tests (requires environment variables)
	go test ./tests/integration/... -v -tags=integration

test-unit: ## Run unit tests only
	go test ./internal/... -v

clean: ## Clean build artifacts
	rm -f di-matrix-cli coverage.out coverage.html

lint: ## Run linters and formatters
	golangci-lint run

fmt: ## Format code using golangci-lint
	golangci-lint fmt

debug-env: ## Debug environment variables
	@echo "GITLAB_TOKEN=$$GITLAB_TOKEN"
	@echo "GITLAB_BASE_URL=$$GITLAB_BASE_URL"

run: build ## Build and run
	@if [ ! -f config.yaml ]; then cp config.example.yaml config.yaml; fi
	./di-matrix-cli analyze --config config.yaml --timeout 1

install: ## Install to GOPATH/bin
	go install ./cmd

# Docker targets for CI systems
docker-build: ## Build Docker image
	docker build -t di-matrix-cli:latest .

docker-test: ## Run tests in Docker container
	docker build --target test-stage -t di-matrix-cli:test .

docker-lint: ## Run linters in Docker container
	docker build --target lint-stage -t di-matrix-cli:lint .

docker-run: ## Run the application in Docker container
	docker run --rm -v $(PWD)/config.yaml:/app/config.yaml di-matrix-cli:latest analyze --config /app/config.yaml

docker-ci: ## Run full CI pipeline in Docker (build, test, lint)
	docker build --target test-stage -t di-matrix-cli:test .
	docker build --target lint-stage -t di-matrix-cli:lint .
	docker build -t di-matrix-cli:latest .
