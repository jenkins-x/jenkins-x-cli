// +build unit

package scheduler_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/scheduler"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	uuid "github.com/satori/go.uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmd_test "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/petergtz/pegomock"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
)

func TestStepSchedulerConfigMigrateNonGitopsBasic(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerMigrateTestOptions{}
	testOptions.createSchedulerMigrateTestOptions("nongitops_basic", false, t)
	testOptions.StepSchedulerConfigMigrateOptions.ProwConfigFileLocation = "test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/config.yaml"
	testOptions.StepSchedulerConfigMigrateOptions.ProwPluginsFileLocation = "test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/plugins.yaml"
	err := testOptions.StepSchedulerConfigMigrateOptions.Run()
	assert.NoError(t, err)
	jxClient, _, err := testOptions.StepSchedulerConfigMigrateOptions.CommonOptions.JXClient()
	_, devEnv := testOptions.StepSchedulerConfigMigrateOptions.CommonOptions.GetDevEnv()
	verifySchedulerResources(err, jxClient, devEnv, t, testOptions)
}

func TestStepSchedulerConfigMigrateNonGitopsAdvanced(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerMigrateTestOptions{}
	testOptions.createSchedulerMigrateTestOptions("nongitops_advanced", false, t)
	testOptions.StepSchedulerConfigMigrateOptions.ProwConfigFileLocation = "test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/config.yaml"
	testOptions.StepSchedulerConfigMigrateOptions.ProwPluginsFileLocation = "test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/plugins.yaml"
	err := testOptions.StepSchedulerConfigMigrateOptions.Run()
	assert.NoError(t, err)
	jxClient, _, err := testOptions.StepSchedulerConfigMigrateOptions.CommonOptions.JXClient()
	_, devEnv := testOptions.StepSchedulerConfigMigrateOptions.CommonOptions.GetDevEnv()
	verifySchedulerResources(err, jxClient, devEnv, t, testOptions)
}

func TestStepSchedulerConfigMigrateGitopsBasic(t *testing.T) {
	tests.SkipForWindows(t, "NewTerminal() does not work on windows")
	pegomock.RegisterMockTestingT(t)
	testOptions := &StepSchedulerMigrateTestOptions{}
	testOptions.createSchedulerMigrateTestOptions("gitops_basic", true, t)
	testOptions.StepSchedulerConfigMigrateOptions.ProwConfigFileLocation = "test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/config.yaml"
	testOptions.StepSchedulerConfigMigrateOptions.ProwPluginsFileLocation = "test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/plugins.yaml"
	envDir, err := testOptions.StepSchedulerConfigMigrateOptions.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := filepath.Join(envDir, testOptions.DevEnvName)
	testOptions.StepSchedulerConfigMigrateOptions.CloneDir = devEnvDir
	err = testOptions.StepSchedulerConfigMigrateOptions.Run()
	assert.NoError(t, err)
	verifySchedulerGitOps(err, t, testOptions, devEnvDir, "cb-kubecd-jx-scheduler-test-group-repo-scheduler")
	verifySchedulerGitOps(err, t, testOptions, devEnvDir, "cb-kubecd-jx-scheduler-test-repo-scheduler")
	verifySchedulerGitOps(err, t, testOptions, devEnvDir, "default-scheduler")
}

func verifySchedulerResources(err error, jxClient versioned.Interface, devEnv *v1.Environment, t *testing.T, testOptions *StepSchedulerMigrateTestOptions) {
	schedulers, err := jxClient.JenkinsV1().Schedulers(devEnv.Namespace).List(metav1.ListOptions{})
	sort.Slice(schedulers.Items, func(i, j int) bool {
		if schedulers.Items[i].Name < schedulers.Items[j].Name {
			return true
		}
		return false
	})
	assert.NoError(t, err)
	migratedSchedulerYaml, err := yaml.Marshal(schedulers)
	assert.NoError(t, err)
	expectedSchedulerYaml, err := ioutil.ReadFile("test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/schedulers.yaml")
	assert.NoError(t, err)
	assert.Equal(t, string(expectedSchedulerYaml), string(migratedSchedulerYaml))
}

func (o *StepSchedulerMigrateTestOptions) loadSchedulersFromGitopsRepo(devEnvDir string, fileName string) (string, error) {
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

func (o *StepSchedulerMigrateTestOptions) loadExpectedSchedulers(testType string, fileName string, gitOrg string, gitRepo string) (string, error) {
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
	return data, err
}

func verifySchedulerGitOps(err error, t *testing.T, testOptions *StepSchedulerMigrateTestOptions, devEnvDir string, schedulerName string) {
	assert.NoError(t, err)
	expectedSchedulerYaml, err := ioutil.ReadFile("test_data/step_scheduler_config_migrate/" + testOptions.TestType + "/" + schedulerName + "-sch.yaml")
	assert.NoError(t, err)
	migratedSchedulerYaml, err := ioutil.ReadFile(filepath.Join(devEnvDir, "templates", schedulerName+"-sch.yaml"))
	assert.NoError(t, err)
	assert.Equal(t, string(migratedSchedulerYaml), string(expectedSchedulerYaml))
	assert.NoError(t, err)

	if schedulerName != "default-scheduler" {
		sourceRepo := &v1.SourceRepository{}
		sourceRepofile := filepath.Join(devEnvDir, "templates", strings.TrimSuffix(schedulerName, "-scheduler")+"-repo.yaml")
		schedulerWiringYaml, err := ioutil.ReadFile(sourceRepofile)
		err = yaml.Unmarshal(schedulerWiringYaml, sourceRepo)
		assert.NoError(t, err)
		assert.Equal(t, sourceRepo.Spec.Scheduler.Name, schedulerName)
	} else {
		devEnvResource := &v1.Environment{}
		devEnvFile := filepath.Join(devEnvDir, "templates", "dev-env.yaml")
		schedulerWiringYaml, err := ioutil.ReadFile(devEnvFile)
		err = yaml.Unmarshal(schedulerWiringYaml, devEnvResource)
		assert.NoError(t, err)
		assert.Equal(t, devEnvResource.Spec.TeamSettings.DefaultScheduler.Name, schedulerName)
	}

}

// StepSchedulerMigrateTestOptions contains all useful data for testing prow config migrations
type StepSchedulerMigrateTestOptions struct {
	StepSchedulerConfigMigrateOptions *scheduler.StepSchedulerConfigMigrateOptions
	DevEnvRepo                        *gits.FakeRepository
	DevRepoName                       string
	DevEnvName                        string
	TestType                          string
}

// CreateAppTestOptions configures the mock environment for running apps related tests
func (o *StepSchedulerMigrateTestOptions) createSchedulerMigrateTestOptions(testType string, gitOps bool, t *testing.T) {
	mockFactory := cmd_test.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(mockFactory)
	testhelpers.ConfigureTestOptions(&commonOpts, gits_test.NewMockGitter(), helm_test.NewMockHelmer())
	o.StepSchedulerConfigMigrateOptions = &scheduler.StepSchedulerConfigMigrateOptions{}
	o.StepSchedulerConfigMigrateOptions.Agent = "prow"
	o.StepSchedulerConfigMigrateOptions.CommonOptions = &commonOpts
	o.TestType = testType

	gitter := gits.NewGitCLI()

	devEnvRepoName := ""
	jxResources := []runtime.Object{}
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
	sourceRepo := &v1.SourceRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Environment",
			APIVersion: jenkinsio.GroupName + "/" + jenkinsio.Version,
		},
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
		installerMock := resources_test.NewMockInstaller()
		testhelpers.ConfigureTestOptionsWithResources(o.StepSchedulerConfigMigrateOptions.CommonOptions,
			[]runtime.Object{},
			jxResources,
			gitter,
			fakeGitProvider,
			o.StepSchedulerConfigMigrateOptions.Helm(),
			installerMock,
		)
		err = testhelpers.CreateTestEnvironmentDir(o.StepSchedulerConfigMigrateOptions.CommonOptions)
		assert.NoError(t, err)
	} else {
		installerMock := resources_test.NewMockInstaller()
		testhelpers.ConfigureTestOptionsWithResources(o.StepSchedulerConfigMigrateOptions.CommonOptions,
			[]runtime.Object{},
			jxResources,
			gits.NewGitLocal(),
			nil,
			o.StepSchedulerConfigMigrateOptions.Helm(),
			installerMock,
		)

	}
}
