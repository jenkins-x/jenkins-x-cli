package gke

import (
	"encoding/json"
	"strings"
)

// ImageTagInfo contains tag information for container images
type ImageTagInfo struct {
	Digest string   `json: "digest"`
	Tags   []string `json: "tags"`
}

// FindLatestImageTag returns the latest image tag from the JSON output of the command
// ` gcloud container images list-tags gcr.io/jenkinsxio/builder-maven --format jsonhig`
func FindLatestImageTag(output string) (string, error) {
	infos := []ImageTagInfo{}

	err := json.Unmarshal([]byte(output), &infos)
	if err != nil {
		return "", err
	}
	for _, info := range infos {
		for _, tag := range info.Tags {
			if tag != "" && !strings.Contains(tag, "SNAPSHOT") {
				return tag, nil
			}
		}
	}
	return "", nil
}
