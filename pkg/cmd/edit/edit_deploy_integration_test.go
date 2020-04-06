// +build integration

package edit_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/edit"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmdEditDeploy(t *testing.T) {
	t.Parallel()

	srcDir := filepath.Join("test_data", "edit_deploy", "testapp")
	require.DirExists(t, srcDir)

	tmpDir, err := ioutil.TempDir("", "test-step-edit-deploy-")
	require.NoError(t, err, "failed to create temp dir")
	require.DirExists(t, tmpDir, "could not create temp dir for running tests")

	type testData struct {
		name     string
		callback func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error
		fail     bool
	}

	tests := []testData{
		{
			name: "team-enable-knative-canary-and-hpa",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertTeamEditDeploy(t, eo, dir, opts.DeployKindKnative, true, true)
			},
		},
		{
			name: "team-enable-canary-and-hpa",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertTeamEditDeploy(t, eo, dir, opts.DeployKindDefault, true, true)
			},
		},
		{
			name: "team-enable-canary",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertTeamEditDeploy(t, eo, dir, opts.DeployKindDefault, true, false)
			},
		},
		{
			name: "team-disable-knative-canary-and-hpa",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertTeamEditDeploy(t, eo, dir, opts.DeployKindDefault, false, false)
			},
		},
		{
			name: "enable-knative-canary-and-hpa",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertEditDeploy(t, eo, dir, opts.DeployKindKnative, true, true)
			},
		},
		{
			name: "enable-canary-and-hpa",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertEditDeploy(t, eo, dir, opts.DeployKindDefault, true, true)
			},
		},
		{
			name: "enable-canary",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertEditDeploy(t, eo, dir, opts.DeployKindDefault, true, false)
			},
		},
		{
			name: "disable-knative-canary-and-hpa",
			callback: func(t *testing.T, eo *edit.EditDeployKindOptions, dir string) error {
				return assertEditDeploy(t, eo, dir, opts.DeployKindDefault, false, false)
			},
		},
	}

	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	testhelpers.ConfigureTestOptions(&commonOpts, commonOpts.Git(), commonOpts.Helm())
	commonOpts.Out = os.Stdout
	commonOpts.Err = os.Stderr

	for i, tt := range tests {
		name := tt.name
		if name == "" {
			name = fmt.Sprintf("test%d", i)
		}
		dir := filepath.Join(tmpDir, name)

		err = util.CopyDir(srcDir, dir, true)
		require.NoError(t, err, "failed to copy source to %s", dir)

		_, eo := edit.NewCmdEditDeployKindAndOption(&commonOpts)
		eo.Dir = dir

		err = tt.callback(t, eo, dir)
		if tt.fail {
			require.Error(t, err, "test %s should have failed", name)
			if err != nil {
				t.Logf("test %s got expected error %s", name, err.Error())
			}

		} else {
			require.NoError(t, err, "failed to run test %s", name)
		}
	}
}

func assertEditDeploy(t *testing.T, eo *edit.EditDeployKindOptions, dir string, expectedKind string, expectedCanary bool, expectedHPA bool) error {
	_, testName := filepath.Split(dir)
	// lets parse the arguments so that we don't prompt for them on the console
	err := eo.Cmd.Flags().Parse(edit.ToDeployArguments(opts.OptionKind, expectedKind, expectedCanary, expectedHPA))
	if err != nil {
		return err
	}

	assert.Equal(t, expectedKind, eo.Kind, "parse argument: kind for test %s", testName)
	assert.Equal(t, expectedCanary, eo.DeployOptions.Canary, "parse argument: deployOptions.Canary for test %s", testName)
	assert.Equal(t, expectedHPA, eo.DeployOptions.HPA, "parse argument: deployOptions.HPA for test %s", testName)

	err = eo.Run()
	if err != nil {
		return err
	}

	// lets assert that the options are
	yamlFile := filepath.Join(dir, "charts", "testapp", "values.yaml")
	data, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		return errors.Wrapf(err, "failed to load file %s", yamlFile)
	}
	kind, deployOptions := eo.FindDefaultDeployKindInValuesYaml(string(data))
	assert.Equal(t, expectedKind, kind, "kind for test %s", testName)
	assert.Equal(t, expectedCanary, deployOptions.Canary, "deployOptions.Canary for test %s", testName)
	assert.Equal(t, expectedHPA, deployOptions.HPA, "deployOptions.HPA for test %s", testName)
	return nil
}

func assertTeamEditDeploy(t *testing.T, eo *edit.EditDeployKindOptions, dir string, expectedKind string, expectedCanary bool, expectedHPA bool) error {
	_, testName := filepath.Split(dir)
	// lets parse the arguments so that we don't prompt for them on the console
	err := eo.Cmd.Flags().Parse([]string{"--team", "--" + opts.OptionKind + "=" + expectedKind, "--" + opts.OptionCanary + "=" + toString(expectedCanary), "--" + opts.OptionHPA + "=" + toString(expectedHPA)})
	if err != nil {
		return err
	}

	assert.Equal(t, expectedKind, eo.Kind, "parse argument: kind for test %s", testName)
	assert.Equal(t, expectedCanary, eo.DeployOptions.Canary, "parse argument: deployOptions.Canary for test %s", testName)
	assert.Equal(t, expectedHPA, eo.DeployOptions.HPA, "parse argument: deployOptions.HPA for test %s", testName)

	err = eo.Run()
	if err != nil {
		return err
	}

	// verify we have not changed the local values.yaml
	localYamlFile := filepath.Join(dir, "charts", "testapp", "values.yaml")
	sourceFile := filepath.Join("test_data", "edit_deploy", "testapp", "charts", "testapp", "values.yaml")

	tests.AssertTextFileContentsEqual(t, sourceFile, localYamlFile)

	teamSettings, err := eo.TeamSettings()
	if err != nil {
		return err
	}
	assert.Equal(t, expectedKind, teamSettings.DeployKind, "teamSettings.DeployKind for test %s", testName)
	teamDeploySettings := teamSettings.DeployOptions
	if !expectedCanary && !expectedHPA {
		assert.Nil(t, teamDeploySettings, "DeployOptions should be empty")
		teamDeploySettings = &v1.DeployOptions{}
	} else {
		assert.Equal(t, expectedCanary, teamDeploySettings.Canary, "teamSettings.DeployOptions.Canary for test %s", testName)
		assert.Equal(t, expectedHPA, teamDeploySettings.HPA, "teamSettings.DeployOptions..HPA for test %s", testName)
	}
	t.Logf("test %s has team settings of deploy kind: %s canary: %v hpa: %v", testName, teamSettings.DeployKind, teamDeploySettings.Canary, teamDeploySettings.HPA)
	return nil
}

func toString(flag bool) string {
	if flag {
		return "true"
	}
	return "false"
}
