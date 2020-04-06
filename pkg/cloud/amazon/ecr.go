package amazon

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"

	"github.com/jenkins-x/jx/pkg/kube"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// GetAccountIDAndRegion returns the current account ID and region
func GetAccountIDAndRegion(profile string, region string) (string, string, error) {
	sess, err := session.NewAwsSession(profile, region)
	// We nee to get the region from the connected cluster instead of the one configured for the calling user
	// as it might not be found and it would then use the default (us-west-2)
	_, region, err = session.GetCurrentlyConnectedRegionAndClusterName()
	if err != nil {
		return "", "", err
	}
	svc := sts.New(sess)

	input := &sts.GetCallerIdentityInput{}

	result, err := svc.GetCallerIdentity(input)
	if err != nil {
		return "", region, err
	}
	if result.Account != nil {
		return *result.Account, region, nil
	}
	return "", region, fmt.Errorf("Could not find the AWS Account ID!")
}

// GetContainerRegistryHost
func GetContainerRegistryHost() (string, error) {
	accountId, region, err := GetAccountIDAndRegion("", "")
	if err != nil {
		return "", err
	}
	return accountId + ".dkr.ecr." + region + ".amazonaws.com", nil
}

/*
Deprecated!

This function is kept for backwards compatibility. AWS region should not be resolved from ECR address, but
read from ConfigMap (see RememberRegion function). To keep backwards compatibility with existing installations this
function will be kept for a while and it will perform migration to config map. Eventually it will be removed from a
codebase.
*/
func GetRegionFromContainerRegistryHost(kubeClient kubernetes.Interface, namespace string, dockerRegistry string) string {
	submatch := regexp.MustCompile(`\.ecr\.(.*)\.amazonaws\.com$`).FindStringSubmatch(dockerRegistry)
	if len(submatch) > 1 {
		region := submatch[1]
		// Migrating jx installations created before AWS region config map
		kube.RememberRegion(kubeClient, namespace, region)
		return region
	} else {
		return ""
	}
}

// LazyCreateRegistry lazily creates the ECR registry if it does not already exist
func LazyCreateRegistry(kube kubernetes.Interface, namespace string, region string, dockerRegistry string, orgName string, appName string) error {
	// strip any tag/version from the app name
	idx := strings.Index(appName, ":")
	if idx > 0 {
		appName = appName[0:idx]
	}
	repoName := appName
	if orgName != "" {
		repoName = orgName + "/" + appName
	}
	repoName = strings.ToLower(repoName)
	log.Logger().Infof("Let's ensure that we have an ECR repository for the Docker image %s", util.ColorInfo(repoName))
	if region == "" {
		region = GetRegionFromContainerRegistryHost(kube, namespace, dockerRegistry)
	}
	sess, err := session.NewAwsSession("", region)
	if err != nil {
		return err
	}
	svc := ecr.New(sess)
	repoInput := &ecr.DescribeRepositoriesInput{
		RepositoryNames: []*string{
			aws.String(repoName),
		},
	}
	result, err := svc.DescribeRepositories(repoInput)
	if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != ecr.ErrCodeRepositoryNotFoundException {
		return err
	}
	for _, repo := range result.Repositories {
		name := repo.String()
		log.Logger().Infof("Found repository: %s", name)
		if name == repoName {
			return nil
		}
	}
	createRepoInput := &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repoName),
	}
	createResult, err := svc.CreateRepository(createRepoInput)
	if err != nil {
		return fmt.Errorf("Failed to create the ECR repository for %s due to: %s", repoName, err)
	}
	repo := createResult.Repository
	if repo != nil {
		u := repo.RepositoryUri
		if u != nil {
			if !strings.HasPrefix(*u, dockerRegistry) {
				log.Logger().Warnf("Created ECR repository (%s) doesn't match registry configured for team (%s)",
					util.ColorInfo(*u), util.ColorInfo(dockerRegistry))
			} else {
				log.Logger().Infof("Created ECR repository: %s", util.ColorInfo(*u))
			}
		}
	}
	return nil
}
