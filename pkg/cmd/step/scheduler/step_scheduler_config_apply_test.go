// +build unit

package scheduler_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/scheduler"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/jenkins-x/jx/pkg/prow"
	uuid "github.com/satori/go.uuid"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/test-infra/prow/plugins"

	cmd_test "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/petergtz/pegomock"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
)

func TestStepSchedulerConfigApplyNonGitopsAll(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("nongitops_all", false, t)
	err := testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	kubeClient, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.KubeClient()
	_, devEnv := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.GetDevEnv()
	verifyProwConfigMap(err, kubeClient, devEnv, t, testOptions)
}

func TestStepSchedulerConfigApplyNonGitopsDefaultScheduler(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("nongitops_default_scheduler", false, t)
	err := testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	kubeClient, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.KubeClient()
	_, devEnv := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.GetDevEnv()
	verifyProwConfigMap(err, kubeClient, devEnv, t, testOptions)
}

func TestStepSchedulerConfigApplyNonGitopsRepoScheduler(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("nongitops_repo_scheduler", false, t)
	err := testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	kubeClient, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.KubeClient()
	_, devEnv := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.GetDevEnv()
	verifyProwConfigMap(err, kubeClient, devEnv, t, testOptions)
}

func TestStepSchedulerConfigApplyNonGitopsRepoGroupScheduler(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("nongitops_repogroup_scheduler", false, t)
	err := testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	kubeClient, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.KubeClient()
	_, devEnv := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.GetDevEnv()
	verifyProwConfigMap(err, kubeClient, devEnv, t, testOptions)
}

func TestStepSchedulerConfigApplyGitopsAll(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("gitops_all", true, t)
	originalJxHome, tempJxHome, err := testhelpers.CreateTestJxHomeDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestJxHomeDir(originalJxHome, tempJxHome)
		assert.NoError(t, err)
	}()
	originalKubeCfg, tempKubeCfg, err := testhelpers.CreateTestKubeConfigDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestKubeConfigDir(originalKubeCfg, tempKubeCfg)
		assert.NoError(t, err)
	}()
	envDir, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := filepath.Join(envDir, testOptions.DevEnvName)
	testOptions.StepSchedulerConfigApplyOptions.CloneDir = devEnvDir
	err = testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	verifyProwGitopsConfig(err, testOptions, devEnvDir, t)
}

func TestStepSchedulerConfigApplyGitopsDefaultScheduler(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("gitops_default_scheduler", true, t)
	originalJxHome, tempJxHome, err := testhelpers.CreateTestJxHomeDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestJxHomeDir(originalJxHome, tempJxHome)
		assert.NoError(t, err)
	}()
	originalKubeCfg, tempKubeCfg, err := testhelpers.CreateTestKubeConfigDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestKubeConfigDir(originalKubeCfg, tempKubeCfg)
		assert.NoError(t, err)
	}()
	envDir, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := filepath.Join(envDir, testOptions.DevEnvName)
	testOptions.StepSchedulerConfigApplyOptions.CloneDir = devEnvDir
	err = testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	verifyProwGitopsConfig(err, testOptions, devEnvDir, t)
}

func TestStepSchedulerConfigApplyGitopsRepoScheduler(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("gitops_repo_scheduler", true, t)
	originalJxHome, tempJxHome, err := testhelpers.CreateTestJxHomeDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestJxHomeDir(originalJxHome, tempJxHome)
		assert.NoError(t, err)
	}()
	originalKubeCfg, tempKubeCfg, err := testhelpers.CreateTestKubeConfigDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestKubeConfigDir(originalKubeCfg, tempKubeCfg)
		assert.NoError(t, err)
	}()
	envDir, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := filepath.Join(envDir, testOptions.DevEnvName)
	testOptions.StepSchedulerConfigApplyOptions.CloneDir = devEnvDir
	err = testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	verifyProwGitopsConfig(err, testOptions, devEnvDir, t)
}

func TestStepSchedulerConfigApplyGitopsRepoGroupScheduler(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerApplyTestOptions{}
	testOptions.createSchedulerTestOptions("gitops_repogroup_scheduler", true, t)
	originalJxHome, tempJxHome, err := testhelpers.CreateTestJxHomeDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestJxHomeDir(originalJxHome, tempJxHome)
		assert.NoError(t, err)
	}()
	originalKubeCfg, tempKubeCfg, err := testhelpers.CreateTestKubeConfigDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestKubeConfigDir(originalKubeCfg, tempKubeCfg)
		assert.NoError(t, err)
	}()
	envDir, err := testOptions.StepSchedulerConfigApplyOptions.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := filepath.Join(envDir, testOptions.DevEnvName)
	testOptions.StepSchedulerConfigApplyOptions.CloneDir = devEnvDir
	err = testOptions.StepSchedulerConfigApplyOptions.Run()
	assert.NoError(t, err)
	verifyProwGitopsConfig(err, testOptions, devEnvDir, t)
}

func verifyProwGitopsConfig(err error, testOptions *StepSchedulerApplyTestOptions, devEnvDir string, t *testing.T) {
	generatedConfig, err := testOptions.loadGeneratedConfigFromGitopsRepo(devEnvDir, "config.yaml")
	assert.NoError(t, err)
	expectedConfig, err := testOptions.loadExpectedConfig(testOptions.TestType, "config.yaml", testOptions.DevEnvRepo.Owner, testOptions.DevRepoName)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, generatedConfig)
	generatedPluginConfig, err := testOptions.loadGeneratedConfigFromGitopsRepo(devEnvDir, "plugins.yaml")
	assert.NoError(t, err)
	expectedPluginConfig, err := testOptions.loadExpectedConfig(testOptions.TestType, "plugins.yaml", testOptions.DevEnvRepo.Owner, testOptions.DevRepoName)
	assert.NoError(t, err)
	assert.Equal(t, expectedPluginConfig, generatedPluginConfig)
}

func verifyProwConfigMap(err error, kubeClient kubernetes.Interface, devEnv *v1.Environment, t *testing.T, testOptions *StepSchedulerApplyTestOptions) {
	configConfigMap, err := kubeClient.CoreV1().ConfigMaps(devEnv.Namespace).Get("config", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, configConfigMap)
	assert.NotNil(t, configConfigMap.Data)
	expectedConfig, err := testOptions.loadExpectedConfig(testOptions.TestType, "config.yaml", testOptions.DevEnvRepo.Owner, testOptions.DevRepoName)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, configConfigMap.Data["config.yaml"])
	pluginsConfigMap, err := kubeClient.CoreV1().ConfigMaps(devEnv.Namespace).Get("plugins", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, pluginsConfigMap)
	assert.NotNil(t, pluginsConfigMap.Data)
	expectedPluginConfig, err := testOptions.loadExpectedConfig(testOptions.TestType, "plugins.yaml", testOptions.DevEnvRepo.Owner, testOptions.DevRepoName)
	assert.NoError(t, err)
	assert.Equal(t, pluginsConfigMap.Data["plugins.yaml"], expectedPluginConfig)
}

// AppTestOptions contains all useful data from the test environment initialized by `prepareInitialPromotionEnv`
type StepSchedulerApplyTestOptions struct {
	StepSchedulerConfigApplyOptions *scheduler.StepSchedulerConfigApplyOptions
	DevEnvRepo                      *gits.FakeRepository
	DevRepoName                     string
	DevEnvName                      string
	TestType                        string
}

// CreateAppTestOptions configures the mock environment for running apps related tests
func (o *StepSchedulerApplyTestOptions) createSchedulerTestOptions(testType string, gitOps bool, t *testing.T) {
	mockFactory := cmd_test.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(mockFactory)
	testhelpers.ConfigureTestOptions(&commonOpts, gits_test.NewMockGitter(), helm_test.NewMockHelmer())
	o.StepSchedulerConfigApplyOptions = &scheduler.StepSchedulerConfigApplyOptions{}
	o.StepSchedulerConfigApplyOptions.Agent = "prow"
	o.StepSchedulerConfigApplyOptions.CommonOptions = &commonOpts
	o.TestType = testType
	gitter := gits.NewGitCLI()
	devEnvRepoName := ""
	jxResources := []runtime.Object{}
	schedulers := o.loadSchedulers(testType)
	if schedulers != nil {
		jxResources = append(jxResources, schedulers)
	}
	sourceRepos := o.loadSourceRepos(testType)
	if sourceRepos != nil {
		jxResources = append(jxResources, sourceRepos)
	}
	sourceRepoGroups := o.loadSourceRepoGroups(testType)
	if sourceRepoGroups != nil {
		jxResources = append(jxResources, sourceRepoGroups)
	}
	testOrgNameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	// Fix the order so the generated config is consistent
	testOrgName := "Z" + testOrgNameUUID.String()
	testRepoNameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	testRepoName := testRepoNameUUID.String()
	devEnvRepoName = fmt.Sprintf("environment-%s-%s-dev", testOrgName, testRepoName)
	devEnv := kube.NewPermanentEnvironmentWithGit("dev", fmt.Sprintf("https://fake.git/%s/%s.git", testOrgName,
		devEnvRepoName))
	addFiles := func(dir string) error {
		// Really we should have a dummy environment chart but for now let's just mock it out as needed
		err = os.MkdirAll(filepath.Join(dir, "templates"), 0700)
		if err != nil {
			return err
		}
		data, err := yaml.Marshal(devEnv)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(dir, "templates", "dev-env.yaml"), data, 0755)
		if err != nil {
			return err
		}
		return nil
	}

	fakeRepo, _ := gits.NewFakeRepository(testOrgName, testRepoName, nil, nil)
	devEnvRepo, _ := gits.NewFakeRepository(testOrgName, devEnvRepoName, addFiles, gitter)

	fakeGitProvider := gits.NewFakeProvider(fakeRepo, devEnvRepo)
	fakeGitProvider.User.Username = testOrgName

	if !gitOps {
		devEnv.Spec.Source.URL = ""
		devEnv.Spec.Source.Ref = ""
	}
	devEnv.Spec.TeamSettings.DefaultScheduler.Name = "default-scheduler"
	sourceRepo := &v1.SourceRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devEnvRepo.Name(),
			Namespace: devEnv.Namespace,
		},
		Spec: v1.SourceRepositorySpec{
			Org:  devEnvRepo.Owner,
			Repo: devEnvRepo.Name(),
		},
	}
	jxResources = append(jxResources, devEnv, sourceRepo)
	o.DevEnvRepo = devEnvRepo
	o.DevRepoName = testRepoName
	o.DevEnvName = devEnv.Name
	if gitOps {

		pluginConfig := &plugins.Configuration{}
		pluginYAML, err := yaml.Marshal(pluginConfig)
		assert.NoError(t, err)
		data := make(map[string]string)
		data[prow.ProwPluginsFilename] = string(pluginYAML)
		cm := &v12.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			Data: data,
			ObjectMeta: metav1.ObjectMeta{
				Name:      prow.ProwPluginsConfigMapName,
				Namespace: "jx",
			},
		}
		kubeResources := []runtime.Object{}
		kubeResources = append(kubeResources, cm)
		installerMock := resources_test.NewMockInstaller()
		testhelpers.ConfigureTestOptionsWithResources(o.StepSchedulerConfigApplyOptions.CommonOptions,
			kubeResources,
			jxResources,
			gitter,
			fakeGitProvider,
			o.StepSchedulerConfigApplyOptions.Helm(),
			installerMock,
		)
		err = testhelpers.CreateTestEnvironmentDir(o.StepSchedulerConfigApplyOptions.CommonOptions)
		assert.NoError(t, err)
	} else {
		installerMock := resources_test.NewMockInstaller()
		testhelpers.ConfigureTestOptionsWithResources(o.StepSchedulerConfigApplyOptions.CommonOptions,
			[]runtime.Object{},
			jxResources,
			gits.NewGitLocal(),
			nil,
			o.StepSchedulerConfigApplyOptions.Helm(),
			installerMock,
		)

	}
}

func (o *StepSchedulerApplyTestOptions) loadGeneratedConfigFromGitopsRepo(devEnvDir string, fileName string) (string, error) {
	resourceLocation := filepath.Join(devEnvDir, "prow", fileName)
	_, err := os.Stat(resourceLocation)
	if err != nil {
		return "", err
	}
	configData, err := ioutil.ReadFile(resourceLocation)
	if err != nil {
		return "", err
	}
	return string(configData), err
}

func (o *StepSchedulerApplyTestOptions) loadExpectedConfig(testType string, fileName string, gitOrg string, gitRepo string) (string, error) {
	resourceLocation := filepath.Join("test_data", "step_scheduler_config_apply", testType, fileName)
	_, err := os.Stat(resourceLocation)
	if err != nil {
		return "", err
	}
	configData, err := ioutil.ReadFile(resourceLocation)
	if err != nil {
		return "", err
	}
	data := string(configData)
	if gitRepo != "" {
		data = strings.Replace(data, "@@DEV_ENV_ORG@@", gitOrg, -1)
		data = strings.Replace(data, "@@DEV_ENV_REPO@@", gitRepo, -1)
	}
	return data, err
}

func (o *StepSchedulerApplyTestOptions) loadSchedulers(testType string) *v1.SchedulerList {
	resourceLocation := filepath.Join("test_data", "step_scheduler_config_apply", testType, "schedulers.yaml")
	_, err := os.Stat(resourceLocation)
	if err != nil {
		return nil
	}
	schedulerData, err := ioutil.ReadFile(resourceLocation)
	if err != nil {
		return nil
	}
	schedulers := &v1.SchedulerList{}
	err = yaml.Unmarshal(schedulerData, schedulers)
	return schedulers
}

func (o *StepSchedulerApplyTestOptions) loadSourceRepos(testType string) *v1.SourceRepositoryList {
	resourceLocation := filepath.Join("test_data", "step_scheduler_config_apply", testType, "sourcerepositories.yaml")
	_, err := os.Stat(resourceLocation)
	if err != nil {
		return nil
	}
	sourceRepositoryData, err := ioutil.ReadFile(resourceLocation)
	if err != nil {
		return nil
	}
	sourceRepositories := &v1.SourceRepositoryList{}
	err = yaml.Unmarshal(sourceRepositoryData, sourceRepositories)
	return sourceRepositories
}

func (o *StepSchedulerApplyTestOptions) loadSourceRepoGroups(testType string) *v1.SourceRepositoryGroupList {
	resourceLocation := filepath.Join("test_data", "step_scheduler_config_apply", testType, "sourcerepositorygroups.yaml")
	_, err := os.Stat(resourceLocation)
	if err != nil {
		return nil
	}
	sourceRepositorygroupData, err := ioutil.ReadFile(resourceLocation)
	if err != nil {
		return nil
	}
	sourceRepoGroups := &v1.SourceRepositoryGroupList{}
	err = yaml.Unmarshal(sourceRepositorygroupData, sourceRepoGroups)
	return sourceRepoGroups
}
