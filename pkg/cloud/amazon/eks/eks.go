package eks

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws/client"

	session2 "github.com/jenkins-x/jx/pkg/cloud/amazon/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/pkg/errors"
)

// eksAPIHandler contains some functions to interact with and serves as an abstraction of the EKS API
type eksAPIHandler struct {
	eks eksiface.EKSAPI
}

// NewEKSAPIHandler will return an eksAPIHandler with configured credentials
func NewEKSAPIHandler(awsSession client.ConfigProvider, eksapi ...eksiface.EKSAPI) (*eksAPIHandler, error) {
	if len(eksapi) == 1 {
		return &eksAPIHandler{
			eks: eksapi[0],
		}, nil
	}
	return &eksAPIHandler{
		eks: eks.New(awsSession),
	}, nil
}

// EksClusterExists checks if EKS cluster with given name exists in given region.
func (e *eksAPIHandler) EksClusterExists(clusterName string, profile string, region string) (bool, error) {
	region, err := session2.ResolveRegion(profile, region)
	if err != nil {
		return false, err
	}
	cmd := exec.Command("eksctl", "get", "cluster", "--region", region) //nolint:gosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	for i, line := range strings.Split(string(output), "\n") {
		if i == 0 {
			continue
		}
		if strings.HasPrefix(line, clusterName+"\t") {
			return true, nil
		}
	}

	return false, nil
}

// DescribeCluster will attempt to describe the given cluster and return a simplified cluster.Cluster struct
func (e *eksAPIHandler) DescribeCluster(clusterName string) (*cluster.Cluster, string, error) {
	output, err := e.eks.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil, "", err
	}
	cl := &cluster.Cluster{
		Name:   *output.Cluster.Name,
		Labels: aws.StringValueMap(output.Cluster.Tags),
		Status: *output.Cluster.Status,
	}

	if output.Cluster.Endpoint != nil {
		cl.Location = *output.Cluster.Endpoint
	}

	return cl, *output.Cluster.Arn, err
}

// ListClusters will list all clusters existing in configured region and describe each one to return enhanced data
func (e *eksAPIHandler) ListClusters() ([]*cluster.Cluster, error) {
	var nextToken *string = nil
	var clusters []*cluster.Cluster
	for {
		output, err := e.eks.ListClusters(&eks.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}

		for _, c := range output.Clusters {
			describeClusters, _, err := e.DescribeCluster(*c)
			if err != nil {
				return nil, err
			}
			clusters = append(clusters, describeClusters)
		}

		if output.NextToken == nil {
			return clusters, err
		}

		if output.NextToken != nil {
			nextToken = output.NextToken
		}
	}
}

func (e eksAPIHandler) GetClusterAsEKSCluster(clusterName string) (*eks.Cluster, error) {
	output, err := e.eks.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error describing EKS cluster %s", clusterName)
	}
	return output.Cluster, nil
}

// AddTagsToCluster adds tags to an EKS cluster
func (e *eksAPIHandler) AddTagsToCluster(clusterName string, tags map[string]*string) error {
	_, clusterARN, err := e.DescribeCluster(clusterName)
	if err != nil {
		return err
	}
	_, err = e.eks.TagResource(&eks.TagResourceInput{
		ResourceArn: aws.String(clusterARN),
		Tags:        tags,
	})
	if err != nil {
		return err
	}
	return nil
}

// EksClusterObsoleteStackExists detects if there is obsolete CloudFormation stack for given EKS cluster.
//
// If EKS cluster creation process is interrupted, there will be CloudFormation stack in ROLLBACK_COMPLETE state left.
// Such dead stack prevents eksctl from creating cluster with the same name. This is common activity then to remove stacks
// like this and this function performs this action.
func (e *eksAPIHandler) EksClusterObsoleteStackExists(clusterName string, profile string, region string) (bool, error) {
	session, err := session2.NewAwsSession(profile, region)
	if err != nil {
		return false, err
	}
	cloudformationService := cloudformation.New(session)
	stacks, err := cloudformationService.ListStacks(&cloudformation.ListStacksInput{
		StackStatusFilter: []*string{aws.String("ROLLBACK_COMPLETE")},
	})
	if err != nil {
		return false, err
	}
	for _, stack := range stacks.StackSummaries {
		if *stack.StackName == EksctlStackName(clusterName) {
			return true, nil
		}
	}

	return false, nil
}

// CleanUpObsoleteEksClusterStack removes dead eksctl CloudFormation stack associated with given EKS cluster name.
func (e *eksAPIHandler) CleanUpObsoleteEksClusterStack(clusterName string, profile string, region string) error {
	session, err := session2.NewAwsSession(profile, region)
	if err != nil {
		return err
	}
	cloudformationService := cloudformation.New(session)
	_, err = cloudformationService.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(EksctlStackName(clusterName)),
	})

	return err
}

// EksctlStackName generates CloudFormation stack name for given EKS cluster name. This function follows eksctl
// naming convention.
func EksctlStackName(clusterName string) string {
	return fmt.Sprintf("eksctl-%s-cluster", clusterName)
}
