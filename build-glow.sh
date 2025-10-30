#!/bin/bash

set -e

REBUILD=

if [ "$1" == "--builder" ]; then
    REBUILD=1
    shift
fi

target=$1

if [ -n "$REBUILD" ]; then
    echo "Rebuilding docker builder image..."
    docker buildx build --no-cache --platform linux/arm64 -t rpi-ws281x-builder-arm64 \
               --load --file docker/app-builder/Dockerfile .
fi

for dir in examples/glowsrv examples/glowcli; do
    app=$(basename $dir)

    docker run --platform linux/arm64 --rm \
            -v "$(pwd)":"/usr/src/$app" -w "/usr/src/$app" \
            rpi-ws281x-builder-arm64 bash \
                -c "cd $dir; go mod tidy; go build -o $app-arm64"

    file $dir/$app-arm64

    if [ -n "$target" ]; then
        scp $dir/$app-arm64 $target:
    fi
done


