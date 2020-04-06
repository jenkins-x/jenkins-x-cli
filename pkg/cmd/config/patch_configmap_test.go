// +build unit

package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStepModifyConfigMapRootLevel(t *testing.T) {
	t.Parallel()

	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	kubeClient, ns, err := options.KubeClientAndNamespace()
	assert.NoError(t, err)

	wd, err := os.Getwd()
	assert.NoError(t, err)

	testDataPath := filepath.Join(wd, "..", "test_data", "step_config", "config_map1.txt")
	bytes, err := ioutil.ReadFile(testDataPath)
	assert.NoErrorf(t, err, "The test data in folder %s should be read", testDataPath)

	configMap := &v1.ConfigMap{}
	err = yaml.Unmarshal(bytes, configMap)
	assert.NoErrorf(t, err, "The test data in folder %s should be unmarshaled", testDataPath)

	_, err = kubeClient.CoreV1().ConfigMaps(ns).Create(configMap)
	assert.NoError(t, err, "the test config map should be created")

	o := &StepModifyConfigMapOptions{
		StepOptions: step.StepOptions{
			CommonOptions: options,
		},

		JSONPatch:     `{"metadata": {"initializers": {"result": {"status": "newstatus"}}}}`,
		ConfigMapName: "config",
		// The fake client only supports strategic
		Type: "strategic",
	}

	err = o.Run()
	assert.NoError(t, err)

	updatedConfig, err := kubeClient.CoreV1().ConfigMaps(ns).Get("config", k8sv1.GetOptions{})
	assert.NoError(t, err, "there should be a config map called config")

	assert.Equal(t, "newstatus", updatedConfig.Initializers.Result.Status)
}

func TestStepModifyConfigMapFirstLevelPropertySet(t *testing.T) {
	t.Parallel()

	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	kubeClient, ns, err := options.KubeClientAndNamespace()
	assert.NoError(t, err)

	wd, err := os.Getwd()
	assert.NoError(t, err)

	testDataPath := filepath.Join(wd, "..", "test_data", "step_config", "config_map1.txt")
	bytes, err := ioutil.ReadFile(testDataPath)
	assert.NoErrorf(t, err, "The test data in folder %s should be read", testDataPath)

	configMap := &v1.ConfigMap{}
	err = yaml.Unmarshal(bytes, configMap)
	assert.NoErrorf(t, err, "The test data in folder %s should be Unmarshal", testDataPath)

	_, err = kubeClient.CoreV1().ConfigMaps(ns).Create(configMap)
	assert.NoError(t, err, "the test config map should be created")

	o := &StepModifyConfigMapOptions{
		StepOptions: step.StepOptions{
			CommonOptions: options,
		},

		JSONPatch:          `[{"op": "replace", "path": "/plank", "value" : {"url": "http:"}}]`,
		ConfigMapName:      "config",
		FirstLevelProperty: "config.yaml",
	}

	err = o.Run()
	assert.NoError(t, err)

	updatedConfig, err := kubeClient.CoreV1().ConfigMaps(ns).Get("config", k8sv1.GetOptions{})
	assert.NoError(t, err, "there should be a config map called config")

	prowConfig := updatedConfig.Data["config.yaml"]
	k := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(prowConfig), &k)
	assert.NoError(t, err)

	k = k["plank"].(map[string]interface{})

	assert.Equal(t, "http:", k["url"])
}
