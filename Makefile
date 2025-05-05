SHELL := /bin/bash

APP_NAME := seqwall
DIST_DIR := dist

# Docker / CI variables
PGPORT ?= 5432
PG_VERS ?= 15
TEST_IMAGE ?= seqwall-test

.PHONY: all build ls test docker-build docker-test go-test lint format coverage clean help

## Default target
all: help

## Build the binary
build:
	@echo "==> Building $(APP_NAME)..."
	@mkdir -p $(DIST_DIR)
	go build -o $(DIST_DIR)/$(APP_NAME) .

## Show contents of the dist directory
ls:
	@echo "==> Listing $(DIST_DIR) directory..."
	@ls -l $(DIST_DIR)

## Build the Docker image used for e2e tests
docker-build:
	docker build -t $(TEST_IMAGE) -f .ci/Dockerfile .

## Run staircase tests inside the Docker container for each PostgreSQL version
docker-test: docker-build
	@for v in $(PG_VERS); do \
		echo "👉 testing on PostgreSQL $$v ..."; \
		docker run --rm -v $$PWD:/work -e PG_VERSION=$$v -e PGPORT=$(PGPORT) $(TEST_IMAGE); \
	done

## Entry‑point for staircase tests
test: docker-test

## Run unit tests
go-test:
	@echo "==> Running unit tests..."
	go test -v ./...

## Run linters (golangci-lint)
lint:
	@echo "==> Running linters..."
	golangci-lint run --timeout 5m \
		--config=.golangci.yml \
		./...

## Run formatters (golangci-lint)
format:
	@echo "==> Formatting..."
	golangci-lint run --timeout 5m \
		--config=.golangci.yml \
		--fix \
		./...

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
	@echo "  build        - Build the binary"
	@echo "  ls           - Show contents of the dist directory"
	@echo "  test         - Run staircase tests in Docker"
	@echo "  go-test      - Run unit tests"
	@echo "  lint         - Run golangci-lint"
	@echo "  format       - Run formatters (golangci-lint)"
	@echo "  coverage     - Generate coverage report (coverage.out, coverage.html)"
	@echo "  clean        - Remove build artifacts"
	@echo "  help         - Show this help"
