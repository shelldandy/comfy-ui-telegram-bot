.PHONY: build run clean test fmt lint

BINARY_NAME=comfy-tg-bot
BUILD_DIR=bin

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
