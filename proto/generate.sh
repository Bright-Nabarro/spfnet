#!/usr/bin/env bash

set -e

echo "SPF Framework - Generating Protocol Buffer Code"
echo ""

# Check protoc
echo "Checking protoc..."
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc not found"
    exit 1
fi
echo "protoc version: $(protoc --version)"

# Check Go plugins
echo ""
echo "Checking protoc-gen-go plugins..."
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi
echo "Plugins ready"

# Clean old files
echo ""
echo "Cleaning old generated files..."
rm -f *.pb.go

# Generate code
echo ""
echo "Generating gRPC code..."
protoc --go_out=. \
       --go_opt=paths=source_relative \
       --go-grpc_out=. \
       --go-grpc_opt=paths=source_relative \
       node.proto

# Check result
if [ -f "node.pb.go" ] && [ -f "node_grpc.pb.go" ]; then
    echo ""
    echo "Code generation successful"
    echo "Generated files:"
    ls -lh *.pb.go
else
    echo ""
    echo "Code generation failed"
    exit 1
fi