// +build integration

package importcmd_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/edit"
	"github.com/jenkins-x/jx/pkg/cmd/importcmd"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestImportProjectNextGenPipelineWithDeploy(t *testing.T) {
	t.Parallel()
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

	tmpDir, err := ioutil.TempDir("", "test-import-deploy-projects-")
	assert.NoError(t, err)
	require.DirExists(t, tmpDir, "could not create temp dir for running tests")

	srcDir := path.Join("test_data", "import_projects", "nodejs")
	assert.DirExists(t, srcDir, "missing source data")

	type testData struct {
		name     string
		callback func(t *testing.T, io *importcmd.ImportOptions, dir string) error
		fail     bool
	}

	tests := []testData{
		{
			name: "team-enable-knative-canary-and-hpa",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployTeamSettings(t, io, dir, opts.DeployKindKnative, true, true)
			},
		},
		{
			name: "team-enable-canary-and-hpa",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployTeamSettings(t, io, dir, opts.DeployKindDefault, true, true)
			},
		},
		{
			name: "team-enable-canary",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployTeamSettings(t, io, dir, opts.DeployKindDefault, true, false)
			},
		},
		{
			name: "team-disable-knative-canary-and-hpa",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployTeamSettings(t, io, dir, opts.DeployKindDefault, false, false)
			},
		},
		{
			name: "enable-knative-canary-and-hpa",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployCLISettings(t, io, dir, opts.DeployKindKnative, true, true)
			},
		},
		{
			name: "enable-canary-and-hpa",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployCLISettings(t, io, dir, opts.DeployKindDefault, true, true)
			},
		},
		{
			name: "enable-canary",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployCLISettings(t, io, dir, opts.DeployKindDefault, true, false)
			},
		},
		{
			name: "disable-knative-canary-and-hpa",
			callback: func(t *testing.T, io *importcmd.ImportOptions, dir string) error {
				return assertImportWithDeployCLISettings(t, io, dir, opts.DeployKindDefault, false, false)
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

		_, io := importcmd.NewCmdImportAndOptions(&commonOpts)
		io.Dir = dir

		err = tt.callback(t, io, dir)
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

func assertImportWithDeployCLISettings(t *testing.T, io *importcmd.ImportOptions, dir string, expectedKind string, expectedCanary bool, expectedHPA bool) error {
	// lets force the CLI arguments to be parsed first to ensure the flags are set to avoid inheriting them from the TeamSettings
	err := io.Cmd.Flags().Parse(edit.ToDeployArguments("deploy-kind", expectedKind, expectedCanary, expectedHPA))
	if err != nil {
		return err
	}

	// lets check we parsed the CLI arguments correctly
	_, testName := filepath.Split(dir)
	assert.Equal(t, expectedKind, io.DeployKind, "parse argument: deployKind for test %s", testName)
	assert.Equal(t, expectedCanary, io.DeployOptions.Canary, "parse argument: deployOptions.Canary for test %s", testName)
	assert.Equal(t, expectedHPA, io.DeployOptions.HPA, "parse argument: deployOptions.HPA for test %s", testName)

	io.DeployKind = expectedKind
	io.DeployOptions = v1.DeployOptions{
		Canary: expectedCanary,
		HPA:    expectedHPA,
	}
	return assertImportHasDeploy(t, io, dir, expectedKind, expectedCanary, expectedHPA)
}

func assertImportWithDeployTeamSettings(t *testing.T, io *importcmd.ImportOptions, dir string, expectedKind string, expectedCanary bool, expectedHPA bool) error {
	err := io.ModifyDevEnvironment(func(env *v1.Environment) error {
		settings := &env.Spec.TeamSettings
		settings.DeployKind = expectedKind
		if !expectedCanary && !expectedHPA {
			settings.DeployOptions = nil
		} else {
			settings.DeployOptions = &v1.DeployOptions{
				Canary: expectedCanary,
				HPA:    expectedHPA,
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to modify team settings")
	}
	return assertImportHasDeploy(t, io, dir, expectedKind, expectedCanary, expectedHPA)
}

func assertImportHasDeploy(t *testing.T, o *importcmd.ImportOptions, testDir string, expectedKind string, expectedCanary bool, expectedHPA bool) error {
	_, testName := filepath.Split(testDir)
	testName = naming.ToValidName(testName)

	o.GitProvider = createFakeGitProvider()
	if o.Out == nil {
		o.Out = tests.Output()
	}
	if o.Out == nil {
		o.Out = os.Stdout
	}
	o.DryRun = true
	o.UseDefaultGit = true

	err := o.Run()
	assert.NoError(t, err, "Failed %s with %s", testName, err)
	if err == nil {
		valuesFile := filepath.Join(testDir, "charts", testName, "values.yaml")
		tests.AssertFileExists(t, filepath.Join(testDir, "charts", testName, "Chart.yaml"))
		tests.AssertFileExists(t, valuesFile)
		t.Logf("completed test in dir %s", testDir)

		// lets validate the resulting values.yaml
		yamlData, err := ioutil.ReadFile(valuesFile)
		assert.NoError(t, err, "Failed to load file %s", valuesFile)

		eo := edit.EditDeployKindOptions{}
		eo.CommonOptions = o.CommonOptions
		kind, dopts := eo.FindDefaultDeployKindInValuesYaml(string(yamlData))

		assert.Equal(t, expectedKind, kind, "kind for test %s", testName)
		assert.Equal(t, expectedCanary, dopts.Canary, "deployOptions.Canary for test %s", testName)
		assert.Equal(t, expectedHPA, dopts.HPA, "deployOptions.HPA for test %s", testName)
	}
	return err
}
