#!/bin/bash

# Default values
IMAGE_NAME="mattstrayer/shove"
PLATFORMS="linux/amd64,linux/arm64"
BUILD_TYPE="multi"
TAG="latest"
PUSH="false"

# Help message
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo "Build Docker image for Shove"
    echo ""
    echo "Options:"
    echo "  -t, --tag TAG        Specify image tag (default: latest)"
    echo "  -s, --single         Build only for current platform"
    echo "  -m, --multi          Build for multiple platforms (default)"
    echo "  -p, --platforms      Specify platforms (default: linux/amd64,linux/arm64)"
    echo "  --push               Push the image to Docker Hub after building"
    echo "  -u, --username       Docker Hub username"
    echo "  -w, --password       Docker Hub password or access token"
    echo "  -h, --help           Show this help message"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -t|--tag)
            TAG="$2"
            shift 2
            ;;
        -s|--single)
            BUILD_TYPE="single"
            shift
            ;;
        -m|--multi)
            BUILD_TYPE="multi"
            shift
            ;;
        -p|--platforms)
            PLATFORMS="$2"
            shift 2
            ;;
        --push)
            PUSH="true"
            shift
            ;;
        -u|--username)
            DOCKER_USERNAME="$2"
            shift 2
            ;;
        -w|--password)
            DOCKER_PASSWORD="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Check for Docker Hub credentials if push is enabled
if [ "$PUSH" = "true" ]; then
    if [ -z "$DOCKER_USERNAME" ] || [ -z "$DOCKER_PASSWORD" ]; then
        echo "Error: Docker Hub credentials required for push"
        echo "Please provide --username and --password options"
        exit 1
    fi

    echo "Logging in to Docker Hub..."
    if ! echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin; then
        echo "Error: Failed to log in to Docker Hub"
        exit 1
    fi
fi

# Ensure Docker buildx is available
if ! docker buildx version > /dev/null 2>&1; then
    echo "Error: Docker buildx is not available"
    echo "Please ensure you have Docker buildx installed and configured"
    exit 1
fi

# Set up buildx builder if needed
if [ "$BUILD_TYPE" = "multi" ]; then
    echo "Setting up multi-platform builder..."
    docker buildx create --use --name shove-builder || true
fi

# Build command construction
BUILD_CMD="docker buildx build"
BUILD_CMD="$BUILD_CMD --file Dockerfile"
BUILD_CMD="$BUILD_CMD --build-arg BUILDKIT_PROGRESS=plain"
BUILD_CMD="$BUILD_CMD --build-arg GOOS=linux"
BUILD_CMD="$BUILD_CMD --build-arg CGO_ENABLED=0"

# Add platform-specific options
if [ "$BUILD_TYPE" = "multi" ]; then
    BUILD_CMD="$BUILD_CMD --platform $PLATFORMS"
    if [ "$PUSH" = "true" ]; then
        BUILD_CMD="$BUILD_CMD --push"
    else
        BUILD_CMD="$BUILD_CMD --load"
    fi
else
    BUILD_CMD="$BUILD_CMD --load"
fi

# Add tags
BUILD_CMD="$BUILD_CMD -t $IMAGE_NAME:$TAG"

# Add context
BUILD_CMD="$BUILD_CMD ."

# Execute build
echo "Building image: $IMAGE_NAME:$TAG"
echo "Build type: $BUILD_TYPE"
if [ "$BUILD_TYPE" = "multi" ]; then
    echo "Platforms: $PLATFORMS"
fi
if [ "$PUSH" = "true" ]; then
    echo "Will push to Docker Hub after build"
fi
echo "Starting build..."

if eval "$BUILD_CMD"; then
    echo "Build completed successfully!"
    if [ "$PUSH" = "true" ]; then
        echo "Image pushed to Docker Hub"
    fi
else
    echo "Build failed!"
    exit 1
fi

# Clean up
if [ "$BUILD_TYPE" = "multi" ]; then
    echo "Cleaning up builder..."
    docker buildx rm shove-builder
fi

# Logout from Docker Hub if we logged in
if [ "$PUSH" = "true" ]; then
    echo "Logging out from Docker Hub..."
    docker logout
fi
