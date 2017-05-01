#!/usr/bin/env bash

set -e
if [ $(go fmt | tee /dev/fd/2 |  wc -l) != 0 ]; then
    echo "Refusing to build, go fmt had something to say"
    exit 1
fi
if [ $(go vet | tee /dev/fd/2 | wc -l) != 0 ]; then
    echo "Refusing to build, go vet had something to say"
    exit 1
fi
gox --osarch="darwin/386 darwin/amd64 linux/386 linux/amd64 windows/386 windows/amd64 freebsd/386" github.com/tigrawap/slit
echo "Build successful"
