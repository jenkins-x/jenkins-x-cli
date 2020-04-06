#!/usr/bin/env sh

dir=$(dirname "$0")
dlv --log-dest 2 --listen=:2345 --headless=true --api-version=2 exec "${dir}/../build/jx" -- "$@" > /dev/null 2>&1
