SHELL := /bin/bash

APP_NAME := seqwall         # Name of the binary
DIST_DIR := dist            # Build destination directory

.PHONY: all build ls test lint coverage clean help

## Default target
all: build

## Build the binary
build:
	@echo "==> Building $(APP_NAME)..."
	@mkdir -p $(DIST_DIR)
	go build -o $(DIST_DIR)/$(APP_NAME) .

## Show contents of the dist directory
ls:
	@echo "==> Listing $(DIST_DIR) directory..."
	@ls -l $(DIST_DIR)

## Run tests
test:
	@echo "==> Running tests..."
	go test -v ./...

## Run linters (golangci-lint)
lint:
	@echo "==> Running linters..."
	golangci-lint run

## Generate coverage report
coverage:
	@echo "==> Generating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "==> Generating HTML coverage report (coverage.html)..."
	go tool cover -html=coverage.out -o coverage.html

## Remove build artifacts
clean:
	@echo "==> Cleaning up..."
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html

## Show help for available targets
help:
	@echo "Make targets:"
	@echo "  build     - Build the binary"
	@echo "  ls        - Show contents of the dist directory"
	@echo "  test      - Run tests"
	@echo "  lint      - Run golangci-lint (make sure it is installed)"
	@echo "  coverage  - Generate coverage report (coverage.out, coverage.html)"
	@echo "  clean     - Remove build artifacts"
	@echo "  help      - Show help for available targets"
