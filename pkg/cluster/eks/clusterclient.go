package eks

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/pkg/errors"

	awsAPI "github.com/aws/aws-sdk-go/aws"
	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cluster"
)

// awsClusterClient that will provide functions to interact with EKS through the AWS API or the AWS CLI
type awsClusterClient struct {
	amazon.Provider
}

// NewAWSClusterClient will return an AWSClusterClient
func NewAWSClusterClient() (cluster.Client, error) {
	provider, err := amazon.NewProvider("", "")
	if err != nil {
		return nil, errors.Wrap(err, "error obtaining a cluster provider for AWS")
	}
	return &awsClusterClient{
		provider,
	}, nil
}

// List will return a slice of every EKS cluster existing in the configured region
func (a awsClusterClient) List() ([]*cluster.Cluster, error) {
	return a.EKS().ListClusters()
}

// ListFilter lists the clusters with the matching label filters
func (a awsClusterClient) ListFilter(tags map[string]string) ([]*cluster.Cluster, error) {
	return cluster.ListFilter(a, tags)
}

// Connect connects to the given cluster - returning an error if the connection cannot be made
func (a awsClusterClient) Connect(cluster *cluster.Cluster) error {
	return a.AWSCli().ConnectToClusterWithAWSCLI(cluster.Name)
}

// String returns a text representation of the client
func (a awsClusterClient) String() string {
	return fmt.Sprintf("EKS Cluster client")
}

// SetClusterLabels adds labels to the given cluster
func (a awsClusterClient) SetClusterLabels(cluster *cluster.Cluster, clusterTags map[string]string) error {
	// AWS works with Tags, should be equal
	return a.EKS().AddTagsToCluster(cluster.Name, awsAPI.StringMap(clusterTags))
}

// Get looks up a given cluster by name returning nil if its not found
func (a awsClusterClient) Get(clusterName string) (*cluster.Cluster, error) {
	describedCluster, _, err := a.EKS().DescribeCluster(clusterName)
	return describedCluster, err
}

// Delete should delete the given cluster using eksctl and delete any created EBS volumes to avoid extra charges
func (a awsClusterClient) Delete(cluster *cluster.Cluster) error {
	log.Logger().Infof("Attempting to delete cluster %s", cluster.Name)
	err := a.EKSCtl().DeleteCluster(cluster)
	if err != nil {
		return errors.Wrapf(err, "error deleting cluster %s", cluster.Name)
	}
	// clean up the left over EBS volumes
	err = a.EC2().DeleteVolumesForCluster(cluster)
	if err != nil {
		return errors.Wrapf(err, "error deleting EC2 Volume for cluster %s", cluster.Name)
	}

	return nil
}
