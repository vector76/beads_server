BINARY_NAME := bs
BUILD_DIR := build
SRC := ./cmd/bs
VERSION ?= $(patsubst v%,%,$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev"))
LDFLAGS := -X 'github.com/vector76/beads_server/internal/cli.version=$(VERSION)'

.PHONY: all build test clean install linux windows darwin-arm64 darwin-amd64

all: test build

build: linux windows darwin-arm64 darwin-amd64

test:
	go test ./...

linux:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC)

windows:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SRC)

darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(SRC)

darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(SRC)

install: linux
	install -m 0755 $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(shell go env GOPATH)/bin/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)
