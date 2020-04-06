package dependencymatrix

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// DependencyUpdates is the struct for the file containing dependency updates
type DependencyUpdates struct {
	Updates []v1.DependencyUpdate `json:"updates"`
}

// DependencyUpdatesAssetName is the name of the asset used for the dependency updates file when uploading to the git provider
const DependencyUpdatesAssetName = "dependency-updates.yaml"
