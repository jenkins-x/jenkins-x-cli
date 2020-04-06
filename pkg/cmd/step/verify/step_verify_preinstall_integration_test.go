// +build integration

package verify_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/jenkins-x/jx/pkg/config"

	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/namespace"
	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/verify"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/stretchr/testify/assert"
)

const (
	testDeployNamespace = "new-jx-ns"
)

func TestStepVerifyPreInstallTerraformKaniko(t *testing.T) {
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		origJxHome := os.Getenv("JX_HOME")

		tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
		assert.NoError(t, err)

		err = os.Setenv("JX_HOME", tmpJxHome)
		assert.NoError(t, err)

		defer func() {
			_ = os.RemoveAll(tmpJxHome)
			err = os.Setenv("JX_HOME", origJxHome)
		}()

		options := createTestStepVerifyPreInstallOptions(filepath.Join("test_data", "preinstall", "terraform_kaniko"))

		_, origNamespace, err := options.KubeClientAndDevNamespace()
		assert.NoError(t, err)
		defer resetNamespace(t, origNamespace)

		err = options.Run()
		assert.Errorf(r, err, "the command should have failed for terraform and kaniko with a missing kaniko secret")
	})
}

func TestStepVerifyPreInstallNoKanikoNoLazyCreate(t *testing.T) {
	// TODO the fake k8s client always seems to lazily create a namespace on demand so the 'jx step verify preinstall' never fails
	t.SkipNow()

	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	options := createTestStepVerifyPreInstallOptions(filepath.Join("test_data", "preinstall", "no_kaniko_or_terraform"))
	// explicitly disable lazy create
	options.LazyCreateFlag = "false"

	_, origNamespace, err := options.KubeClientAndDevNamespace()
	assert.NoError(t, err)
	defer resetNamespace(t, origNamespace)

	err = options.Run()
	if err != nil {
		t.Logf("returned error: %s", err.Error())
	}
	assert.Errorf(t, err, "the command should have failed due to missing namespace")
}

func TestStepVerifyPreInstallNoKanikoLazyCreate(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	options := createTestStepVerifyPreInstallOptions(filepath.Join("test_data", "preinstall", "no_kaniko_or_terraform"))

	_, origNamespace, err := options.KubeClientAndDevNamespace()
	assert.NoError(t, err)
	defer resetNamespace(t, origNamespace)

	// we default to lazy create if not using terraform
	err = options.Run()
	assert.NoErrorf(t, err, "the command should not have failed as we should have lazily created the deploy namespace")
}

func TestStepVerifyPreInstallNoTLS(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	options := createTestStepVerifyPreInstallOptions(filepath.Join("test_data", "preinstall", "no_tls"))

	_, origNamespace, err := options.KubeClientAndDevNamespace()
	assert.NoError(t, err)
	defer resetNamespace(t, origNamespace)

	// we default to lazy create if not using terraform
	err = options.Run()
	assert.NoError(t, err)
}

func TestStepVerifyPreInstallRequirements(t *testing.T) {
	tests := map[string]bool{
		"lighthouse_gitlab": true,
		"prow_github":       true,
		"prow_gitlab":       false,
	}

	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	for dir, actual := range tests {
		testDir := filepath.Join("test_data", "preinstall", dir)
		assert.DirExists(t, testDir)
		options := createTestStepVerifyPreInstallOptions(testDir)
		options.Namespace = "jx"

		_, origNamespace, err := options.KubeClientAndDevNamespace()
		assert.NoError(t, err)
		defer resetNamespace(t, origNamespace)

		requirements, requirementsFileName, err := config.LoadRequirementsConfig(testDir)
		assert.NoError(t, err, "for test %s", dir)

		err = options.ValidateRequirements(requirements, requirementsFileName)
		if actual {
			assert.NoError(t, err, "for test %s", dir)
			t.Logf("correctly validated test %s", dir)
		} else {
			assert.Error(t, err, "for test %s", dir)
			t.Logf("correctly failed to validate test %s with error: %v", dir, err)
		}
	}
}

func TestStepVerifyPreInstallSetClusterRequirementsViaEnvars(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	options := createTestStepVerifyPreInstallOptions(filepath.Join("test_data", "preinstall", "set_cluster_req_via_envvar"))

	kc, origNamespace, err := options.KubeClientAndDevNamespace()
	assert.NoError(t, err)
	defer resetNamespace(t, origNamespace)

	// we default to lazy create if not using terraform
	err = options.VerifyInstallConfig(kc, origNamespace, config.NewRequirementsConfig(), "")
	assert.NoErrorf(t, err, "the command should not have failed as we should have lazily created the deploy namespace")

	t.Parallel()

	commonOpts := opts.CommonOptions{
		BatchMode: false,
	}
	o := &verify.StepVerifyIngressOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &commonOpts,
		},
	}

	dir, err := ioutil.TempDir("", "test_set_cluster_req_via_envvar")
	assert.NoError(t, err, "should create a temporary config dir")

	o.Dir = dir
	file := filepath.Join(o.Dir, config.RequirementsConfigFileName)
	requirements := getBaseRequirements()

	// using nip.io on gke should disable the use of external dns as we cannot transfer domain ownership to google dns
	requirements.Ingress.Domain = "34.76.24.247.nip.io"
	requirements.Cluster.Provider = "gke"

	err = requirements.SaveConfig(file)
	assert.NoError(t, err, "failed to save file %s", file)

	requirements, fileName, err := config.LoadRequirementsConfig(o.Dir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", o.Dir)
	assert.FileExists(t, fileName)

	assert.Equal(t, false, requirements.Ingress.ExternalDNS, "requirements.Ingress.ExternalDNS")

}

func createTestStepVerifyPreInstallOptions(dir string) *verify.StepVerifyPreInstallOptions {
	options := &verify.StepVerifyPreInstallOptions{
		DisableVerifyHelm:    true,
		TestKanikoSecretData: "test-kaniko-secret",
	}
	// fake the output stream to be checked later
	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	options.CommonOptions = &commonOpts
	testhelpers.ConfigureTestOptions(options.CommonOptions, gits.NewGitCLI(), helm_test.NewMockHelmer())
	testhelpers.SetFakeFactoryFromKubeClients(options.CommonOptions)
	options.Dir = dir
	options.Namespace = testDeployNamespace
	options.Err = os.Stdout
	options.Out = os.Stdout
	return options
}

func resetNamespace(t *testing.T, ns string) {
	commonOpts := opts.NewCommonOptionsWithFactory(clients.NewFactory())
	commonOpts.Out = os.Stdout
	namespaceOptions := &namespace.NamespaceOptions{
		CommonOptions: &commonOpts,
	}
	namespaceOptions.Args = []string{ns}
	err := namespaceOptions.Run()
	assert.NoError(t, err)
}

func getBaseRequirements() *config.RequirementsConfig {
	requirements := config.NewRequirementsConfig()
	requirements.Cluster.ProjectID = "test-project"
	requirements.Cluster.ClusterName = "test-cluster"
	requirements.Cluster.EnvironmentGitOwner = "test-org"
	requirements.Cluster.Zone = "test-zone"
	return requirements
}
