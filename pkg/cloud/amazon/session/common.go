package session

import (
	"os"
	"regexp"
	"runtime"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
)

const DefaultRegion = "us-west-2"

func NewAwsSession(profileOption string, regionOption string) (*session.Session, error) {
	config := aws.Config{}
	if regionOption != "" {
		config.Region = aws.String(regionOption)
	}

	sessionOptions := session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            config,
	}

	if profileOption != "" {
		sessionOptions.Profile = profileOption
	}

	awsSession, err := session.NewSessionWithOptions(sessionOptions)
	if err != nil {
		return nil, err
	}

	if *awsSession.Config.Region == "" {
		awsSession.Config.Region = aws.String(DefaultRegion)
	}

	return awsSession, nil
}

func NewAwsSessionWithoutOptions() (*session.Session, error) {
	return NewAwsSession("", "")
}

func ResolveRegion(profileOption string, regionOption string) (string, error) {
	session, err := NewAwsSession(profileOption, regionOption)
	if err != nil {
		return "", err
	}
	return *session.Config.Region, nil
}

func ResolveRegionWithoutOptions() (string, error) {
	return ResolveRegion("", "")
}

// GetClusterNameAndRegionFromAWS uses the AWS SDK to parse through each EKS cluster until it finds one that matches the endpoint in
// the kubeconfig. From there it will retrieve the cluster name
func GetClusterNameAndRegionFromAWS(profileOption string, regionOption string, kubeEndpoint string) (string, string, error) {
	session, err := NewAwsSession(profileOption, regionOption)
	if err != nil {
		return "", "", errors.Wrapf(err, "Error creating AWS Session")
	}
	svc := eks.New(session)

	input := &eks.ListClustersInput{}
	result, err := svc.ListClusters(input)
	if err != nil {
		return "", "", errors.Wrapf(err, "Error calling Eks List Clusters")
	}

	for _, cluster := range result.Clusters {
		input := &eks.DescribeClusterInput{
			Name: aws.String(*cluster),
		}
		result, err := svc.DescribeCluster(input)
		if err != nil {
			return "", "", errors.Wrapf(err, "Error calling Describe Cluster on "+*cluster)
		}

		if *result.Cluster.Endpoint == kubeEndpoint {
			return *result.Cluster.Name, *session.Config.Region, nil
		}
	}

	return "", "", errors.Errorf("Unable to get cluster name from AWS")
}

// ParseContext parses the EKS cluster context to extract the cluster name and the region
func ParseContext(context string) (string, string, error) {
	// First check if context name matches <cluster-name>.<region>.*
	reg := regexp.MustCompile(`([a-zA-Z][-a-zA-Z0-9]*)\.((us(-gov)?|ap|ca|cn|eu|sa)-(central|(north|south)?(east|west)?)-\d)\.*`)
	result := reg.FindStringSubmatch(context)
	if len(result) >= 3 {
		return result[1], result[2], nil
	}

	// or else if the context name matchesAWS ARN format as defined:
	// https://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html
	reg = regexp.MustCompile(`arn:aws:eks:((?:us(?:-gov)?|ap|ca|cn|eu|sa)-(?:central|(?:north|south)?(?:east|west)?)-\d):[0-9]*:cluster\/([a-zA-Z][-a-zA-Z0-9]*)`)
	result = reg.FindStringSubmatch(context)
	if len(result) >= 3 {
		return result[2], result[1], nil
	}

	return "", "", errors.Errorf("unable to parse %s as <cluster_name>.<region>.* or arn:aws:<region>:account-id:cluster/<cluster_name>", context)

}

// GetCurrentlyConnectedRegionAndClusterName gets the current context for the connected cluster and parses it
// to extract both the Region and the ClusterName
func GetCurrentlyConnectedRegionAndClusterName() (string, string, error) {
	kubeConfig, _, err := kube.NewKubeConfig().LoadConfig()
	if err != nil {
		return "", "", errors.Wrapf(err, "loading kubeconfig")
	}

	context := kube.Cluster(kubeConfig)
	server := kube.CurrentServer(kubeConfig)
	currentClusterName, currentRegion, err := GetClusterNameAndRegionFromAWS("", "", server)
	if err != nil {
		currentClusterName, currentRegion, err := ParseContext(context)
		if err != nil {
			return "", "", errors.Wrapf(err, "parsing the current Kubernetes context %s", context)
		}
		return currentClusterName, currentRegion, nil
	}
	return currentClusterName, currentRegion, nil
}

// UserHomeDir returns the home directory for the user the process is running under.
// This is a copy of shareddefaults.UserHomeDir in the internal AWS package.
// We can't user user.Current().HomeDir as we want to override this during testing. :-|
func UserHomeDir() string {
	if runtime.GOOS == "windows" { // Windows
		return os.Getenv("USERPROFILE")
	}

	// *nix
	return os.Getenv("HOME")
}
