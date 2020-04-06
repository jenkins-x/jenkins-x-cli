// +build unit

package namespace_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/cmd/namespace"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	kuber_mocks "github.com/jenkins-x/jx/pkg/kube/mocks"
	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Test_display_current_namespace(t *testing.T) {
	commonOpts, _, stdOutFileName, stdErrFileName, kubeConfig := setUp(t)
	defer func() {
		_ = os.Remove(stdOutFileName)
		_ = os.Remove(stdErrFileName)
		_ = os.Remove(kubeConfig)
	}()

	namespace := &namespace.NamespaceOptions{
		CommonOptions: commonOpts,
	}

	err := namespace.Run()
	assert.NoError(t, err, "should not error")

	expected := "Using namespace 'snafu' from context named 'foo-context' on server 'https://fubar.com'.\n"
	actual := sanitizedContent(t, stdOutFileName)
	assert.Equal(t, expected, actual, "wrong message returned")

	assert.Empty(t, sanitizedContent(t, stdErrFileName))
}

func Test_change_current_namespace(t *testing.T) {
	commonOpts, kubeClient, stdOutFileName, stdErrFileName, kubeConfig := setUp(t)
	defer func() {
		_ = os.Remove(stdOutFileName)
		_ = os.Remove(stdErrFileName)
		_ = os.Remove(kubeConfig)
	}()

	ns := core_v1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "acme",
		},
	}
	_, err := kubeClient.CoreV1().Namespaces().Create(&ns)

	namespace := &namespace.NamespaceOptions{
		CommonOptions: commonOpts,
	}

	commonOpts.Args = []string{"acme"}

	err = namespace.Run()
	assert.NoError(t, err, "should not error")

	expected := "Now using namespace 'acme' on server 'https://fubar.com'.\n"
	actual := sanitizedContent(t, stdOutFileName)
	assert.Equal(t, expected, actual, "wrong message returned")

	assert.Empty(t, sanitizedContent(t, stdErrFileName))
}

func Test_change_to_new_namespace_with_create(t *testing.T) {
	commonOpts, kubeClient, stdOutFileName, stdErrFileName, kubeConfig := setUp(t)
	defer func() {
		_ = os.Remove(stdOutFileName)
		_ = os.Remove(stdErrFileName)
		_ = os.Remove(kubeConfig)
	}()

	namespace := &namespace.NamespaceOptions{
		CommonOptions: commonOpts,
		Create:        true,
	}

	testNamespaceName := "test-ns"
	commonOpts.Args = []string{testNamespaceName}

	err := namespace.Run()
	assert.NoError(t, err, "should not error")

	expected := fmt.Sprintf("Now using namespace '%s' on server 'https://fubar.com'.\n", testNamespaceName)
	actual := sanitizedContent(t, stdOutFileName)
	assert.Equal(t, expected, actual, "wrong message returned")

	assert.Empty(t, sanitizedContent(t, stdErrFileName))

	_, err = kubeClient.CoreV1().Namespaces().Get(testNamespaceName, meta_v1.GetOptions{})
	if err != nil {
		t.Fatalf("The namespace '%s' should have been created", testNamespaceName)
	}
}

func Test_change_to_unknown_namespace_creates_error(t *testing.T) {
	commonOpts, _, stdOutFileName, stdErrFileName, kubeConfig := setUp(t)
	defer func() {
		_ = os.Remove(stdOutFileName)
		_ = os.Remove(stdErrFileName)
		_ = os.Remove(kubeConfig)
	}()

	namespace := &namespace.NamespaceOptions{
		CommonOptions: commonOpts,
	}

	commonOpts.Args = []string{"unknown"}

	err := namespace.Run()
	assert.Error(t, err, "should return error since namespace is unknown")
	assert.Equal(t, "namespaces \"unknown\" not found", err.Error())
	assert.Empty(t, sanitizedContent(t, stdOutFileName))
}

func sanitizedContent(t *testing.T, fileName string) string {
	raw, err := ioutil.ReadFile(fileName)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	content := string(raw)
	// remove potential ANSI escape sequences
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	content = re.ReplaceAllString(content, "")

	return content
}

func setUp(t *testing.T) (*opts.CommonOptions, *fake.Clientset, string, string, string) {
	// mock factory
	factory := mocks.NewMockFactory()
	// mock Kubernetes interface
	kubeInterface := fake.NewSimpleClientset()

	// Override CreateKubeClient to return mock Kubernetes interface
	When(factory.CreateKubeClient()).ThenReturn(kubeInterface, "jx-testing", nil)

	kubeMock := kuber_mocks.NewMockKuber()
	fakeKubeConfig := api.NewConfig()
	fakeKubeConfig.CurrentContext = "foo-context"
	fakeKubeConfig.Clusters = map[string]*api.Cluster{
		"foo-cluster": {
			Server: "https://fubar.com",
		},
	}
	fakeKubeConfig.Contexts = map[string]*api.Context{
		"foo-context": {
			Cluster:   "foo-cluster",
			Namespace: "snafu",
		},
		"acme-context": {
			Cluster:   "foo-cluster",
			Namespace: "acme",
		},
	}
	pathOptions := clientcmd.NewDefaultPathOptions()
	When(kubeMock.LoadConfig()).ThenReturn(fakeKubeConfig, pathOptions, nil)

	tmpKubeConfig, err := ioutil.TempFile("", "jx-test-namespace")
	err = os.Setenv("KUBECONFIG", tmpKubeConfig.Name())
	if err != nil {
		assert.Fail(t, err.Error())
	}

	// Setup options
	tmpFileStdOut, err := ioutil.TempFile("", "jx-test-namespace")
	if err != nil {
		assert.Fail(t, err.Error())
	}

	tmpFileStdErr, err := ioutil.TempFile("", "jx-test-namespace")
	if err != nil {
		assert.Fail(t, err.Error())
	}

	commonOpts := opts.NewCommonOptionsWithFactory(factory)
	commonOpts.Out = tmpFileStdOut
	commonOpts.Err = tmpFileStdErr
	// all tests in batch mode, non interactive
	commonOpts.BatchMode = true

	commonOpts.SetKube(kubeMock)

	return &commonOpts, kubeInterface, tmpFileStdOut.Name(), tmpFileStdErr.Name(), tmpKubeConfig.Name()
}
