package testutils

import (
	"github.com/jenkins-x/jx/pkg/cloud/amazon/awscli"
	awsclitest "github.com/jenkins-x/jx/pkg/cloud/amazon/awscli/mocks"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/ec2"
	ec2test "github.com/jenkins-x/jx/pkg/cloud/amazon/ec2/mocks"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/eks"
	ekstest "github.com/jenkins-x/jx/pkg/cloud/amazon/eks/mocks"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/eksctl"
	eksctltest "github.com/jenkins-x/jx/pkg/cloud/amazon/eksctl/mocks"
)

type MockClusterProvider struct {
	mockProviderServices
	Region  string
	Profile string
}

type mockProviderServices struct {
	ec2    *ec2test.MockEC2er
	eks    *ekstest.MockEKSer
	eksctl *eksctltest.MockEKSCtl
	cli    *awsclitest.MockAWS
}

// EC2 returns an initialized instance of EC2Options
func (p mockProviderServices) EC2() ec2.EC2er {
	return p.ec2
}

// EKSCtl returns an abstraction of the eksctl CLI
func (p mockProviderServices) EKSCtl() eksctl.EKSCtl {
	return p.eksctl
}

// EKS returns an initialized instance of EKSClusterOptions
func (p mockProviderServices) EKS() eks.EKSer {
	return p.eks
}

// AWSCli returns an abstraction of the AWS CLI
func (p mockProviderServices) AWSCli() awscli.AWS {
	return p.cli
}

// NewMockProvider returns a mocked representation of cluster.Provider
func NewMockProvider(region string, profile string) *MockClusterProvider {
	services := mockProviderServices{}

	ec2Mock := ec2test.NewMockEC2er()
	services.ec2 = ec2Mock

	eksctlCLIMock := eksctltest.NewMockEKSCtl()
	services.eksctl = eksctlCLIMock

	eksMock := ekstest.NewMockEKSer()
	services.eks = eksMock

	awscliMock := awsclitest.NewMockAWS()
	services.cli = awscliMock

	provider := &MockClusterProvider{
		mockProviderServices: services,
		Region:               region,
		Profile:              profile,
	}
	return provider
}
