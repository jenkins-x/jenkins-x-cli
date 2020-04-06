package kube

import (
	"github.com/Jeffail/gabs"
)

const (
	nodeJson = `[
      {
        "Effect": "Allow",
        "Action": ["ecr:InitiateLayerUpload", "ecr:UploadLayerPart","ecr:CompleteLayerUpload","ecr:PutImage"],
        "Resource": ["*"]
      }
    ]`
)

// EnableInsecureRegistry appends the Docker Registry
func EnableInsecureRegistry(iqJson string, dockerRegistry string) (string, error) {
	doc, err := gabs.ParseJSON([]byte(iqJson))
	if err != nil {
		return "", err
	}

	_, err = doc.Set(dockerRegistry, "spec", "docker", "insecureRegistry")
	if err != nil {
		return "", err
	}

	_, err = doc.Set(nodeJson, "spec", "additionalPolicies", "node")
	if err != nil {
		return "", err
	}
	return doc.String(), nil
}
