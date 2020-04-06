// +build unit

package verify

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/boot"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/util"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

func TestStepVerifyEnvironmentsOptions_StoreRequirementsInTeamSettings(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	testOptions := &StepVerifyEnvironmentsOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: options,
			},
		},
	}

	requirementsYamlFile := path.Join("test_data", "verify_environments", "default", "jx-requirements.yml")
	exists, err := util.FileExists(requirementsYamlFile)
	assert.NoError(t, err)
	assert.True(t, exists)

	bytes, err := ioutil.ReadFile(requirementsYamlFile)
	assert.NoError(t, err)
	requirements := &config.RequirementsConfig{}
	err = yaml.Unmarshal(bytes, requirements)
	assert.NoError(t, err)

	err = testOptions.storeRequirementsInTeamSettings(requirements)
	assert.NoError(t, err, "there shouldn't be any error adding the requirements to TeamSettings")

	teamSettings, err := testOptions.TeamSettings()
	assert.NoError(t, err)

	requirementsCm := teamSettings.BootRequirements
	assert.NotEqual(t, "", requirementsCm, "the BootRequirements field should be present and not empty")

	mapRequirements, err := config.GetRequirementsConfigFromTeamSettings(teamSettings)
	assert.NoError(t, err)

	assert.Equal(t, requirements, mapRequirements)
}

func TestStepVerifyEnvironmentsOptions_StoreRequirementsConfigMapWithModification(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	requirementsYamlFile := path.Join("test_data", "verify_environments", "ghe", "jx-requirements.yml")
	exists, err := util.FileExists(requirementsYamlFile)
	assert.NoError(t, err)
	assert.True(t, exists)

	bytes, err := ioutil.ReadFile(requirementsYamlFile)
	assert.NoError(t, err)
	requirements := &config.RequirementsConfig{}
	err = yaml.Unmarshal(bytes, requirements)
	assert.NoError(t, err)

	err = options.ModifyDevEnvironment(func(env *v1.Environment) error {
		env.Spec.TeamSettings.BootRequirements = string(bytes)
		return nil
	})
	assert.NoError(t, err)

	// We make a modification to the requirements and we should see it when we retrieve the ConfigMap later
	requirements.Storage.Logs = config.StorageEntryConfig{
		Enabled: true,
		URL:     "gs://randombucket",
	}

	testOptions := &StepVerifyEnvironmentsOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: options,
			},
		},
	}

	err = testOptions.storeRequirementsInTeamSettings(requirements)
	assert.NoError(t, err, "there shouldn't be any error updating the team settings")

	teamSettings, err := testOptions.TeamSettings()
	assert.NoError(t, err)

	assert.Equal(t, requirements.Cluster.GitServer, teamSettings.GitServer, "the GitServer field should be set to the requirements.cluster.gitServer value")

	requirementsCm := teamSettings.BootRequirements
	assert.NotEqual(t, "", requirementsCm, "the BootRequirements field should be present and not empty")

	mapRequirements, err := config.GetRequirementsConfigFromTeamSettings(teamSettings)
	assert.NoError(t, err)

	assert.Equal(t, requirements.Storage.Logs, mapRequirements.Storage.Logs, "the change done before calling"+
		"VerifyRequirementsInTeamSettings should be present in the retrieved configuration")
}

func Test_ReadEnvironment(t *testing.T) {
	origConfigRepoURL, foundConfigRepoURLEnvKey := os.LookupEnv(boot.ConfigRepoURLEnvVarName)
	origConfigRepoRef, foundConfigRepoRefEnvKey := os.LookupEnv(boot.ConfigBaseRefEnvVarName)
	defer func() {
		if foundConfigRepoURLEnvKey {
			_ = os.Setenv(boot.ConfigRepoURLEnvVarName, origConfigRepoURL)
		}

		if foundConfigRepoRefEnvKey {
			_ = os.Setenv(boot.ConfigBaseRefEnvVarName, origConfigRepoRef)
		}
	}()

	o := StepVerifyEnvironmentsOptions{}

	var tests = []struct {
		url         string
		ref         string
		expectError bool
		errorString string
	}{
		{"https://github.com/jenkins-x/jenkins-x-boot-config", "master", false, ""},
		{"https://github.com/jenkins-x/jenkins-x-boot-config", "", true, "the environment variable CONFIG_BASE_REF must be specified"},
		{"", "master", true, "the environment variable CONFIG_REPO_URL must be specified"},
		{"", "", true, "[the environment variable CONFIG_REPO_URL must be specified, the environment variable CONFIG_BASE_REF must be specified]"},
	}

	for _, testCase := range tests {
		t.Run(fmt.Sprintf("%s-%s", testCase.url, testCase.ref), func(t *testing.T) {
			if testCase.url == "" {
				err := os.Unsetenv(boot.ConfigRepoURLEnvVarName)
				assert.NoError(t, err)
			} else {
				err := os.Setenv(boot.ConfigRepoURLEnvVarName, testCase.url)
				assert.NoError(t, err)
			}

			if testCase.ref == "" {
				err := os.Unsetenv(boot.ConfigBaseRefEnvVarName)
				assert.NoError(t, err)
			} else {
				err := os.Setenv(boot.ConfigBaseRefEnvVarName, testCase.ref)
				assert.NoError(t, err)

			}

			repo, ref, err := o.readEnvironment()
			if testCase.expectError {
				assert.Error(t, err)
				assert.Equal(t, testCase.errorString, err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.url, repo)
				assert.Equal(t, testCase.ref, ref)
			}
		})
	}
}

func Test_ModifyPipelineGitEnvVars(t *testing.T) {
	origGitAuthorName, foundGitAuthorName := os.LookupEnv(gitAuthorNameEnvKey)
	origGitAuthorEmail, foundGitAuthorEmail := os.LookupEnv(gitAuthorEmailEnvKey)
	origGitCommitterName, foundGitCommitterName := os.LookupEnv(gitCommitterNameEnvKey)
	origGitCommitterEmail, foundGitCommitterEmail := os.LookupEnv(gitCommitterEmailEnvKey)

	defer func() {
		if foundGitAuthorName {
			_ = os.Setenv(gitAuthorNameEnvKey, origGitAuthorName)
		}
		if foundGitAuthorEmail {
			_ = os.Setenv(gitAuthorEmailEnvKey, origGitAuthorEmail)
		}
		if foundGitCommitterName {
			_ = os.Setenv(gitCommitterNameEnvKey, origGitCommitterName)
		}
		if foundGitCommitterEmail {
			_ = os.Setenv(gitCommitterEmailEnvKey, origGitCommitterEmail)
		}
	}()

	dir, err := ioutil.TempDir("", "modify-pipeline-git-env-vars-")
	assert.NoError(t, err)

	defer func() {
		err := os.RemoveAll(dir)
		assert.NoError(t, err)
	}()

	testDir := path.Join("test_data", "verify_environments", "pipeline_git_env_vars")
	exists, err := util.DirExists(testDir)
	assert.NoError(t, err)
	assert.True(t, exists)

	err = util.CopyDir(testDir, dir, true)
	assert.NoError(t, err)

	o := StepVerifyEnvironmentsOptions{}

	err = o.modifyPipelineGitEnvVars(dir)
	assert.NoError(t, err)

	parameterValues, err := helm.LoadParametersValuesFile(dir)
	assert.NoError(t, err)

	expectedUsername := util.GetMapValueAsStringViaPath(parameterValues, "pipelineUser.username")
	expectedEmail := util.GetMapValueAsStringViaPath(parameterValues, "pipelineUser.email")
	assert.NotEqual(t, "", expectedUsername, "should not have empty expected username")
	assert.NotEqual(t, "", expectedEmail, "should not have empty expected email")

	newGitAuthorName, _ := os.LookupEnv(gitAuthorNameEnvKey)
	newGitAuthorEmail, _ := os.LookupEnv(gitAuthorEmailEnvKey)
	newGitCommitterName, _ := os.LookupEnv(gitCommitterNameEnvKey)
	newGitCommitterEmail, _ := os.LookupEnv(gitCommitterEmailEnvKey)

	assert.Equal(t, expectedUsername, newGitAuthorName)
	assert.Equal(t, expectedUsername, newGitCommitterName)
	assert.Equal(t, expectedEmail, newGitAuthorEmail)
	assert.Equal(t, expectedEmail, newGitCommitterEmail)

	confFileName := filepath.Join(dir, config.ProjectConfigFileName)
	projectConf, err := config.LoadProjectConfigFile(confFileName)
	assert.NoError(t, err)

	pipelineEnv := projectConf.PipelineConfig.Pipelines.Release.Pipeline.Environment

	assert.Equal(t, expectedUsername, pipelineEnvValueForKey(pipelineEnv, gitAuthorNameEnvKey))
	assert.Equal(t, expectedUsername, pipelineEnvValueForKey(pipelineEnv, gitCommitterNameEnvKey))
	assert.Equal(t, expectedEmail, pipelineEnvValueForKey(pipelineEnv, gitAuthorEmailEnvKey))
	assert.Equal(t, expectedEmail, pipelineEnvValueForKey(pipelineEnv, gitCommitterEmailEnvKey))
}

func Test_getEnvironmentURLTemplate(t *testing.T) {
	var tests = []struct {
		name             string
		cfg              *config.EnvironmentConfig
		expectedTemplate string
	}{
		{"Default Template Test", &config.EnvironmentConfig{}, config.ExposeDefaultURLTemplate},
		{"Custom Template Test", &config.EnvironmentConfig{URLTemplate: "NewTemplate"}, "NewTemplate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedTemplate, getEnvironmentURLTemplate(test.cfg), "The returned template should be the expected based on input")
		})
	}
}

func pipelineEnvValueForKey(envVars []corev1.EnvVar, key string) string {
	for _, entry := range envVars {
		if entry.Name == key {
			return entry.Value
		}
	}
	return ""
}
