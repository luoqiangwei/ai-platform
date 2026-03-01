#!/bin/bash

# File to store the current version
VERSION_FILE=".version"

# 1. Initialize version file if it doesn't exist
if [ ! -f $VERSION_FILE ]; then
    echo "1" > $VERSION_FILE
fi

# 2. Read the current version
CURRENT_VERSION=$(cat $VERSION_FILE)

# 3. Define the image name
IMAGE_NAME="ai-platform"
TAG="v$CURRENT_VERSION"

echo "------------------------------------------"
echo "Building Docker image: $IMAGE_NAME:$TAG"
echo "------------------------------------------"

# 4. Execute Docker build
# We use --build-arg if you want to pass the version into the Go binary (optional)
docker build --build-arg APP_VERSION=$TAG \
             -t "$IMAGE_NAME:$TAG" \
             -t "$IMAGE_NAME:latest" .

# 5. Check if build was successful
if [ $? -eq 0 ]; then
    echo "------------------------------------------"
    echo "Build Successful!"
    echo "Tagged as: $IMAGE_NAME:$TAG"
    echo "Tagged as: $IMAGE_NAME:latest"

    # Increment version for the next run
    NEXT_VERSION=$((CURRENT_VERSION + 1))
    echo $NEXT_VERSION > $VERSION_FILE
    echo "Next version will be: v$NEXT_VERSION"
else
    echo "------------------------------------------"
    echo "Build Failed. Version not incremented."
    exit 1
fi

mkdir data
