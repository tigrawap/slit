#!/usr/bin/env bash

set -e
gox --osarch="darwin/386 darwin/amd64 linux/386 linux/amd64 windows/386 windows/amd64 freebsd/386" github.com/tigrawap/slit
echo "Build successful"
