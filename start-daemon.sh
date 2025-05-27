#!/bin/bash

# Quick start script for daemon development

echo "🚀 Starting Agent Master Daemon Development..."

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "❌ protoc not found. Please install Protocol Buffers compiler:"
    echo "   brew install protobuf"
    exit 1
fi

# Check if Go protoc plugins are installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "📦 Installing Go protoc plugins..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Generate proto files
echo "🔧 Generating proto files..."
mkdir -p daemon/proto
protoc --go_out=. --go_opt=module=github.com/b-open-io/agent-master-engine \
       --go-grpc_out=. --go-grpc_opt=module=github.com/b-open-io/agent-master-engine \
       daemon/proto/daemon.proto

# Build the daemon
echo "🔨 Building daemon..."
go build -o agent-master-daemon ./cmd/daemon

# Create storage directory
mkdir -p ~/.agent-master/daemon

# Start the daemon
echo "✅ Starting daemon on port 50051..."
echo "   Storage: ~/.agent-master/daemon"
echo "   Lock file: /tmp/agent-master-daemon.lock"
echo ""
echo "Press Ctrl+C to stop"
echo ""

./agent-master-daemon \
    --port 50051 \
    --storage ~/.agent-master/daemon \
    --log-level debug