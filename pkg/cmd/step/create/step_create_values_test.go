// +build unit

package create

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ghodss/yaml"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jenkins-x/jx/pkg/config"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/Netflix/go-expect"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"

	clients_test "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	vault_test "github.com/jenkins-x/jx/pkg/vault/mocks"

	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/petergtz/pegomock"
)

var timeout = 5 * time.Second

func TestCreateValuesFileWithVault(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")

	sourceData := filepath.Join("test_data", "step_create_values", "install")
	assert.DirExists(t, sourceData)

	testData, err := ioutil.TempDir("", "test-jx-step-create-values-")
	assert.NoError(t, err)

	err = util.CopyDir(sourceData, testData, true)
	assert.NoError(t, err)
	assert.DirExists(t, testData)

	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		testOrgNameUUID, err := uuid.NewV4()
		assert.NoError(t, err)
		testOrgName := testOrgNameUUID.String()
		testRepoNameUUID, err := uuid.NewV4()
		assert.NoError(t, err)
		testRepoName := testRepoNameUUID.String()
		devEnvRepoName := fmt.Sprintf("environment-%s-%s-dev", testOrgName, testRepoName)
		devEnvRepo, _ := gits.NewFakeRepository(testOrgName, devEnvRepoName, nil, nil)
		mockFactory := clients_test.NewMockFactory()
		commonOpts := opts.NewCommonOptionsWithFactory(mockFactory)
		mockVaultClient := vault_test.NewMockClient()
		devEnv := kube.NewPermanentEnvironmentWithGit("dev", fmt.Sprintf("https://fake.git/%s/%s.git", testOrgName,
			devEnvRepoName))
		devEnv.Spec.Source.URL = devEnvRepo.GitRepo.CloneURL
		devEnv.Spec.Source.Ref = "master"
		pegomock.When(mockFactory.SecretsLocation()).ThenReturn(pegomock.ReturnValue(secrets.VaultLocationKind))
		pegomock.When(mockFactory.CreateSystemVaultClient(pegomock.AnyString())).ThenReturn(pegomock.ReturnValue(mockVaultClient), pegomock.ReturnValue(nil))
		mockHelmer := helm_test.NewMockHelmer()
		installerMock := resources_test.NewMockInstaller()
		testhelpers.ConfigureTestOptionsWithResources(&commonOpts,
			[]runtime.Object{},
			[]runtime.Object{
				devEnv,
			},
			gits.NewGitLocal(),
			nil,
			mockHelmer,
			installerMock,
		)
		testhelpers.MockFactoryWithKubeClients(mockFactory, &commonOpts)

		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		commonOpts.In = console.In
		commonOpts.Out = console.Out
		commonOpts.Err = console.Err

		commonOpts.BatchMode = false

		outFile, err := ioutil.TempFile("", "")
		assert.NoError(t, err)

		o := StepCreateValuesOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: &commonOpts,
				},
			},
			Dir:           testData,
			Name:          "values",
			SecretsScheme: "vault",
			ValuesFile:    outFile.Name(),
		}

		donec := make(chan struct{})
		go func() {
			defer close(donec)
			console.ExpectString("Jenkins X Admin Username")
			console.SendLine("admin")
			console.ExpectString("Jenkins X Admin Password")
			console.SendLine("abc")
			console.ExpectString("HMAC token")
			console.SendLine("abc")
			console.ExpectString("Pipeline bot Git username")
			console.SendLine("james")
			console.ExpectString("Pipeline bot Git token")
			console.SendLine("123456789")
			console.ExpectString("Do you want to configure a Docker Registry?")
			console.SendLine("y")
			console.ExpectString("Docker Registry URL")
			console.SendLine("")
			console.ExpectString("Docker Registry username")
			console.SendLine("james")
			console.ExpectString("Docker Registry password")
			console.SendLine("abc")
			console.ExpectString("Do you want to configure a GPG Key?")
			console.SendLine("n")
			console.ExpectEOF()
		}()
		err = o.Run()
		assert.NoError(r, err)
		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

		// template the goldenfile to insert the generated org and repo name
		values := map[string]string{
			"org":  testOrgName,
			"repo": testRepoName,
		}
		goldenTmplBytes, err := ioutil.ReadFile(filepath.Join(testData, "values.yaml.golden"))
		assert.NoError(t, err)
		goldenTmplStr := string(goldenTmplBytes)
		goldenTmpl, err := template.New("goldenBytes").Parse(goldenTmplStr)
		assert.NoError(t, err)
		var goldenBytes bytes.Buffer
		err = goldenTmpl.Execute(&goldenBytes, values)
		assert.NoError(t, err)

		actual, err := ioutil.ReadFile(outFile.Name())
		assert.NoError(t, err)
		assert.Equal(t, goldenBytes.String(), string(actual))
	})
}

func TestCreateValuesFileWithLocalSecrets(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")

	sourceData := filepath.Join("test_data", "step_create_values", "local_secrets_install")
	assert.DirExists(t, sourceData)

	testData, err := ioutil.TempDir("", "test-jx-step-create-values-")
	assert.NoError(t, err)

	err = util.CopyDir(sourceData, testData, true)
	assert.NoError(t, err)
	assert.DirExists(t, testData)

	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		testOrgNameUUID, err := uuid.NewV4()
		assert.NoError(t, err)
		testOrgName := testOrgNameUUID.String()
		testRepoNameUUID, err := uuid.NewV4()
		assert.NoError(t, err)
		testRepoName := testRepoNameUUID.String()
		devEnvRepoName := fmt.Sprintf("environment-%s-%s-dev", testOrgName, testRepoName)
		devEnvRepo, _ := gits.NewFakeRepository(testOrgName, devEnvRepoName, nil, nil)
		mockFactory := clients_test.NewMockFactory()
		commonOpts := opts.NewCommonOptionsWithFactory(mockFactory)
		mockVaultClient := vault_test.NewMockClient()
		devEnv := kube.NewPermanentEnvironmentWithGit("dev", fmt.Sprintf("https://fake.git/%s/%s.git", testOrgName,
			devEnvRepoName))
		devEnv.Spec.Source.URL = devEnvRepo.GitRepo.CloneURL
		devEnv.Spec.Source.Ref = "master"
		pegomock.When(mockFactory.SecretsLocation()).ThenReturn(pegomock.ReturnValue(secrets.VaultLocationKind))
		pegomock.When(mockFactory.CreateSystemVaultClient(pegomock.AnyString())).ThenReturn(pegomock.ReturnValue(mockVaultClient), pegomock.ReturnValue(nil))
		mockHelmer := helm_test.NewMockHelmer()
		installerMock := resources_test.NewMockInstaller()
		testhelpers.ConfigureTestOptionsWithResources(&commonOpts,
			[]runtime.Object{},
			[]runtime.Object{
				devEnv,
			},
			gits.NewGitLocal(),
			nil,
			mockHelmer,
			installerMock,
		)
		testhelpers.MockFactoryWithKubeClients(mockFactory, &commonOpts)

		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		commonOpts.In = console.In
		commonOpts.Out = console.Out
		commonOpts.Err = console.Err

		commonOpts.BatchMode = false

		o := StepCreateValuesOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: &commonOpts,
				},
			},
			Dir:           testData,
			Name:          "parameters",
			SecretsScheme: "local",
		}

		donec := make(chan struct{})
		go func() {
			defer close(donec)
			console.ExpectString("Jenkins X Admin Username")
			console.SendLine("admin")
			console.ExpectString("Jenkins X Admin Password")
			console.SendLine("abc")
			console.ExpectString("HMAC token")
			console.SendLine("abc")
			console.ExpectString("Pipeline bot Git username")
			console.SendLine("james")
			console.ExpectString("Pipeline bot Git token")
			console.SendLine("123456789")
			console.ExpectString("Do you want to configure a Docker Registry?")
			console.SendLine("y")
			console.ExpectString("Docker Registry URL")
			console.SendLine("")
			console.ExpectString("Docker Registry username")
			console.SendLine("james")
			console.ExpectString("Docker Registry password")
			console.SendLine("abc")
			console.ExpectString("Do you want to configure a GPG Key?")
			console.SendLine("n")
			console.ExpectEOF()
		}()
		err = os.Setenv("OVERRIDE_IN_CLUSTER_CHECK", "true")
		assert.NoError(r, err)
		defer os.Unsetenv("OVERRIDE_IN_CLUSTER_CHECK")
		err = o.Run()
		assert.NoError(r, err)
		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

		jxRequirements, err := ioutil.ReadFile(filepath.Join(testData, "jx-requirements.yml"))
		reqs := &config.RequirementsConfig{}
		err = yaml.Unmarshal(jxRequirements, reqs)
		assert.NoError(r, err)

		assert.Equal(r, "docker.io", reqs.Cluster.Registry, "it should default to \"docker.io\" if no other registry is provided")

		kubeClient, ns, err := mockFactory.CreateKubeClient()
		assert.NoError(r, err)

		secret, err := kubeClient.CoreV1().Secrets(ns).Get("local-param-secrets", v1.GetOptions{})
		assert.NoError(r, err)

		assert.Equal(r, "password: abc\n", string(secret.Data["adminUser.yaml"]))
		assert.Equal(r, "password: abc\n", string(secret.Data["docker.yaml"]))
		assert.Equal(r, "token: \"123456789\"\n", string(secret.Data["pipelineUser.yaml"]))
		assert.Equal(r, "hmacToken: abc\n", string(secret.Data["prow.yaml"]))
	})
}
