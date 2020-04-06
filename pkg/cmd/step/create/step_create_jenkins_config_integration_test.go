// +build integration

package create_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/create"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCreateJenkinsConfig(t *testing.T) {
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

	testData := path.Join("test_data", "step_create_jenkins_config")
	assert.DirExists(t, testData)

	outputFile, err := ioutil.TempFile("", "test-step-create-jenkins-config.xml")
	require.NoError(t, err)
	fileName := outputFile.Name()

	files, err := ioutil.ReadDir(testData)
	assert.NoError(t, err)

	// load the test ConfigMaps
	ns := "jx"
	runtimeObjects := []runtime.Object{}
	for _, f := range files {
		if !f.IsDir() {
			name := f.Name()
			srcFile := filepath.Join(testData, name)
			data, err := ioutil.ReadFile(srcFile)
			require.NoError(t, err, "failed to read file %s", srcFile)

			cm := &corev1.ConfigMap{}
			err = yaml.Unmarshal(data, cm)
			require.NoError(t, err, "failed to unmarshal file %s", srcFile)

			require.NotEqual(t, "", cm.Name, "file %s contains a ConfigMap with no name", srcFile)
			cm.Namespace = ns
			runtimeObjects = append(runtimeObjects, cm)
		}
	}

	o := &create.StepCreateJenkinsConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			},
		},
		Output: fileName,
	}

	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		runtimeObjects,
		nil,
		gits.NewGitCLI(),
		nil,
		helm.NewHelmCLI("helm", helm.V2, "", true),
		resources_test.NewMockInstaller(),
	)

	err = o.Run()
	require.NoError(t, err, "failed to run step")

	t.Logf("Generated config.xml file at %s\n", fileName)

	assert.FileExists(t, fileName, "failed to create valid file")

	tests.AssertFileContains(t, fileName, "<name>maven</name>")
	tests.AssertFileContains(t, fileName, "<name>gradle</name>")
}
