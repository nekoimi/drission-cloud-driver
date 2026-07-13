.PHONY: run run-server build test lint docker-build docker-up docker-down dev-up dev-down clean

# Run the server
run: run-server

run-server:
	go run cmd/server/main.go --config config/config.dev.yaml

# Build
build:
	go build -o bin/server cmd/server/main.go

# Test
test:
	go test ./...

# Lint (requires golangci-lint in PATH: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)
lint:
	golangci-lint run ./...

# Docker
docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Dev environment
dev-up:
	docker compose up -d

dev-down:
	docker compose down

# Clean
clean:
	rm -rf bin/
