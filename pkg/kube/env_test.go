// +build unit

package kube_test

import (
	"testing"

	expect "github.com/Netflix/go-expect"
	jenkinsio_v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	versiond_mocks "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	cmd_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"
	v1 "k8s.io/api/core/v1"

	git_mocks "github.com/jenkins-x/jx/pkg/gits/mocks"
	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	"gopkg.in/AlecAivazis/survey.v1/core"
	k8sv1 "k8s.io/api/core/v1"
	apiextentions_mocks "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube_mocks "k8s.io/client-go/kubernetes/fake"
)

func init() {
	// disable color output for all prompts to simplify testing
	core.DisableColor = true
}

func TestSortEnvironments(t *testing.T) {
	t.Parallel()
	environments := []jenkinsio_v1.Environment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "c",
			},
			Spec: jenkinsio_v1.EnvironmentSpec{
				Order: 100,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "z",
			},
			Spec: jenkinsio_v1.EnvironmentSpec{
				Order: 5,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "d",
			},
			Spec: jenkinsio_v1.EnvironmentSpec{
				Order: 100,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a",
			},
			Spec: jenkinsio_v1.EnvironmentSpec{
				Order: 150,
			},
		},
	}

	kube.SortEnvironments(environments)

	assert.Equal(t, "z", environments[0].Name, "Environment 0")
	assert.Equal(t, "c", environments[1].Name, "Environment 1")
	assert.Equal(t, "d", environments[2].Name, "Environment 2")
	assert.Equal(t, "a", environments[3].Name, "Environment 3")
}

func TestSortEnvironments2(t *testing.T) {
	t.Parallel()
	environments := []jenkinsio_v1.Environment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dev",
			},
			Spec: jenkinsio_v1.EnvironmentSpec{
				Order: 0,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prod",
			},
			Spec: jenkinsio_v1.EnvironmentSpec{
				Order: 200,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "staging",
			},
			Spec: jenkinsio_v1.EnvironmentSpec{
				Order: 100,
			},
		},
	}

	kube.SortEnvironments(environments)

	assert.Equal(t, "dev", environments[0].Name, "Environment 0")
	assert.Equal(t, "staging", environments[1].Name, "Environment 1")
	assert.Equal(t, "prod", environments[2].Name, "Environment 2")
}

func TestReplaceMakeVariable(t *testing.T) {
	t.Parallel()
	lines := []string{"FOO", "NAMESPACE:=\"abc\"", "BAR", "NAMESPACE := \"abc\""}

	actual := append([]string{}, lines...)
	expectedValue := "\"changed\""
	kube.ReplaceMakeVariable(actual, "NAMESPACE", expectedValue)

	assert.Equal(t, "FOO", actual[0], "line 0")
	assert.Equal(t, "NAMESPACE := "+expectedValue, actual[1], "line 1")
	assert.Equal(t, "BAR", actual[2], "line 2")
	assert.Equal(t, "NAMESPACE := "+expectedValue, actual[3], "line 3")
}

func TestGetDevNamespace(t *testing.T) {
	namespace := &k8sv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "jx-testing",
		},
	}
	kubernetesInterface := kube_mocks.NewSimpleClientset(namespace)
	testNS := "jx-testing"
	testEnv := ""

	ns, env, err := kube.GetDevNamespace(kubernetesInterface, testNS)

	assert.NoError(t, err, "Should not error")
	assert.Equal(t, testNS, ns)
	assert.Equal(t, testEnv, env)
}

func TestCreateEnvironmentSurvey(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on Windows. ")
	// namespace fixture
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "jx-testing",
		},
	}
	// mock factory
	factory := cmd_mocks.NewMockFactory()

	// mock Kubernetes interface
	kubernetesInterface := kube_mocks.NewSimpleClientset(namespace)
	// Override CreateKubeClient to return mock Kubernetes interface
	When(factory.CreateKubeClient()).ThenReturn(kubernetesInterface, "jx-testing", nil)

	// mock versiond interface
	versiondInterface := versiond_mocks.NewSimpleClientset()
	// Override CreateJXClient to return mock versiond interface
	When(factory.CreateJXClient()).ThenReturn(versiondInterface, "jx-testing", nil)

	// mock apiExtensions interface
	apiextensionsInterface := apiextentions_mocks.NewSimpleClientset()
	// Override CreateApiExtensionsClient to return mock apiextensions interface
	When(factory.CreateApiExtensionsClient()).ThenReturn(apiextensionsInterface, nil)

	console := tests.NewTerminal(t, nil)
	defer console.Cleanup()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		console.ExpectString("Name:")
		console.SendLine("staging")
		console.ExpectString("Label:")
		console.SendLine("Staging")
		console.ExpectString("Namespace:")
		console.SendLine("jx-testing")
		console.ExpectString("Environment in separate cluster to Dev Environment:")
		console.SendLine("n")
		console.ExpectString("Promotion Strategy:")
		console.SendLine("A")
		console.ExpectString("Order:")
		console.SendLine("1")
		console.ExpectString("We will now create a Git repository to store your staging environment, ok? :")
		console.SendLine("N")
		console.ExpectString("Git URL for the Environment source code:")
		console.SendLine("https://github.com/derekzoolanderreallyreallygoodlooking/staging-env")
		console.ExpectString("Git branch for the Environment source code:")
		console.SendLine("master")
		console.ExpectEOF()
	}()

	batchMode := false
	authConfigSvc := tests.CreateAuthConfigService()
	devEnv := jenkinsio_v1.Environment{}
	data := jenkinsio_v1.Environment{}
	conf := jenkinsio_v1.Environment{}
	forkEnvGitURL := ""
	ns := "jx-testing"
	envDir := ""
	gitRepoOptions := gits.GitRepositoryOptions{}
	helmValues := config.HelmValuesConfig{
		ExposeController: &config.ExposeController{
			Config: config.ExposeControllerConfig{
				Domain: "good.looking.zoolander.com",
			},
		},
	}
	prefix := ""
	gitter := git_mocks.NewMockGitter()
	handles := util.IOFileHandles{
		Err: console.Err,
		In:  console.In,
		Out: console.Out,
	}

	_, err := kube.CreateEnvironmentSurvey(
		batchMode,
		authConfigSvc,
		&devEnv,
		&data,
		&conf,
		true,
		forkEnvGitURL,
		ns,
		versiondInterface,
		kubernetesInterface,
		envDir,
		&gitRepoOptions,
		helmValues,
		prefix,
		gitter,
		nil,
		handles,
	)
	assert.NoError(t, err, "Should not error")

	// Close the slave end of the pty, and read the remaining bytes from the master end.
	// Dump the terminal's screen.
	t.Log(expect.StripTrailingEmptyLines(console.CurrentState()))
	console.Close()
	<-donec
}

func TestGetPreviewEnvironmentReleaseName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		env                 *jenkinsio_v1.Environment
		expectedReleaseName string
	}{
		{
			env:                 nil,
			expectedReleaseName: "",
		},
		{
			env:                 &jenkinsio_v1.Environment{},
			expectedReleaseName: "",
		},
		{
			env:                 kube.NewPreviewEnvironment("test"),
			expectedReleaseName: "",
		},
		{
			env: func() *jenkinsio_v1.Environment {
				env := kube.NewPreviewEnvironment("test")
				if env.Annotations == nil {
					env.Annotations = map[string]string{}
				}
				env.Annotations[kube.AnnotationReleaseName] = "release-name"
				return env
			}(),
			expectedReleaseName: "release-name",
		},
	}

	for i, test := range tests {
		releaseName := kube.GetPreviewEnvironmentReleaseName(test.env)
		if releaseName != test.expectedReleaseName {
			t.Errorf("[%d] Expected release name %s but got %s", i, test.expectedReleaseName, releaseName)
		}
	}
}
