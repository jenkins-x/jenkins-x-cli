package factory

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud"
	amazonStorage "github.com/jenkins-x/jx/pkg/cloud/amazon/storage"
	"github.com/jenkins-x/jx/pkg/cloud/buckets"
	"github.com/jenkins-x/jx/pkg/cloud/gke/storage"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// NewBucketProvider creates a new bucket provider for a given Kubernetes provider
func NewBucketProvider(requirements *config.RequirementsConfig) buckets.Provider {
	switch requirements.Cluster.Provider {
	case cloud.GKE:
		return storage.NewGKEBucketProvider(requirements)
	case cloud.EKS:
		fallthrough
	case cloud.AWS:
		return amazonStorage.NewAmazonBucketProvider(requirements)
	default:
		// we have an implementation for GKE / EKS but not for AKS so we should fall back to default
		// but we don't have every func implemented
		return buckets.NewLegacyBucketProvider()
	}
}

// NewBucketProviderFromTeamSettingsConfiguration returns a bucket provider based on the jx-requirements file embedded in TeamSettings
func NewBucketProviderFromTeamSettingsConfiguration(factory clients.Factory) (buckets.Provider, error) {
	jxClient, ns, err := factory.CreateJXClient()
	if err != nil {
		return nil, err
	}
	teamSettings, err := kube.GetDevEnvTeamSettings(jxClient, ns)
	if err != nil {
		return nil, errors.Wrap(err, "error obtaining the dev environment teamSettings to select the correct bucket provider")
	}
	requirements, err := config.GetRequirementsConfigFromTeamSettings(teamSettings)
	if err != nil || requirements == nil {
		return nil, util.CombineErrors(err, errors.New("error obtaining the requirements file to decide bucket provider"))
	}
	return NewBucketProvider(requirements), nil
}

// NewBucketProviderFromTeamSettingsConfigurationOrDefault returns a bucket provider based on the jx-requirements file embedded in TeamSettings
// or returns the default LegacyProvider and initializes it
func NewBucketProviderFromTeamSettingsConfigurationOrDefault(factory clients.Factory, storageLocation v1.StorageLocation) (buckets.Provider, error) {
	provider, err1 := NewBucketProviderFromTeamSettingsConfiguration(factory)
	if err1 != nil {
		log.Logger().Warn("Could not obtain a valid provider, falling back to DefaultProvider")
		legacyProvider := buckets.NewLegacyBucketProvider()
		// LegacyBucketProvider is just here to keep backwards compatibility with non boot clusters, that's why we need to pass
		// some configuration in a different way, it shouldn't be the norm for providers
		err := legacyProvider.(*buckets.LegacyBucketProvider).Initialize(storageLocation.BucketURL, storageLocation.Classifier)
		if err != nil {
			return nil, util.CombineErrors(err1, errors.Wrap(err, "there was a problem initializing the legacy bucket provider"))
		}
		return legacyProvider, nil
	}
	return provider, nil
}
