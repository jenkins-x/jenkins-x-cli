package awscli

// AWS is an interface to abstract the use of the AWS CLI
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/cloud/amazon/awscli AWS -o mocks/awsclimock.go
type AWS interface {
	ConnectToClusterWithAWSCLI(clusterName string) error
}
