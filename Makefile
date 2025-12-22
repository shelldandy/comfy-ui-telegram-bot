.PHONY: build run clean test fmt lint docker-build docker-run docker-stop docker-logs

BINARY_NAME=comfy-tg-bot
BUILD_DIR=bin
DOCKER_IMAGE=comfy-tg-bot

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/bot

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)
	go clean

test:
	go test -v ./...

fmt:
	go fmt ./...

lint:
	go vet ./...

deps:
	go mod download
	go mod tidy

docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker compose up -d

docker-stop:
	docker compose down

docker-logs:
	docker compose logs -f
