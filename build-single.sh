#!/bin/bash

REBUILD=

if [ "$1" == "--builder" ]; then
    REBUILD=1
    shift
fi

dir=$1

if [ -z "$dir" ]; then
    echo "Usage: $(basename $0) [--builder] <directory-containing-examples>"
    exit 1
fi

if [ -n "$REBUILD" ]; then
    echo "Rebuilding docker builder image..."
    docker buildx build --no-cache --platform linux/arm64 -t rpi-ws281x-builder-arm64 \
               --load --file docker/app-builder/Dockerfile .
fi

app=$(basename $dir)
docker run --platform linux/arm64 --rm \
           -v "$(pwd)":"/usr/src/$app" -w "/usr/src/$app" \
           rpi-ws281x-builder-arm64 bash \
               -c "cd $dir; go mod tidy; go build -o $app-arm64"

file $dir/$app-arm64

