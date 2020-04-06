package awscli

import (
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// awsClient is an abstraction over the AWS CLI operations
type awsClient struct{}

// NewAWSCli returns an AWS CLI abstraction client
func NewAWSCli() awsClient {
	return awsClient{}
}

// ConnectToClusterWithAWSCLI will modify the kube-config file to add the provided cluster and change context to it
func (awsClient) ConnectToClusterWithAWSCLI(clusterName string) error {
	args := []string{"eks", "update-kubeconfig", "--name", clusterName}

	cmd := util.Command{
		Name: "aws",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to connect to region cluster %s", clusterName)
	}
	return nil
}
