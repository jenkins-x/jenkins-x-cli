package eksctl

import "github.com/jenkins-x/jx/pkg/cluster"

// EKSCtl is the interface that abstract the use of the eksctl cli
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/cloud/amazon/eksctl EKSCtl -o mocks/eksctl.go
type EKSCtl interface {
	// DeleteCluster performs an eksctl cluster deletion process
	DeleteCluster(cluster *cluster.Cluster) error
}
