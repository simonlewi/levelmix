#!/bin/bash

# Exit on any error
set -e

# Create build directory if it doesn't exist
mkdir -p build

# Build the project
echo "Building levelmix..."
go build -ldflags="-w -s" -o build/levelmix ./core/cmd/server/main.go
go build -ldflags="-w -s" -o build/levelmix-worker ./core/cmd/worker/main.go

# Make the binaries executable
chmod +x build/levelmix
chmod +x build/levelmix-worker

echo "Build complete!"
echo "Server binary: build/levelmix"
echo "Worker binary: build/levelmix-worker"