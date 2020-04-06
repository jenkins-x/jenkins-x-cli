package amazon

import (
	"github.com/jenkins-x/jx/pkg/cloud/amazon/awscli"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/ec2"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/eks"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/eksctl"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"
	"github.com/pkg/errors"
)

// Provider provides an interface to access AWS services with supported actions
type Provider interface {
	EKS() eks.EKSer
	EC2() ec2.EC2er
	EKSCtl() eksctl.EKSCtl
	AWSCli() awscli.AWS
}

type clusterProvider struct {
	providerServices
	Region  string
	Profile string
}

type providerServices struct {
	cli    awscli.AWS
	eks    eks.EKSer
	ec2    ec2.EC2er
	eksctl eksctl.EKSCtl
}

// EKS returns an initialized instance of eks.EKSer
func (p providerServices) EKS() eks.EKSer {
	return p.eks
}

// EC2 returns an initialized instance of ec2.EC2er
func (p providerServices) EC2() ec2.EC2er {
	return p.ec2
}

// EKSCtl returns an abstraction of the eksctl CLI
func (p providerServices) EKSCtl() eksctl.EKSCtl {
	return p.eksctl
}

// AWSCli returns an abstraction of the AWS CLI
func (p providerServices) AWSCli() awscli.AWS {
	return p.cli
}

// NewProvider returns a Provider implementation configured with a session and implementations for AWS services
func NewProvider(region string, profile string) (*clusterProvider, error) {
	session, err := session.NewAwsSession(profile, region)
	if err != nil {
		return nil, errors.Wrap(err, "error obtaining a valid AWS session")
	}

	services := providerServices{}

	ec2Options, err := ec2.NewEC2APIHandler(session)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing the EC2 API")
	}
	services.ec2 = ec2Options

	eksOptions, err := eks.NewEKSAPIHandler(session)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing the EKS API")
	}
	services.eks = eksOptions

	services.eksctl = eksctl.NewEksctlClient()

	services.cli = awscli.NewAWSCli()

	provider := &clusterProvider{
		providerServices: services,
		Region:           region,
		Profile:          profile,
	}
	return provider, nil
}
