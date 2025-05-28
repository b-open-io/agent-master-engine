# Agent Master Engine Makefile

VERSION ?= 0.1.9
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.Version=$(VERSION) \
	-X main.GitCommit=$(GIT_COMMIT) \
	-X main.BuildDate=$(BUILD_DATE)"

.PHONY: all build test clean install daemon proto

all: test build

# Generate protobuf files
proto:
	@echo "🔧 Generating proto files..."
	@mkdir -p daemon/proto
	@protoc --go_out=. --go_opt=module=github.com/b-open-io/agent-master-engine \
		--go-grpc_out=. --go-grpc_opt=module=github.com/b-open-io/agent-master-engine \
		daemon/proto/daemon.proto

# Build daemon binary
daemon: proto
	@echo "🔨 Building daemon (v$(VERSION))..."
	@go build $(LDFLAGS) -o agent-master-daemon ./cmd/daemon

# Build all binaries
build: daemon

# Install daemon to user's Go bin
install: daemon
	@echo "📦 Installing daemon to ~/go/bin..."
	@cp agent-master-daemon ~/go/bin/

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	@rm -f agent-master-daemon
	@rm -f ~/go/bin/agent-master-daemon

# Run daemon with debug logging
run-daemon: daemon
	@echo "🚀 Starting daemon..."
	@./agent-master-daemon --port 50051 --storage ~/.agent-master/daemon --log-level debug

# Check versions
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Help
help:
	@echo "Available targets:"
	@echo "  make build      - Build daemon binary"
	@echo "  make install    - Install daemon to ~/go/bin"
	@echo "  make test       - Run tests"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make run-daemon - Run daemon with debug logging"
	@echo "  make version    - Show version information"