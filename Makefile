# Load environment variables from .env if it exists
-include .env
export

# Project variables (can be overridden by .env or command line)
PROJECT_ID ?= $(shell gcloud config get-value project)
REGION ?= asia-northeast1
AR_REPO ?= youtube-trend-repo
SERVICE_NAME ?= youtube-trend-tracker
IMAGE_TAG ?= latest
BQ_DATASET ?= youtube

# Go variables
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod
BINARY_NAME = fetcher
MAIN_PATH = ./cmd/fetcher

# Colors for output
RED = \033[0;31m
GREEN = \033[0;32m
YELLOW = \033[1;33m
NC = \033[0m # No Color

.PHONY: all build clean test coverage lint fmt vet run docker-build docker-push deploy help

## help: Display this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/## //'

## all: Build and test the project
all: test build

## build: Build the binary
build:
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PATH)

## clean: Remove build artifacts
clean:
	@echo "$(YELLOW)Cleaning...$(NC)"
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

## test: Run all tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	$(GOTEST) -v -short ./...

## test-all: Run all tests including integration tests
test-all:
	@echo "$(GREEN)Running all tests...$(NC)"
	$(GOTEST) -v ./...

## coverage: Run tests with coverage
coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## coverage-summary: Display coverage summary
coverage-summary:
	@echo "$(GREEN)Coverage summary:$(NC)"
	$(GOTEST) -coverprofile=coverage.out ./... 2>/dev/null
	@$(GOCMD) tool cover -func=coverage.out | tail -1

## lint: Run golangci-lint
lint:
	@echo "$(GREEN)Running linter...$(NC)"
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not installed. Install with:$(NC)"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin"; \
	fi

## fmt: Format code
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GOCMD) vet ./...

## mod-tidy: Tidy go modules
mod-tidy:
	@echo "$(GREEN)Tidying modules...$(NC)"
	$(GOMOD) tidy

## mod-download: Download dependencies
mod-download:
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	$(GOMOD) download

## run: Run the application locally
run: build
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	./$(BINARY_NAME)

## docker-build: Build Docker image
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(REGION)-docker.pkg.dev/$(PROJECT_ID)/$(AR_REPO)/$(SERVICE_NAME):$(IMAGE_TAG) .

## docker-push: Push Docker image to Artifact Registry
docker-push:
	@echo "$(GREEN)Pushing Docker image to Artifact Registry...$(NC)"
	docker push $(REGION)-docker.pkg.dev/$(PROJECT_ID)/$(AR_REPO)/$(SERVICE_NAME):$(IMAGE_TAG)

## docker-run: Run Docker container locally
docker-run:
	@echo "$(GREEN)Running Docker container locally...$(NC)"
	docker run --rm -p 8080:8080 \
		-e GOOGLE_CLOUD_PROJECT=$(PROJECT_ID) \
		-e YOUTUBE_API_KEY=$(YOUTUBE_API_KEY) \
		-e MAX_VIDEOS_PER_CHANNEL=200 \
		$(REGION)-docker.pkg.dev/$(PROJECT_ID)/$(AR_REPO)/$(SERVICE_NAME):$(IMAGE_TAG)

## deploy: Deploy to Cloud Run
deploy: docker-build docker-push
	@echo "$(GREEN)Deploying to Cloud Run...$(NC)"
	./scripts/deploy-cloud-run.sh $(PROJECT_ID) $(REGION) $(AR_REPO) $(SERVICE_NAME)

## redeploy: Rebuild and redeploy
redeploy:
	@echo "$(GREEN)Redeploying...$(NC)"
	./scripts/redeploy.sh $(PROJECT_ID) $(REGION) $(AR_REPO) $(SERVICE_NAME)

## setup: Initial project setup
setup: env-check
	@echo "$(GREEN)Setting up project...$(NC)"
	@echo "Step 1/8: Enabling APIs..."
	./scripts/enable-apis.sh $(PROJECT_ID)
	@echo "Step 2/8: Setting up service accounts..."
	./scripts/setup-service-accounts.sh $(PROJECT_ID) $(REGION) $(SERVICE_NAME)
	@echo "Step 3/8: Setting up BigQuery..."
	./scripts/setup-bigquery.sh $(PROJECT_ID) $(BQ_DATASET)
	@echo "Step 4/8: Setting up Artifact Registry..."
	./scripts/setup-artifact-registry.sh $(PROJECT_ID) $(REGION) $(AR_REPO)
	@echo "Step 5/8: Creating secrets..."
	./scripts/create-secret.sh
	@echo "Step 6/8: Building and pushing image..."
	./scripts/build-and-push.sh $(PROJECT_ID) $(REGION) $(AR_REPO) $(SERVICE_NAME)
	@echo "Step 7/8: Deploying Cloud Run service..."
	./scripts/deploy-cloud-run.sh $(PROJECT_ID) $(REGION) $(AR_REPO) $(SERVICE_NAME)
	@echo "Step 8/8: Creating Cloud Scheduler job..."
	./scripts/create-scheduler.sh $(PROJECT_ID) $(REGION) $(SERVICE_NAME)
	@echo "$(GREEN)Setup complete!$(NC)"

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "$(GREEN)All checks passed!$(NC)"

## ci: Run CI pipeline
ci: mod-download check coverage-summary
	@echo "$(GREEN)CI pipeline complete!$(NC)"

# Development helpers
## watch: Watch for changes and rebuild
watch:
	@echo "$(GREEN)Watching for changes...$(NC)"
	@if command -v air &> /dev/null; then \
		air; \
	else \
		echo "$(YELLOW)air not installed. Install with:$(NC)"; \
		echo "  go install github.com/air-verse/air@latest"; \
	fi

## install-tools: Install development tools
install-tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	go install github.com/air-verse/air@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin

## setup-iam: Setup service accounts and IAM permissions
setup-iam:
	@echo "$(GREEN)Setting up service accounts and IAM permissions...$(NC)"
	./scripts/setup-service-accounts.sh $(PROJECT_ID) $(REGION) $(SERVICE_NAME)
	@echo "$(GREEN)IAM setup complete!$(NC)"

## setup-bigquery: Setup BigQuery dataset and tables
setup-bigquery:
	@echo "$(GREEN)Setting up BigQuery dataset and tables...$(NC)"
	./scripts/setup-bigquery.sh $(PROJECT_ID) $(BQ_DATASET)
	@echo "$(GREEN)BigQuery setup complete!$(NC)"

## diag: Display diagnostic information
diag:
	@echo "$(GREEN)=== Diagnostic Information ===$(NC)"
	@echo "Go version:" && go version || echo "Go not installed"
	@echo "gcloud:" && gcloud --version | head -n1 || echo "gcloud not installed"
	@echo "Docker:" && docker --version || echo "Docker not installed"
	@echo "Project: $(PROJECT_ID)"
	@echo "Region: $(REGION)"
	@echo "Service: $(SERVICE_NAME)"
	@echo "AR Repo: $(AR_REPO)"
	@echo "BQ Dataset: $(BQ_DATASET)"

## run-once: Run the fetcher once with debug mode
run-once: mod-download
	@echo "$(GREEN)Running fetcher once in debug mode...$(NC)"
	go run $(MAIN_PATH)/main.go --once --debug

## bq-test: Test BigQuery connection
bq-test:
	@echo "$(GREEN)Testing BigQuery connection...$(NC)"
	bq query --use_legacy_sql=false --project_id=$(PROJECT_ID) "SELECT 1 as test"
	@echo "$(GREEN)BigQuery connection successful!$(NC)"

## env-check: Verify required environment variables
env-check:
	@echo "$(GREEN)Checking environment variables...$(NC)"
	@test -n "$(PROJECT_ID)" || (echo "$(RED)ERROR: PROJECT_ID not set$(NC)" && false)
	@test -n "$(REGION)" || (echo "$(RED)ERROR: REGION not set$(NC)" && false)
	@test -n "$(AR_REPO)" || (echo "$(RED)ERROR: AR_REPO not set$(NC)" && false)
	@test -n "$(SERVICE_NAME)" || (echo "$(RED)ERROR: SERVICE_NAME not set$(NC)" && false)
	@echo "$(GREEN)All required environment variables are set!$(NC)"