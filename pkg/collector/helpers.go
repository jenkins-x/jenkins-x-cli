package collector

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud/factory"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/pkg/errors"
)

// NewCollector creates a new collector from the storage configuration
func NewCollector(storageLocation v1.StorageLocation, gitter gits.Gitter) (Collector, error) {
	classifier := storageLocation.Classifier
	if classifier == "" {
		classifier = "default"
	}
	gitURL := storageLocation.GitURL
	if gitURL != "" {
		return NewGitCollector(gitter, gitURL, storageLocation.GetGitBranch())
	}
	bucketProvider, err := factory.NewBucketProviderFromTeamSettingsConfigurationOrDefault(clients.NewFactory(), storageLocation)
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem obtaining the bucket provider from cluster configuratio")
	}
	return NewBucketCollector(storageLocation.BucketURL, classifier, bucketProvider)
}
