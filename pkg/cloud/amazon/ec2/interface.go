package ec2

import "github.com/jenkins-x/jx/pkg/cluster"

// EC2er is an interface that abstracts the EC2 API
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/cloud/amazon/ec2 EC2er -o mocks/ec2erMock.go
type EC2er interface {

	// DeleteVolumeForCluster should delete every volume with Kubernetes / JX owned tags
	DeleteVolumesForCluster(cluster *cluster.Cluster) error
}
