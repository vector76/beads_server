BINARY_NAME := bs
BUILD_DIR := build
SRC := ./cmd/bs
VERSION ?= dev
LDFLAGS := -X 'github.com/yourorg/beads_server/internal/cli.version=$(VERSION)'

.PHONY: all build test clean linux windows darwin-arm64 darwin-amd64

all: test build

build: linux windows darwin-arm64 darwin-amd64

test:
	go test ./...

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC)

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SRC)

darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(SRC)

darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(SRC)

clean:
	rm -rf $(BUILD_DIR)
