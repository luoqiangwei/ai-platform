#!/bin/bash

set -e # Exit immediately if a command exits with a non-zero status.

# File to store the current version
VERSION_FILE=".version"

# 1. Initialize version file if it doesn't exist
if [ ! -f $VERSION_FILE ]; then
    echo "1" > $VERSION_FILE
fi

# 2. Read the current version
CURRENT_VERSION=$(cat $VERSION_FILE)

# 3. Define the image names
APP_IMAGE_NAME="ai-platform"
OPENCLAW_IMAGE_NAME="openclaw-cn-im"
TAG="v$CURRENT_VERSION"

echo "=========================================="
echo " Starting Build Process for Version: $TAG"
echo "=========================================="

# 4. Build OpenClaw CN-IM from the local directory
echo ">>> Building OpenClaw Gateway..."
if [ -d "OpenClaw-Docker-CN-IM" ]; then
    cd OpenClaw-Docker-CN-IM
    docker build -t "$APP_IMAGE_NAME:$TAG" \
                 -t "$OPENCLAW_IMAGE_NAME:latest" .
    cd ..
else
    echo "Error: Directory OpenClaw-Docker-CN-IM not found!"
    exit 1
fi

# 5. Execute Docker build for the Go App
echo ">>> Building Go Platform App..."
docker build --build-arg APP_VERSION=$TAG \
             -t "$APP_IMAGE_NAME:$TAG" \
             -t "$APP_IMAGE_NAME:latest" .

# 6. Check if build was successful
echo "=========================================="
echo " Build Successful! "
echo " App Tagged as: $APP_IMAGE_NAME:$TAG"
echo " Gateway Tagged as: $OPENCLAW_IMAGE_NAME:latest"
echo "=========================================="

# Increment version for the next run
NEXT_VERSION=$((CURRENT_VERSION + 1))
echo $NEXT_VERSION > $VERSION_FILE
echo "Next version will be: v$NEXT_VERSION"

mkdir data
mkdir openclaw_data
