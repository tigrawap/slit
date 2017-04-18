#!/usr/bin/env bash

set -e
gox --osarch="darwin/386 linux/386" github.com/tigrawap/slit
echo "Build successful"
