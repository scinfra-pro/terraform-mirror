.PHONY: build run test clean health help

# Binary name
BINARY=tf-mirror

# Help (default)
help:
	@echo "Terraform Mirror - commands:"
	@echo ""
	@echo "  make build    Build for Linux"
	@echo "  make run      Run (go run)"
	@echo "  make test     Run tests"
	@echo "  make health   Check GET /health"
	@echo "  make clean    Clean up"
	@echo ""
	@echo "Docker:"
	@echo "  docker compose up -d      Start (nginx + tf-mirror)"
	@echo "  docker compose down       Stop"
	@echo "  docker compose logs -f    Logs"
	@echo ""

# Build for Linux
build:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) ./cmd/tf-mirror

# Run (for development)
run:
	go run ./cmd/tf-mirror

# Tests
test:
	go test -v ./...

# Check health endpoint
health:
	curl -s http://localhost:8080/health | jq .

# Clean up
clean:
	rm -f $(BINARY)
	rm -rf cache/

