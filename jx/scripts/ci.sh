#!/usr/bin/env bash
set -e

echo "verifying Pull Request"
export ORG="jenkinsxio"
export APP_NAME="jx"
export TEAM="$(echo ${BRANCH_NAME}-$BUILD_ID  | tr '[:upper:]' '[:lower:]')"
export REPORTS_DIR="${BASE_WORKSPACE}/build/reports"

# for BDD tests
export GIT_PROVIDER_URL="https://github.beescloud.com"
export GHE_TOKEN="$GH_ACCESS_TOKEN"
export GINKGO_ARGS="-v"

export JX_DISABLE_DELETE_APP="true"
export JX_DISABLE_DELETE_REPO="true"

# Disable manual promotion test for bdd context
export JX_BDD_SKIP_MANUAL_PROMOTION="true"

echo ""
git config --global credential.helper store
git config --global --add user.name JenkinsXBot
git config --global --add user.email jenkins-x@googlegroups.com

JX_HOME="/tmp/jxhome"
KUBECONFIG="/tmp/jxhome/config"

export GIT_COMMITTER_NAME="dev1"

mkdir -p $JX_HOME

# Disable coverage for jx version as we don't validate the output at all
COVER_JX_BINARY=false jx version
jx step git credentials

# lets create a team for this PR and run the BDD tests
gcloud auth activate-service-account --key-file ${GKE_SA}
gcloud container clusters get-credentials jx-bdd-tests --zone europe-west1-c --project jenkins-x-infra

sed -e s/\$VERSION/${VERSION_PREFIX}${VERSION}/g -e s/\$CODECOV_TOKEN/${CODECOV_TOKEN}/g myvalues.yaml.template > myvalues.yaml

#echo the myvalues.yaml file is:
#cat myvalues.yaml

echo "creating team: $TEAM"

git config --global --add user.name JenkinsXBot
git config --global --add user.email jenkins-x@googlegroups.com

# lets trigger the BDD tests in a new team and git provider
jx step bdd -b --provider=gke --git-provider=ghe --git-provider-url=https://github.beescloud.com --git-username dev1 --git-api-token $GH_ACCESS_TOKEN --default-admin-password $JENKINS_PASSWORD --no-delete-app --no-delete-repo --tests install --tests test-create-spring

# Reset the namespace back to jx after test for any followup steps
COVER_JX_BINARY=false jx ns jx
