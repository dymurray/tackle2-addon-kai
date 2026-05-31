GOPATH    ?= $(HOME)/go
GOBIN     ?= $(GOPATH)/bin
GOIMPORTS = $(GOBIN)/goimports
STATICCHECK = $(GOBIN)/staticcheck
IMG       ?= quay.io/djzager/tackle2-addon-kai:latest

PKG = ./cmd/...
PKGDIR = $(subst /...,,$(PKG))

.PHONY: all build test clean fmt vet lint tidy deps help
.DEFAULT_GOAL := help

help: ## Display this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: clean fmt vet lint test build ## Run all checks and build

build: fmt vet ## Build all binaries
	@echo "Building binaries..."
	mkdir -p bin
	go build -ldflags="-w -s" -o bin/addon ./cmd/addon
	go build -ldflags="-w -s" -o bin/fetch-analysis ./cmd/fetch-analysis
	@echo "Built: bin/addon bin/fetch-analysis"

test: ## Run all tests with coverage
	@echo "Running tests..."
	go test -v -race -cover ./...

test-coverage: ## Run tests with detailed coverage report
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmark tests
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

clean: ## Clean build artifacts and cache
	@echo "Cleaning..."
	rm -rf bin/ coverage.out coverage.html
	go clean -cache -testcache -modcache

fmt: $(GOIMPORTS) ## Format Go code
	@echo "Formatting code..."
	$(GOIMPORTS) -w $(PKGDIR)
	gofmt -s -w $(PKGDIR)

vet: ## Run go vet
	@echo "Running go vet..."
	go vet $(PKG)

lint: $(STATICCHECK) ## Run static analysis
	@echo "Running static analysis..."
	$(STATICCHECK) $(PKG)

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	go mod tidy
	go mod verify

deps: $(GOIMPORTS) $(STATICCHECK) ## Install development dependencies
	@echo "Installing dependencies..."

image-docker: ## Build Docker image
	docker build -t $(IMG) .

image-podman: ## Build Podman image
	podman build -t $(IMG) .

# Install development tools
$(GOIMPORTS):
	@echo "Installing goimports..."
	go install golang.org/x/tools/cmd/goimports@v0.27.0

$(STATICCHECK):
	@echo "Installing staticcheck..."
	go install honnef.co/go/tools/cmd/staticcheck@2024.1.1
