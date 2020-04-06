package eksctl

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cluster"
)

// eksctlClient is an abstraction of the eksctl CLI operations
type eksctlClient struct{}

// NewEksctlClient returns an abstraction of an eksctl client
func NewEksctlClient() eksctlClient {
	return eksctlClient{}
}

// DeleteCluster performs an eksctl cluster deletion process
func (eksctlClient) DeleteCluster(cluster *cluster.Cluster) error {
	cmd := exec.Command("eksctl", "delete", "cluster", "--name", cluster.Name) //nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "error deleting cluster executing eksctl delete cluster")
	}
	return nil
}
