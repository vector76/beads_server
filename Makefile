BINARY_NAME := bs
BUILD_DIR := build
SRC := ./cmd/bs

.PHONY: all build test clean linux windows

all: test build

build: linux windows

test:
	go test ./...

linux:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC)

windows:
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SRC)

clean:
	rm -rf $(BUILD_DIR)
