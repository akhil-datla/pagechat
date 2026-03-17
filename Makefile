.PHONY: build test lint clean run docker-build docker-run

BINARY  := pagechat
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

## build: Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/pagechat

## run: Build and run the server
run: build
	./bin/$(BINARY)

## test: Run all tests
test:
	go test -v -race -count=1 ./...

## cover: Run tests with coverage
cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

## docker-build: Build Docker image
docker-build:
	docker build -t $(BINARY):$(VERSION) -t $(BINARY):latest .

## docker-run: Run in Docker
docker-run: docker-build
	docker run -p 8080:8080 $(BINARY):latest

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
