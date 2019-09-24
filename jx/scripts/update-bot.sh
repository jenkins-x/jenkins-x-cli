#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

./build/linux/jx step create pr docker --name JX_VERSION --version $VERSION --repo https://github.com/jenkins-x/jenkins-x-builders.git --repo https://github.com/jenkins-x/jenkins-x-serverless.git --repo https://github.com/jenkins-x/jenkins-x-builders-ml.git
./build/linux/jx step create pr chart --name jx --version $VERSION  --repo https://github.com/jenkins-x/jenkins-x-platform.git
./build/linux/jx step create pr regex --regex "\s*jxTag:\s*(.*)" --version $VERSION --files prow/values.yaml --repo https://github.com/jenkins-x-charts/prow.git
./build/linux/jx step create pr go --name github.com/jenkins-x/jx --version $VERSION --build "make mod" --repo https://github.com/jenkins-x/lighthouse.git
./build/linux/jx step create pr go --name github.com/jenkins-x/jx --version $VERSION --build "make build" --repo https://github.com/cloudbees/jxui-backend.git
