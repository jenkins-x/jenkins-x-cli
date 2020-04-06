#!/usr/bin/env bash
set -e

echo "verifying Pull Request"

export GH_USERNAME="jenkins-x-bot-test"
export GH_OWNER="jenkins-x-bot-test"

export REPORTS_DIR="${BASE_WORKSPACE}/build/reports"
export GINKGO_ARGS="-v"

# fix broken `BUILD_NUMBER` env var
export BUILD_NUMBER="$BUILD_ID"

JX_HOME="/tmp/jxhome"
KUBECONFIG="/tmp/jxhome/config"

mkdir -p $JX_HOME

# Disable coverage for jx version as we don't validate the output at all
COVER_JX_BINARY=false jx version
#jx step git credentials

gcloud auth activate-service-account --key-file $GKE_SA

sed -e s/\$VERSION/${VERSION_PREFIX}${VERSION}/g -e s/\$CODECOV_TOKEN/${CODECOV_TOKEN}/g myvalues.yaml.tekton.template > myvalues.yaml

#echo the myvalues.yaml file is:
#cat myvalues.yaml

# lets setup git
git config --global --add user.name JenkinsXBot
git config --global --add user.email jenkins-x@googlegroups.com

# lets avoid the git/credentials causing confusion during the test
export XDG_CONFIG_HOME=$JX_HOME

echo "running the BDD tests with JX_HOME = $JX_HOME"
jx step bdd --versions-repo https://github.com/jenkins-x/jenkins-x-versions.git \
  --config jx/bdd/tekton/cluster.yaml \
  --gopath /tmp  \
  --git-provider=github \
  --git-username $GH_USERNAME \
  --git-owner $GH_OWNER \
  --git-api-token $GH_ACCESS_TOKEN \
  --default-admin-password $JENKINS_PASSWORD \
  --no-delete-app \
  --no-delete-repo \
  --tekton \
  --tests test-upgrade-ingress \
  --tests test-create-spring \
  --tests test-import-golang-http-from-jenkins-x-yml \
  --tests test-app-lifecycle
