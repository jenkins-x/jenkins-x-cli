package factory

import (
	"fmt"
	"os"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/jenkins-x/jx/pkg/cluster/eks"
	"github.com/jenkins-x/jx/pkg/cluster/gke"
)

// NewClientFromEnv uses environment variables to detect which kind of cluster we are running inside
func NewClientFromEnv() (cluster.Client, error) {
	if os.Getenv(cluster.EnvGKEProject) != "" && os.Getenv(cluster.EnvGKERegion) != "" {
		return gke.NewGKEFromEnv()
	}
	// lets try discover the current project
	return nil, fmt.Errorf("could not detect the cluter.Client from the environment variables")
}

// NewClientForProvider will return a provider specific cluster client or an error if the provider has no client
func NewClientForProvider(provider string) (cluster.Client, error) {
	switch provider {
	case cloud.GKE:
		return gke.NewGKEFromEnv()
	case cloud.AWS:
		fallthrough
	case cloud.EKS:
		return eks.NewAWSClusterClient()
	default:
		return nil, fmt.Errorf("no cluster client found for provier %s", provider)
	}
}
