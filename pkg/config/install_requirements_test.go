// +build unit

package config_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jenkins-x/jx/pkg/cloud/gke"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

var (
	testDataDir = path.Join("test_data")
)

func TestRequirementsConfigMarshalExistingFile(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-requirements-config-")
	assert.NoError(t, err, "should create a temporary config dir")

	expectedClusterName := "my-cluster"
	expectedSecretStorage := config.SecretStorageTypeVault
	expectedDomain := "cheese.co.uk"

	file := filepath.Join(dir, config.RequirementsConfigFileName)
	requirements := config.NewRequirementsConfig()
	requirements.SecretStorage = expectedSecretStorage
	requirements.Cluster.ClusterName = expectedClusterName
	requirements.Ingress.Domain = expectedDomain
	requirements.Kaniko = true

	err = requirements.SaveConfig(file)
	assert.NoError(t, err, "failed to save file %s", file)

	requirements, fileName, err := config.LoadRequirementsConfig(dir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", dir)
	assert.FileExists(t, fileName)

	assert.Equal(t, true, requirements.Kaniko, "requirements.Kaniko")
	assert.Equal(t, expectedClusterName, requirements.Cluster.ClusterName, "requirements.ClusterName")
	assert.Equal(t, expectedSecretStorage, requirements.SecretStorage, "requirements.SecretStorage")
	assert.Equal(t, expectedDomain, requirements.Ingress.Domain, "requirements.Domain")

	// lets check we can load the file from a sub dir
	subDir := filepath.Join(dir, "subdir")
	requirements, fileName, err = config.LoadRequirementsConfig(subDir)
	assert.NoError(t, err, "failed to load requirements file in subDir: %s", subDir)
	assert.FileExists(t, fileName)

	assert.Equal(t, true, requirements.Kaniko, "requirements.Kaniko")
	assert.Equal(t, expectedClusterName, requirements.Cluster.ClusterName, "requirements.ClusterName")
	assert.Equal(t, expectedSecretStorage, requirements.SecretStorage, "requirements.SecretStorage")
	assert.Equal(t, expectedDomain, requirements.Ingress.Domain, "requirements.Domain")
}

func TestOverrideRequirementsFromEnvironment(t *testing.T) {
	t.Parallel()

	requirements, fileName, err := config.LoadRequirementsConfig(testDataDir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", testDataDir)
	assert.FileExists(t, fileName)

	err = os.Setenv("JX_REQUIREMENT_VELERO_SCHEDULE", "*/5 * * * *")
	assert.NoError(t, err, "could not Setenv JX_REQUIREMENT_VELERO_SCHEDULE")

	requirements.OverrideRequirementsFromEnvironment(func() gke.GClouder {
		return nil
	})

	tempDir, err := ioutil.TempDir("", "test-requirements-config")
	assert.NoError(t, err, "should create a temporary config dir")

	err = requirements.SaveConfig(filepath.Join(tempDir, config.RequirementsConfigFileName))
	assert.NoError(t, err, "failed to save requirements file in dir %s", tempDir)
	overrideRequirements, fileName, err := config.LoadRequirementsConfig(tempDir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", testDataDir)
	assert.FileExists(t, fileName)

	assert.Equal(t, "*/5 * * * *", overrideRequirements.Velero.Schedule)

}

func TestRequirementsConfigMarshalExistingFileKanikoFalse(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-requirements-config-")
	assert.NoError(t, err, "should create a temporary config dir")

	file := filepath.Join(dir, config.RequirementsConfigFileName)
	requirements := config.NewRequirementsConfig()
	requirements.Kaniko = false

	err = requirements.SaveConfig(file)
	assert.NoError(t, err, "failed to save file %s", file)
	t.Logf("saved file %s", file)

	requirements, fileName, err := config.LoadRequirementsConfig(dir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", dir)
	assert.FileExists(t, fileName)

	assert.Equal(t, false, requirements.Kaniko, "requirements.Kaniko")

}

func TestRequirementsConfigMarshalInEmptyDir(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-requirements-config-empty-")

	requirements, fileName, err := config.LoadRequirementsConfig(dir)
	assert.Error(t, err)
	assert.Empty(t, fileName)
	assert.Nil(t, requirements)
}

func TestRequirementsConfigIngressAutoDNS(t *testing.T) {
	t.Parallel()

	requirements := config.NewRequirementsConfig()

	requirements.Ingress.Domain = "1.2.3.4.nip.io"
	assert.Equal(t, true, requirements.Ingress.IsAutoDNSDomain(), "requirements.Ingress.IsAutoDNSDomain() for domain %s", requirements.Ingress.Domain)

	requirements.Ingress.Domain = "foo.bar"
	assert.Equal(t, false, requirements.Ingress.IsAutoDNSDomain(), "requirements.Ingress.IsAutoDNSDomain() for domain %s", requirements.Ingress.Domain)

	requirements.Ingress.Domain = ""
	assert.Equal(t, false, requirements.Ingress.IsAutoDNSDomain(), "requirements.Ingress.IsAutoDNSDomain() for domain %s", requirements.Ingress.Domain)
}

func Test_env_repository_visibility(t *testing.T) {
	t.Parallel()

	var gitPublicTests = []struct {
		yamlFile          string
		expectedGitPublic bool
	}{
		{"git_public_nil_git_private_true.yaml", false},
		{"git_public_nil_git_private_false.yaml", true},
		{"git_public_false_git_private_nil.yaml", false},
		{"git_public_true_git_private_nil.yaml", true},
	}

	for _, testCase := range gitPublicTests {
		t.Run(testCase.yamlFile, func(t *testing.T) {
			content, err := ioutil.ReadFile(path.Join(testDataDir, testCase.yamlFile))
			assert.NoError(t, err)

			config := config.NewRequirementsConfig()

			_ = log.CaptureOutput(func() {
				err = yaml.Unmarshal(content, config)
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedGitPublic, config.Cluster.EnvironmentGitPublic, "unexpected value for repository visibility")
			})
		})
	}
}

func TestMergeSave(t *testing.T) {
	t.Parallel()
	type TestSpec struct {
		Name           string
		Original       *config.RequirementsConfig
		Changed        *config.RequirementsConfig
		ValidationFunc func(orig *config.RequirementsConfig, ch *config.RequirementsConfig)
	}

	testCases := []TestSpec{
		{
			Name: "Merge Cluster Config Test",
			Original: &config.RequirementsConfig{
				Cluster: config.ClusterConfig{
					EnvironmentGitOwner:  "owner",
					EnvironmentGitPublic: false,
					GitPublic:            false,
					Provider:             cloud.GKE,
					Namespace:            "jx",
					ProjectID:            "project-id",
					ClusterName:          "cluster-name",
					Region:               "region",
					GitKind:              gits.KindGitHub,
					GitServer:            gits.KindGitHub,
				},
			},
			Changed: &config.RequirementsConfig{
				Cluster: config.ClusterConfig{
					EnvironmentGitPublic: true,
					GitPublic:            true,
					Provider:             cloud.GKE,
				},
			},
			ValidationFunc: func(orig *config.RequirementsConfig, ch *config.RequirementsConfig) {
				assert.True(t, orig.Cluster.EnvironmentGitPublic == ch.Cluster.EnvironmentGitPublic &&
					orig.Cluster.GitPublic == ch.Cluster.GitPublic &&
					orig.Cluster.ProjectID != ch.Cluster.ProjectID, "ClusterConfig validation failed")
			},
		},
		{
			Name: "Merge EnvironmentConfig slices Test",
			Original: &config.RequirementsConfig{
				Environments: []config.EnvironmentConfig{
					{
						Key:        "dev",
						Repository: "repo",
					},
					{
						Key: "production",
						Ingress: config.IngressConfig{
							Domain: "domain",
						},
					},
					{
						Key: "staging",
						Ingress: config.IngressConfig{
							Domain: "domainStaging",
							TLS: config.TLSConfig{
								Email: "email",
							},
						},
					},
				},
			},
			Changed: &config.RequirementsConfig{
				Environments: []config.EnvironmentConfig{
					{
						Key:   "dev",
						Owner: "owner",
					},
					{
						Key: "production",
						Ingress: config.IngressConfig{
							CloudDNSSecretName: "secret",
						},
					},
					{
						Key: "staging",
						Ingress: config.IngressConfig{
							Domain:          "newDomain",
							DomainIssuerURL: "issuer",
							TLS: config.TLSConfig{
								Enabled: true,
							},
						},
					},
					{
						Key: "ns2",
					},
				},
			},
			ValidationFunc: func(orig *config.RequirementsConfig, ch *config.RequirementsConfig) {
				assert.True(t, len(orig.Environments) == len(ch.Environments), "the environment slices should be of the same len")
				// -- Dev Environment's asserts
				devOrig, devCh := orig.Environments[0], ch.Environments[0]
				assert.True(t, devOrig.Owner == devCh.Owner && devOrig.Repository != devCh.Repository,
					"the dev environment should've been merged correctly")
				// -- Production Environment's asserts
				prOrig, prCh := orig.Environments[1], ch.Environments[1]
				assert.True(t, prOrig.Ingress.Domain == "domain" &&
					prOrig.Ingress.CloudDNSSecretName == prCh.Ingress.CloudDNSSecretName,
					"the production environment should've been merged correctly")
				// -- Staging Environmnet's asserts
				stgOrig, stgCh := orig.Environments[2], ch.Environments[2]
				assert.True(t, stgOrig.Ingress.Domain == stgCh.Ingress.Domain &&
					stgOrig.Ingress.TLS.Email == "email" && stgOrig.Ingress.TLS.Enabled == stgCh.Ingress.TLS.Enabled,
					"the staging environment should've been merged correctly")
			},
		},
		{
			Name: "Merge StorageConfig test",
			Original: &config.RequirementsConfig{
				Storage: config.StorageConfig{
					Logs: config.StorageEntryConfig{
						Enabled: true,
						URL:     "value1",
					},
					Reports: config.StorageEntryConfig{},
					Repository: config.StorageEntryConfig{
						Enabled: true,
						URL:     "value3",
					},
				},
			},
			Changed: &config.RequirementsConfig{
				Storage: config.StorageConfig{
					Reports: config.StorageEntryConfig{
						Enabled: true,
						URL:     "",
					},
				},
			},
			ValidationFunc: func(orig *config.RequirementsConfig, ch *config.RequirementsConfig) {
				assert.True(t, orig.Storage.Logs.Enabled && orig.Storage.Logs.URL == "value1" &&
					orig.Storage.Repository.Enabled && orig.Storage.Repository.URL == "value3" &&
					orig.Storage.Reports.Enabled == ch.Storage.Reports.Enabled,
					"The storage configuration should've been merged correctly")
			},
		},
	}
	f, err := ioutil.TempFile("", "")
	assert.NoError(t, err)
	defer util.DeleteFile(f.Name())

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			err = tc.Original.MergeSave(tc.Changed, f.Name())
			assert.NoError(t, err, "the merge shouldn't fail for case %s", tc.Name)
			tc.ValidationFunc(tc.Original, tc.Changed)
		})
	}
}

func Test_EnvironmentGitPublic_and_EnvironmentGitPrivate_specified_together_return_error(t *testing.T) {
	content, err := ioutil.ReadFile(path.Join(testDataDir, "git_public_true_git_private_true.yaml"))
	assert.NoError(t, err)

	config := config.NewRequirementsConfig()
	err = yaml.Unmarshal(content, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only EnvironmentGitPublic should be used")
}

func TestHelmfileRequirementValues(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-requirements-config-")
	assert.NoError(t, err, "should create a temporary config dir")

	file := filepath.Join(dir, config.RequirementsConfigFileName)
	requirements := config.NewRequirementsConfig()
	requirements.Cluster.ClusterName = "jx_rocks"

	err = requirements.SaveConfig(file)
	assert.NoError(t, err, "failed to save file %s", file)
	assert.FileExists(t, file)
	valuesFile := filepath.Join(dir, config.RequirementsValuesFileName)

	valuesFileExists, err := util.FileExists(valuesFile)
	assert.False(t, valuesFileExists, "file %s should not exist", valuesFile)

	requirements.Helmfile = true
	err = requirements.SaveConfig(file)
	assert.NoError(t, err, "failed to save file %s", file)
	assert.FileExists(t, file)
	assert.FileExists(t, valuesFile, "file %s should exist", valuesFile)
}

func Test_LoadRequirementsConfig(t *testing.T) {
	t.Parallel()

	var gitPublicTests = []struct {
		requirementsPath   string
		createRequirements bool
	}{
		{"a", false},
		{"a/b", false},
		{"a/b/c", false},
		{"e", true},
		{"e/f", true},
		{"e/f/g", true},
	}

	for _, testCase := range gitPublicTests {
		t.Run(testCase.requirementsPath, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "jx-test-load-requirements-config")
			require.NoError(t, err, "failed to create tmp directory")
			defer func() {
				_ = os.RemoveAll(dir)
			}()

			testPath := filepath.Join(dir, testCase.requirementsPath)
			err = os.MkdirAll(testPath, 0700)
			require.NoError(t, err, "unable to create test path %s", testPath)

			var expectedRequirementsFile string
			if testCase.createRequirements {
				expectedRequirementsFile = filepath.Join(testPath, config.RequirementsConfigFileName)
				dummyRequirementsData := []byte("webhook: prow\n")
				err := ioutil.WriteFile(expectedRequirementsFile, dummyRequirementsData, 0644)
				require.NoError(t, err, "unable to write requirements file %s", expectedRequirementsFile)
			}

			requirements, requirementsFile, err := config.LoadRequirementsConfig(testPath)
			if testCase.createRequirements {
				require.NoError(t, err)
				assert.Equal(t, expectedRequirementsFile, requirementsFile)
				assert.Equal(t, fmt.Sprintf("%s", requirements.Webhook), "prow")
			} else {
				require.Error(t, err)
				assert.Equal(t, "", requirementsFile)
				assert.Nil(t, requirements)
			}
		})
	}
}

func TestLoadRequirementsConfig_load_invalid_yaml(t *testing.T) {
	testDir := path.Join(testDataDir, "jx-requirements-syntax-error")

	absolute, err := filepath.Abs(testDir)
	assert.NoError(t, err, "could not find absolute path of dir %s", testDataDir)

	_, _, err = config.LoadRequirementsConfig(testDir)
	requirementsConfigPath := path.Join(absolute, config.RequirementsConfigFileName)
	assert.EqualError(t, err, fmt.Sprintf("validation failures in YAML file %s:\nenvironments.0: Additional property namespace is not allowed", requirementsConfigPath))
}
