BIN_DIR := dist
VERSION ?= dev

LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: all build-server build-worker build-client clean

all: build-server build-worker build-client

build-server:
	mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-server ./cmd/pkgtug-server

build-worker:
	mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-worker ./cmd/pkgtug-worker

build-client:
	mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug ./cmd/pkgtug

clean:
	rm -rf $(BIN_DIR)

# Cross-compile targets. Use distrobox or set GOOS/GOARCH directly.
build-linux-amd64:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-server-linux-amd64 ./cmd/pkgtug-server
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-worker-linux-amd64 ./cmd/pkgtug-worker
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-linux-amd64 ./cmd/pkgtug

build-linux-arm64:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-server-linux-arm64 ./cmd/pkgtug-server
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-worker-linux-arm64 ./cmd/pkgtug-worker
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/pkgtug-linux-arm64 ./cmd/pkgtug
