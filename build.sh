#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Configuration
BINARY_NAME="aws-lambda-rie"
IMAGE_NAME="${IMAGE_NAME:-anyfaas-lambda-multi}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
ARCH="${ARCH:-x86_64}"

# Map architecture to Go arch
case "$ARCH" in
    x86_64|amd64)
        GO_ARCH="amd64"
        ARCH_SUFFIX="x86_64"
        ;;
    arm64|aarch64)
        GO_ARCH="arm64"
        ARCH_SUFFIX="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "=== Building RIE binary for $ARCH_SUFFIX ==="

# Create bin directory
mkdir -p bin

# Build the binary
CGO_ENABLED=0 GOOS=linux GOARCH="$GO_ARCH" go build -buildvcs=false -ldflags "-s -w" -o "bin/${BINARY_NAME}" ./cmd/aws-lambda-rie

echo "Binary built: bin/${BINARY_NAME}"

echo "=== Building Docker image: ${IMAGE_NAME}:${IMAGE_TAG} ==="

# Build Docker image
docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" .

echo "=== Build complete ==="
echo "Image: ${IMAGE_NAME}:${IMAGE_TAG}"