#!/bin/bash

set -e -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if ! [ -x "$(command -v golangci-lint)" ]; then
	echo "Installing GolangCI-Lint"
	${DIR}/install_golint.sh -b $GOPATH/bin v1.15.0
fi

golangci-lint run \
	--no-config \
    --disable-all \
	-E misspell \
	-E unconvert \
    -E deadcode \
    -E unconvert \
    -E errcheck \
    -E unused \
    --skip-dirs vendor \
    --deadline 5m0s

#    -E goimports \
#    -E goconst \
#    -D errcheck \
#    -D ineffassign \
#    -D deadcode \
#    -D govet \
#    -D varcheck \
#    -D structcheck \
#    -D typecheck \
#    -E goimports
#    -E golint
#    -E gosec
#    -E unparam
#    -E gocritic
#    -E interfacer
#    -E maligned
