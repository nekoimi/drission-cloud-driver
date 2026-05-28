.PHONY: run build test lint docker-build docker-up docker-down clean

# Run the server
run:
	go run cmd/server/main.go --config configs/config.dev.yaml

# Build
build:
	go build -o bin/server cmd/server/main.go

# Test
test:
	go test ./...

# Lint (requires golangci-lint in PATH)
lint:
	golangci-lint run ./...

# Docker
docker-build:
	docker compose -f deployments/docker-compose.yml build

docker-up:
	docker compose -f deployments/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker-compose.yml down

# Clean
clean:
	rm -rf bin/
