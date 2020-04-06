#!/usr/bin/env bash

# Helper script for testing/developing the pipelinerunner controller.
# In separate shell run: './build/jx controller pipelinerunner', then
# execute this script to send an example payload.

read -r -d '' JSON << EOJSON
{
  "labels": {
    "created-by-prow": "true",
    "prowJobName": "cdf89f04-98ec-11e9-a846-4ad95a1bb3ab"
  },
  "prowJobSpec": {
    "type": "pullrequest",
    "agent": "tekton",
    "cluster": "default",
    "namespace": "jx",
    "job": "serverless-jenkins",
    "refs": {
      "org": "jenkins-x-quickstarts",
      "repo": "golang-http",
      "repo_link": "https://github.com/jenkins-x-quickstarts/golang-http",
      "base_ref": "master",
      "base_sha": "068a14d5fd69e8abda9aed69d132d5020e5f36d0",
      "pulls": [
        {
          "number": 1,
          "sha": "3f00363d651280ab2a8ee67f395de1689156d762"
        }
      ]
    },
    "report": true,
    "context": "serverless-jenkins",
    "rerun_command": "/test this"
  }
}
EOJSON

curl -v http://localhost:8080 --data "$JSON" \
--header "Host: pipelinerunner" \
--header "Content-Type: application/json" \
--header "Accept-Encoding: gzip"





