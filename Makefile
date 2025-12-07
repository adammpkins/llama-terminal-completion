.PHONY: build install clean test

# Binary name
BINARY=lt

# Build directory
BUILD_DIR=bin

# Version (can be overridden)
VERSION?=dev
LDFLAGS=-ldflags "-X github.com/adammpkins/llamaterm/internal/cli.Version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

## build: Build the binary
build:
	@echo "Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/lt

## install: Install to GOBIN or GOPATH/bin
install: build
	@echo "Installing $(BINARY)..."
	@if [ -n "$(GOBIN)" ]; then \
		cp $(BUILD_DIR)/$(BINARY) $(GOBIN)/$(BINARY); \
	elif [ -n "$(GOPATH)" ]; then \
		mkdir -p $(GOPATH)/bin; \
		cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY); \
	else \
		mkdir -p $(HOME)/go/bin; \
		cp $(BUILD_DIR)/$(BINARY) $(HOME)/go/bin/$(BINARY); \
	fi
	@echo "Installed to $$(which lt 2>/dev/null || echo '~/go/bin/lt')"

## install-local: Install to /usr/local/bin (requires sudo)
install-local: build
	@echo "Installing to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## test: Run tests
test:
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "\nTo view HTML report: go tool cover -html=coverage.out"

## deps: Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## run: Build and run
run: build
	./$(BUILD_DIR)/$(BINARY)

## help: Show this help
help:
	@echo "LlamaTerm Makefile"
	@echo ""
	@echo "Usage:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

.DEFAULT_GOAL := build
