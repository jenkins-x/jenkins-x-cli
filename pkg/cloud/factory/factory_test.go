// +build unit

package factory

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cloud"
	amazonStorage "github.com/jenkins-x/jx/pkg/cloud/amazon/storage"
	"github.com/jenkins-x/jx/pkg/cloud/buckets"
	"github.com/jenkins-x/jx/pkg/cloud/gke/storage"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewBucketProviderFromTeamSettingsConfiguration(t *testing.T) {
	var fac clients.Factory
	fac = &fake.FakeFactory{}
	jxClient, ns, err := fac.CreateJXClient()
	assert.NoError(t, err)

	requirementsYamlFile := path.Join("test_data", "jx-requirements.yml")
	exists, err := util.FileExists(requirementsYamlFile)
	assert.NoError(t, err)
	assert.True(t, exists)

	bytes, err := ioutil.ReadFile(requirementsYamlFile)
	assert.NoError(t, err)
	requirements := &config.RequirementsConfig{}
	err = yaml.Unmarshal(bytes, requirements)
	assert.NoError(t, err)

	_, err = jxClient.JenkinsV1().Environments(ns).Create(&v1.Environment{
		ObjectMeta: v12.ObjectMeta{
			Name: "dev",
		},
		Spec: v1.EnvironmentSpec{
			TeamSettings: v1.TeamSettings{
				BootRequirements: string(bytes),
			},
		},
	})
	assert.NoError(t, err)

	testCases := []struct {
		provider     string
		providerType buckets.Provider
	}{
		{
			provider:     cloud.GKE,
			providerType: &storage.GKEBucketProvider{},
		},
		{
			provider:     cloud.AWS,
			providerType: &amazonStorage.AmazonBucketProvider{},
		},
		{
			provider:     cloud.EKS,
			providerType: &amazonStorage.AmazonBucketProvider{},
		},
		{
			provider:     "NonSupportedProvider",
			providerType: &buckets.LegacyBucketProvider{},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.provider, func(t *testing.T) {
			requirements.Cluster.Provider = tt.provider
			err = updateDevEnvironment(jxClient, ns, requirements)
			assert.NoError(t, err)
			pro, err := NewBucketProviderFromTeamSettingsConfiguration(fac)
			assert.NoError(t, err)
			assert.IsType(t, tt.providerType, pro)
		})
	}
}
func TestNewBucketProviderFromTeamSettingsConfigurationOrDefault(t *testing.T) {
	var fac clients.Factory
	fac = &fake.FakeFactory{}
	jxClient, ns, err := fac.CreateJXClient()
	assert.NoError(t, err)

	_, err = jxClient.JenkinsV1().Environments(ns).Create(&v1.Environment{
		ObjectMeta: v12.ObjectMeta{
			Name: "dev",
		},
		Spec: v1.EnvironmentSpec{
			TeamSettings: v1.TeamSettings{},
		},
	})
	assert.NoError(t, err)

	provider, err := NewBucketProviderFromTeamSettingsConfigurationOrDefault(fac, v1.StorageLocation{
		Classifier: "logs",
		BucketURL:  "file://bucketname",
	})
	assert.NoError(t, err)

	assert.IsType(t, &buckets.LegacyBucketProvider{}, provider)
}

func updateDevEnvironment(jxClient versioned.Interface, ns string, requirements *config.RequirementsConfig) error {
	bytes, err := yaml.Marshal(requirements)
	if err != nil {
		return err
	}
	_, err = jxClient.JenkinsV1().Environments(ns).PatchUpdate(&v1.Environment{
		ObjectMeta: v12.ObjectMeta{
			Name: "dev",
		},
		Spec: v1.EnvironmentSpec{
			TeamSettings: v1.TeamSettings{
				BootRequirements: string(bytes),
			},
		},
	})
	return err
}
