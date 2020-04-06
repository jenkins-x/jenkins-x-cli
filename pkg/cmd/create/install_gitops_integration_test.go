// +build integration

package create_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fake_clients "github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/platform"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/jenkins-x/jx/pkg/testkube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/helm/pkg/chartutil"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/stretchr/testify/assert"
)

func TestInstallGitOps(t *testing.T) {
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

	tempDir, err := ioutil.TempDir("", "test-install-gitops")
	assert.NoError(t, err)

	const clusterAdminRoleName = "cluster-admin"

	clusterAdminRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterAdminRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "watch", "list", "create", "delete", "update", "patch"},
				APIGroups: []string{""},
				Resources: []string{"*"},
			},
		},
	}

	co := opts.CommonOptions{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}
	o := create.CreateInstallOptions(&co)
	o.SetFactory(fake_clients.NewFakeFactory())

	gitter := gits.NewGitFake()
	helmer := helm_test.NewMockHelmer()
	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{
			clusterAdminRole,
			testkube.CreateFakeGitSecret(),
		},
		[]runtime.Object{},
		gitter,
		nil,
		helmer,
		resources_test.NewMockInstaller(),
	)
	o.CommonOptions.SetGit(gitter)
	o.CommonOptions.InstallDependencies = true
	o.CommonOptions.SetHelm(helmer)

	o.InitOptions.CommonOptions = o.CommonOptions
	o.CreateEnvOptions.CommonOptions = o.CommonOptions

	jxClient, ns, err := o.JXClientAndDevNamespace()
	require.NoError(t, err, "failed to create JXClient")
	kubeClient, err := o.KubeClient()
	require.NoError(t, err, "failed to create KubeClient")

	// lets remove the default generated Environment so we can assert that we don't create any environments
	// via: jx import --gitops
	jxClient.JenkinsV1().Environments(ns).Delete(kube.LabelValueDevEnvironment, nil)
	assertNoEnvironments(t, jxClient, ns)

	testOrg := "mytestorg"
	testEnvPrefix := "test"
	o.Flags.Provider = cloud.GKE
	o.Flags.Dir = tempDir
	o.Flags.GitOpsMode = true
	o.Flags.NoGitOpsEnvApply = true
	o.Flags.NoGitOpsVault = true
	o.Flags.NoGitOpsEnvSetup = true
	o.Flags.NoDefaultEnvironments = true
	o.Flags.DisableSetKubeContext = true
	o.Flags.EnvironmentGitOwner = testOrg
	o.Flags.Domain = "mytestdomain"
	o.Flags.DefaultEnvironmentPrefix = testEnvPrefix
	o.InitOptions.Flags.SkipTiller = true
	o.InitOptions.Flags.NoTiller = true
	o.InitOptions.Flags.SkipIngress = true
	o.InitOptions.Flags.SkipClusterRole = true
	o.InitOptions.Flags.NoGitValidate = true
	o.InitOptions.Flags.UserClusterRole = clusterAdminRoleName
	o.BatchMode = true

	// lets use a fake git provider
	testDevRepo := "environment-dev-mytest"
	o.GitRepositoryOptions.ServerURL = gits.FakeGitURL
	o.GitRepositoryOptions.Owner = testOrg
	o.GitRepositoryOptions.RepoName = testDevRepo
	o.GitRepositoryOptions.Username = "mytestuser"
	o.GitRepositoryOptions.ApiToken = "mytestoken"

	cobraCmd := cobra.Command{}
	cobraCmd.ParseFlags([]string{})
	o.CommonOptions.Cmd = &cobraCmd

	err = o.Run()
	require.NoError(t, err, "Failed to run jx install")

	t.Logf("Completed install to dir %s", tempDir)

	envsDir, err := util.EnvironmentsDir()
	require.NoError(t, err, "Failed to get the environments dir")
	devEnvDir := fmt.Sprintf("environment-%s-dev", testEnvPrefix)
	outDir := filepath.Join(envsDir, testOrg, devEnvDir)
	envDir := filepath.Join(outDir, "env")

	chartFile := filepath.Join(envDir, helm.ChartFileName)
	reqFile := filepath.Join(envDir, helm.RequirementsFileName)
	secretsFile := filepath.Join(envDir, helm.SecretsFileName)
	valuesFile := filepath.Join(envDir, helm.ValuesFileName)
	assert.FileExists(t, chartFile)
	assert.FileExists(t, reqFile)
	assert.FileExists(t, secretsFile)
	assert.FileExists(t, valuesFile)

	for _, name := range []string{"dev-env.yaml", "ingress-config-configmap.yaml", "jx-install-config-secret.yaml"} {
		assert.FileExists(t, filepath.Join(envDir, "templates", name))
	}
	for _, name := range []string{".gitignore", "Jenkinsfile", "README.md"} {
		assert.FileExists(t, filepath.Join(outDir, name))
	}
	if !o.Flags.DisableSetKubeContext {
		for _, name := range []string{"jx-install-config-configmap.yaml"} {
			assert.FileExists(t, filepath.Join(envDir, "templates", name))
		}
	}
	req, err := helm.LoadRequirementsFile(reqFile)
	require.NoError(t, err)

	require.Equal(t, 1, len(req.Dependencies), "Number of dependencies in file %s", reqFile)
	dep0 := req.Dependencies[0]
	require.NotNil(t, dep0, "first dependency in file %s", reqFile)
	assert.Equal(t, kube.DefaultChartMuseumURL, dep0.Repository, "requirement.dependency[0].Repository")
	assert.Equal(t, platform.JenkinsXPlatformChartName, dep0.Name, "requirement.dependency[0].Name")
	assert.NotEmpty(t, dep0.Version, "requirement.dependency[0].Version")

	values, err := chartutil.ReadValuesFile(valuesFile)
	require.NoError(t, err, "Failed to load values file", valuesFile)
	// assertValuesHasPathValue(t, "values.yaml", values, "jenkins-x-platform.expose")
	assertValuesHasPathValue(t, "values.yaml", values, "jenkins-x-platform.postinstalljob.enabled")

	secrets, err := chartutil.ReadValuesFile(secretsFile)
	require.NoError(t, err, "Failed to load secrets file", secretsFile)
	assertValuesHasPathValue(t, "secrets.yaml", secrets, "jenkins-x-platform.PipelineSecrets")

	// lets verify that we don't have any created resources in the cluster - as everything should be created in the file system
	assertNoEnvironments(t, jxClient, ns)

	_, cmNames, _ := kube.GetConfigMaps(kubeClient, ns)
	assert.Empty(t, cmNames, "Expected no ConfigMap names in namespace %s", ns)

	_, secretNames, _ := kube.GetSecrets(kubeClient, ns)
	assert.Equal(t, []string{"jx-pipeline-git-fake"}, secretNames, "Secret names in namespace %s", ns)
}

func assertNoEnvironments(t *testing.T, jxClient versioned.Interface, ns string) {
	_, envNames, _ := kube.GetEnvironments(jxClient, ns)
	assert.Empty(t, envNames, "Expected no Environment names in namespace %s", ns)
}

// assertValuesHasPathValue asserts that the Values object has the given
func assertValuesHasPathValue(t *testing.T, message string, values chartutil.Values, key string) (interface{}, error) {
	keys := strings.Split(key, ".")
	lastIdx := len(keys) - 1
	for i, key := range keys {
		value := values.AsMap()[key]
		path := strings.Join(keys[0:i+1], ".")
		if value == nil {
			if !assert.NotNil(t, value, "%s values does not contain entry for key %s", message, path) {
				return nil, nil
			}
		}
		if i == lastIdx {
			return value, nil
		}
		m, ok := value.(map[string]interface{})
		if !ok {
			assert.Failf(t, "%s value for key %s should be a a map [string]interface{} but was %#v", message, path, value)
			return nil, nil
		}
		values = chartutil.Values(m)
	}
	return nil, nil
}
