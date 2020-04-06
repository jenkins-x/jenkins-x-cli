#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
RESET='\033[0m'

if ! [ -x "$(command -v goimports)" ]; then
	echo "Installing goimports"
    go get golang.org/x/tools/cmd/goimports
fi

echo "Running validation scripts..."

scripts=(
    "make verify-generation-complete"
    "make generate-docs"
)
fail=0
for s in "${scripts[@]}"; do
    echo "RUN ${s}"
    set +e
    $s
    result=$?
    set -e
    if [[ $result  -eq 0 ]]; then
        echo -e "${GREEN}PASSED${RESET} ${s}"
    else
        echo -e "${RED}FAILED${RESET} ${s}"
        fail=1
    fi
done
exit $fail
