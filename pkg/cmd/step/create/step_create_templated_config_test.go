// +build unit

package create_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/create"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"k8s.io/helm/pkg/chartutil"
)

const tmpl = `
cluster:
  name: {{ .Requirements.cluster.clusterName }}
`

const tmplWithParameters = `
cluster:
  name: {{ .Requirements.cluster.clusterName }}
  key: {{ .Parameters.cluster.key }}
`

const parameters = `
cluster:
  key: value:test-cluster/key:value
`

const renderedTmpl = `
cluster:
  name: test
`
const renderedTmplWithParams = `
cluster:
  name: test
  key: value:test-cluster/key:value
`

func TestCreateTemplatedConfigCommand(t *testing.T) {
	dir, err := ioutil.TempDir("", "step-create-templated-config")
	assert.NoError(t, err, "unable to create a temp directory for test")
	defer os.RemoveAll(dir)

	requirementsFile := filepath.Join(dir, config.RequirementsConfigFileName)
	req := config.NewRequirementsConfig()
	req.Cluster.ClusterName = "test"
	err = req.SaveConfig(requirementsFile)
	assert.NoError(t, err, "unable to save the config file")

	tmplFile := filepath.Join(dir, "config.tmpl.yml")
	err = ioutil.WriteFile(tmplFile, []byte(tmpl), util.DefaultFileWritePermissions)
	assert.NoError(t, err, "unable to save the template file")

	configFile := filepath.Join(dir, "config.yml")

	options := &create.StepCreateTemplatedConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: nil,
		},
		TemplateFile:    tmplFile,
		ConfigFile:      configFile,
		RequirementsDir: dir,
	}

	err = options.Run()
	assert.NoError(t, err, "step create templated config command should run successfully")
	exists, err := util.FileExists(configFile)
	assert.NoError(t, err, "step create templated config should create a config file without error")
	assert.True(t, exists, "step create templated config should create a config file")

	expected, err := chartutil.ReadValues([]byte(renderedTmpl))
	assert.NoError(t, err, "should load the expected config")
	result, err := chartutil.ReadValuesFile(configFile)
	assert.NoError(t, err, "should read the rendered config")
	assert.EqualValues(t, expected, result)
}

func TestCreateTemplatedConfigCommandWithoutRequirements(t *testing.T) {
	dir, err := ioutil.TempDir("", "step-create-templated-config-empty")
	assert.NoError(t, err, "unable to create a second temp directory for test")
	defer os.RemoveAll(dir)

	tmplFile := filepath.Join(dir, "config.tmpl.yml")
	err = ioutil.WriteFile(tmplFile, []byte(tmpl), util.DefaultFileWritePermissions)
	assert.NoError(t, err, "unable to save the template file")

	configFile := filepath.Join(dir, "config.yml")

	options := &create.StepCreateTemplatedConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: nil,
		},
		TemplateFile:    tmplFile,
		ConfigFile:      configFile,
		RequirementsDir: dir,
	}

	err = options.Run()
	assert.Error(t, err, "step created templated config should fail without requirements")
}

func TestCreateTemplatedConfigCommandWithRequirementsInOtherDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "step-create-templated-config")
	assert.NoError(t, err, "unable to create a temp directory for test")
	defer os.RemoveAll(dir)

	dir2, err := ioutil.TempDir("", "step-create-templated-config-req")
	assert.NoError(t, err, "unable to create a temp directory for test")
	defer os.RemoveAll(dir2)

	requirementsFile := filepath.Join(dir2, config.RequirementsConfigFileName)
	req := config.NewRequirementsConfig()
	req.Cluster.ClusterName = "test"
	err = req.SaveConfig(requirementsFile)
	assert.NoError(t, err, "unable to save the config file")

	tmplFile := filepath.Join(dir, "config.tmpl.yml")
	err = ioutil.WriteFile(tmplFile, []byte(tmpl), util.DefaultFileWritePermissions)
	assert.NoError(t, err, "unable to save the template file")

	configFile := filepath.Join(dir, "config.yml")

	options := &create.StepCreateTemplatedConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: nil,
		},
		TemplateFile:    tmplFile,
		ConfigFile:      configFile,
		RequirementsDir: dir2,
	}

	err = options.Run()
	assert.NoError(t, err, "step create templated config command should run successfully")
	exists, err := util.FileExists(configFile)
	assert.NoError(t, err, "step create templated config should create a config file without error")
	assert.True(t, exists, "step create templated config should create a config file")

	expected, err := chartutil.ReadValues([]byte(renderedTmpl))
	assert.NoError(t, err, "should load the expected config")
	result, err := chartutil.ReadValuesFile(configFile)
	assert.NoError(t, err, "should read the rendered config")
	assert.EqualValues(t, expected, result)
}

func TestCreateTemplatedConfigCommandWithParameters(t *testing.T) {
	dir, err := ioutil.TempDir("", "step-create-templated-config")
	assert.NoError(t, err, "unable to create a temp directory for test")
	defer os.RemoveAll(dir)

	requirementsFile := filepath.Join(dir, config.RequirementsConfigFileName)
	req := config.NewRequirementsConfig()
	req.Cluster.ClusterName = "test"
	err = req.SaveConfig(requirementsFile)
	assert.NoError(t, err, "unable to save the config file")

	parametersFile := filepath.Join(dir, helm.ParametersYAMLFile)
	err = ioutil.WriteFile(parametersFile, []byte(parameters), util.DefaultFileWritePermissions)
	assert.NoError(t, err, "unable to save the parameters file")

	tmplFile := filepath.Join(dir, "config.tmpl.yml")
	err = ioutil.WriteFile(tmplFile, []byte(tmplWithParameters), util.DefaultFileWritePermissions)
	assert.NoError(t, err, "unable to save the template file")

	configFile := filepath.Join(dir, "config.yml")

	options := &create.StepCreateTemplatedConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: nil,
		},
		TemplateFile:    tmplFile,
		ParametersFile:  parametersFile,
		ConfigFile:      configFile,
		RequirementsDir: dir,
	}

	err = options.Run()
	assert.NoError(t, err, "step create templated config command should run successfully")
	exists, err := util.FileExists(configFile)
	assert.NoError(t, err, "step create templated config should create a config file without error")
	assert.True(t, exists, "step create templated config should create a config file")

	expected, err := chartutil.ReadValues([]byte(renderedTmplWithParams))
	assert.NoError(t, err, "should load the expected config")
	result, err := chartutil.ReadValuesFile(configFile)
	assert.NoError(t, err, "should read the rendered config")
	assert.EqualValues(t, expected, result)
}

func TestCreateTemplatedConfigCommandWithWrongParametersFilename(t *testing.T) {
	dir, err := ioutil.TempDir("", "step-create-templated-config")
	assert.NoError(t, err, "unable to create a temp directory for test")
	defer os.RemoveAll(dir)

	requirementsFile := filepath.Join(dir, config.RequirementsConfigFileName)
	req := config.NewRequirementsConfig()
	req.Cluster.ClusterName = "test"
	err = req.SaveConfig(requirementsFile)
	assert.NoError(t, err, "unable to save the config file")

	parametersFile := filepath.Join(dir, "test-parameters.yaml")
	err = ioutil.WriteFile(parametersFile, []byte(parameters), util.DefaultFileWritePermissions)
	assert.NoError(t, err, "unable to save the parameters file")

	tmplFile := filepath.Join(dir, "config.tmpl.yml")
	err = ioutil.WriteFile(tmplFile, []byte(tmplWithParameters), util.DefaultFileWritePermissions)
	assert.NoError(t, err, "unable to save the template file")

	configFile := filepath.Join(dir, "config.yml")

	options := &create.StepCreateTemplatedConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: nil,
		},
		TemplateFile:    tmplFile,
		ParametersFile:  parametersFile,
		ConfigFile:      configFile,
		RequirementsDir: dir,
	}

	err = options.Run()
	assert.Error(t, err, "step create templated config command should run fail with wrong parameters file name")
}
