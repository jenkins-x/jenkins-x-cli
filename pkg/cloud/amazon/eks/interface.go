package eks

import (
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/jenkins-x/jx/pkg/cluster"
)

// EKSer is an interface that abstracts the use of the EKS API
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/cloud/amazon/eks EKSer -o mocks/ekserMock.go
type EKSer interface {
	// EksClusterExists returns whether a cluster exists
	EksClusterExists(clusterName string, profile string, region string) (bool, error)

	// DescribeCluster describes a cluster and returns an internal cluster.Cluster representation
	DescribeCluster(clusterName string) (*cluster.Cluster, string, error)

	// ListClusters returns a list of clusters in the AWS account
	ListClusters() ([]*cluster.Cluster, error)

	// GetClusterAsEKSCluster returns a cluster as eks.Cluster with extra information
	GetClusterAsEKSCluster(clusterName string) (*eks.Cluster, error)

	// AddTagsToCluster adds the given tags to a cluster
	AddTagsToCluster(clusterName string, tags map[string]*string) error

	// EksClusterObsoleteStackExists detects if there is obsolete CloudFormation stack for given EKS cluster.
	//
	// If EKS cluster creation process is interrupted, there will be CloudFormation stack in ROLLBACK_COMPLETE state left.
	// Such dead stack prevents eksctl from creating cluster with the same name. This is common activity then to remove stacks
	// like this and this function performs this action.
	EksClusterObsoleteStackExists(clusterName string, profile string, region string) (bool, error)

	// CleanUpObsoleteEksClusterStack removes dead eksctl CloudFormation stack associated with given EKS cluster name.
	CleanUpObsoleteEksClusterStack(clusterName string, profile string, region string) error
}
