// +build unit

package opts

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientmocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubemocks "k8s.io/client-go/kubernetes/fake"

	. "github.com/petergtz/pegomock"
)

const (
	testCommandName = "foo"
	testFlagName    = "snafu"
)

var (
	cmdUnderTest        *cobra.Command
	commonOptsUnderTest CommonOptions
)

type TestFlags struct {
	Snafu      bool       `mapstructure:"snafu"`
	ChildFlags ChildFlags `mapstructure:"children"`
}

type ChildFlags struct {
	Child string `mapstructure:"child"`
}

func Test_FlagExplicitlySet_returns_true_if_flag_explicitly_set_to_false(t *testing.T) {
	setupTestCommand()

	err := cmdUnderTest.Flags().Parse([]string{testCommandName, fmt.Sprintf("--%s", testFlagName), "false"})
	assert.NoError(t, err)
	explicit := commonOptsUnderTest.IsFlagExplicitlySet(testFlagName)
	assert.True(t, explicit, "the flag should be explicitly set")
}

func Test_FlagExplicitlySet_returns_true_if_flag_explicitly_set_to_true(t *testing.T) {
	setupTestCommand()

	err := cmdUnderTest.Flags().Parse([]string{testCommandName, fmt.Sprintf("--%s", testFlagName), "true"})
	assert.NoError(t, err)
	explicit := commonOptsUnderTest.IsFlagExplicitlySet(testFlagName)
	assert.True(t, explicit, "the flag should be explicitly set")
}

func Test_FlagExplicitlySet_returns_false_if_flag_is_not_set(t *testing.T) {
	setupTestCommand()

	explicit := commonOptsUnderTest.IsFlagExplicitlySet(testFlagName)
	assert.False(t, explicit, "the flag should not be explicitly set")
}

func Test_FlagExplicitlySet_returns_false_if_flag_is_unknown(t *testing.T) {
	setupTestCommand()

	explicit := commonOptsUnderTest.IsFlagExplicitlySet("fubar")
	assert.False(t, explicit, "the flag should be unknown")
}

func Test_NotifyProgress(t *testing.T) {
	setupTestCommand()

	commonOptsUnderTest.NotifyProgress(LogInfo, "hello %s", "world\n")

	actual := ""
	expectedText := "hello again\n"

	commonOptsUnderTest.NotifyCallback = func(level LogLevel, text string) {
		actual = text
	}

	commonOptsUnderTest.NotifyProgress(LogInfo, expectedText)

	assert.Equal(t, expectedText, actual, "callback receives the log message")
}

func Test_JXNamespace(t *testing.T) {
	setupTestCommand()

	// mock factory
	factory := clientmocks.NewMockFactory()

	// namespace fixture
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "jx-testing",
		},
	}

	// mock Kubernetes interface
	kubernetesInterface := kubemocks.NewSimpleClientset(namespace)
	// Override CreateKubeClient to return mock Kubernetes interface
	When(factory.CreateKubeClient()).ThenReturn(kubernetesInterface, "jx-testing", nil)

	commonOptsUnderTest.SetFactory(factory)

	kubeClient, ns, err := commonOptsUnderTest.KubeClientAndNamespace()
	assert.NoError(t, err, "Failed to create kube client")

	if err == nil {
		resource, err := kubeClient.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
		assert.NoError(t, err, "Failed to query namespace")
		if err == nil {
			log.Logger().Warnf("Found namespace %#v", resource)
		}
		assert.Equal(t, namespace.Namespace, ns)
	}
}

func Test_GetConfiguration(t *testing.T) {
	setupTestCommand()

	fileContent := fmt.Sprintf("%s: %t\n", testFlagName, true)
	configFile, removeTmp := setupTestConfig(t, fileContent)

	defer removeTmp(configFile)

	commonOptsUnderTest = CommonOptions{}
	commonOptsUnderTest.ConfigFile = configFile

	testFlags := TestFlags{}
	err := commonOptsUnderTest.GetConfiguration(&testFlags)
	assert.NoError(t, err, "Failed to GetConfiguration")

	assert.Equal(t, true, testFlags.Snafu)
}

func Test_configExists_child(t *testing.T) {
	setupTestCommand()

	valuesYaml := fmt.Sprintf("children:\n  child: foo")
	configFile, removeTmp := setupTestConfig(t, valuesYaml)

	defer removeTmp(configFile)

	assert.True(t, configExists("children", "child"))
}

func Test_configExists_no_path(t *testing.T) {
	setupTestCommand()

	valuesYaml := fmt.Sprintf("snafu: true")
	configFile, removeTmp := setupTestConfig(t, valuesYaml)

	defer removeTmp(configFile)

	assert.True(t, configExists("", "snafu"))
}

func Test_configNotExists(t *testing.T) {
	setupTestCommand()

	valuesYaml := fmt.Sprintf("children:\n  child: foo")
	configFile, removeTmp := setupTestConfig(t, valuesYaml)

	defer removeTmp(configFile)

	assert.False(t, configExists("children", "son"))
}

func Test_IsJXBoot(t *testing.T) {
	orig, found := os.LookupEnv(jxInterpretPipelineEnvKey)
	defer func() {
		if found {
			_ = os.Setenv(jxInterpretPipelineEnvKey, orig)
		}
	}()

	o := CommonOptions{}

	err := os.Setenv(jxInterpretPipelineEnvKey, "")
	assert.NoError(t, err)
	assert.False(t, o.IsJXBoot(), "we should not be in boot mode")

	err = os.Setenv(jxInterpretPipelineEnvKey, "false")
	assert.NoError(t, err)
	assert.False(t, o.IsJXBoot(), "we should not be in boot mode")

	err = os.Setenv(jxInterpretPipelineEnvKey, "true")
	assert.NoError(t, err)
	assert.True(t, o.IsJXBoot(), "we should be in boot mode")
}

func setupTestConfig(t *testing.T, config string) (string, func(string)) {
	setupTestCommand()

	tmpDir, err := ioutil.TempDir("", "")
	require.Nil(t, err, "Failed creating tmp dir")
	configFile := path.Join(tmpDir, "config.yaml")
	err = ioutil.WriteFile(configFile, []byte(config), 0640)
	require.Nil(t, err, "Failed writing config yaml file")

	commonOptsUnderTest.ConfigFile = configFile

	testFlags := TestFlags{}
	err = commonOptsUnderTest.GetConfiguration(&testFlags)
	assert.NoError(t, err, "Failed to GetConfiguration")

	removeAllFunc := func(configFile string) {
		_ = os.RemoveAll(configFile)
	}
	return configFile, removeAllFunc
}

func setupTestCommand() {
	var flag bool
	cmdUnderTest = &cobra.Command{
		Use:   testCommandName,
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			// noop
		},
	}
	cmdUnderTest.Flags().BoolVar(&flag, testFlagName, false, "")
	_ = viper.BindPFlag(testFlagName, cmdUnderTest.Flags().Lookup(testFlagName))

	commonOptsUnderTest = CommonOptions{}
	commonOptsUnderTest.Cmd = cmdUnderTest
}
