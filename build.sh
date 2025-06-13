#!/bin/bash

# Exit on any error
set -e

# Create build directory if it doesn't exist
mkdir -p build

# Build the project
echo "Building levelmix..."
go build -o build/levelmix ./core/cmd/main.go

# Make the binary executable
chmod +x build/levelmix

echo "Build complete! Binary located at build/levelmix"