#!/bin/bash

mkdir -p builds

VERSION=$(git describe --tags 2>/dev/null || echo "v1.0.0")
BUILD_TIME=$(date +%FT%T%z)

echo "Building GPUD ${VERSION}"
echo "Build Time: ${BUILD_TIME}"

# Build for all platforms
GOOS=darwin GOARCH=amd64 go build -o builds/gpud-macos-intel
echo "✓ Built for macOS (Intel)"

GOOS=darwin GOARCH=arm64 go build -o builds/gpud-macos-arm
echo "✓ Built for macOS (ARM)"

GOOS=linux GOARCH=amd64 go build -o builds/gpud-linux-amd64
echo "✓ Built for Linux (AMD64)"

GOOS=linux GOARCH=arm64 go build -o builds/gpud-linux-arm64
echo "✓ Built for Linux (ARM64)"

GOOS=windows GOARCH=amd64 go build -o builds/gpud-windows-amd64.exe
echo "✓ Built for Windows (AMD64)"

GOOS=windows GOARCH=arm64 go build -o builds/gpud-windows-arm64.exe
echo "✓ Built for Windows (ARM64)"

echo "Done! Builds are available in the builds directory"
ls -lh builds/
