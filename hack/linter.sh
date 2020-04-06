#!/bin/bash

set -e -o pipefail

if [ "$DISABLE_LINTER" == "true" ]
then
  exit 0
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if ! [ -x "$(command -v golangci-lint)" ]; then
	echo "Installing GolangCI-Lint"
	${DIR}/install_golint.sh -b $GOPATH/bin v1.20.0
fi

export GO111MODULE=on
golangci-lint run \
	--no-config \
  --disable-all \
	-E misspell \
	-E unconvert \
  -E deadcode \
  -E unconvert \
  -E gosec \
  -E gofmt \
  -E goimports \
  -E structcheck \
  -E interfacer \
  -E typecheck \
  --skip-dirs vendor \
  --deadline 15m0s \
  --verbose \
  --build-tags build

# -E errcheck \
# -E varcheck \
# -E govet \
# -E unparam \
# -E megacheck \
# -E goconst \
# -E ineffassign \
# -E golint
# -E unparam
# -E gocritic
# -E maligned
