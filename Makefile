.PHONY: build run test clean fmt lint docker-build

BINARY=ai-gateway
OUTPUT=bin/$(BINARY)

build:
	@echo "Building $(BINARY)..."
	go build -o $(OUTPUT) ./cmd/gateway

run:
	@echo "Running $(BINARY)..."
	go run ./cmd/gateway -config configs/gateway.yaml

test:
	@echo "Running tests..."
	go test -v -race -count=1 ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	go fmt ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out coverage.html

docker-build:
	docker build -t $(BINARY):latest .

docker-run:
	docker run -p 8080:8080 -v $(PWD)/configs:/app/configs -e OPENAI_API_KEY=$(OPENAI_API_KEY) $(BINARY):latest

dev:
	@echo "Starting in development mode with hot reload..."
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	air

.DEFAULT_GOAL := build