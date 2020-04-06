// +build integration

package create_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/importcmd"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/quickstarts"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
)

func TestCreateMLQuickstartProjects(t *testing.T) {

	// TODO lets skip this test for now as it often fails with rate limits
	t.SkipNow()

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

	testDir, err := ioutil.TempDir("", "test-create-mlquickstart")
	assert.NoError(t, err)

	appName := "mymlapp"

	o := &create.CreateMLQuickstartOptions{
		CreateProjectOptions: create.CreateProjectOptions{
			ImportOptions: importcmd.ImportOptions{
				CommonOptions: &opts.CommonOptions{},
			},
		},
		GitHubOrganisations: []string{"machine-learning-quickstarts"},
		Filter: quickstarts.QuickstartFilter{
			Text:        "machine-learning-quickstarts/ML-python-pytorch-cpu",
			ProjectName: appName,
		},
	}
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits.NewGitCLI(), helm.NewHelmCLI("helm", helm.V2, testDir, true))
	o.Dir = testDir
	o.OutDir = testDir
	o.DryRun = true
	o.DisableMaven = true
	o.Verbose = true
	o.IgnoreTeam = true
	o.Repository = appName

	err = o.Run()
	assert.NoError(t, err)
	if err == nil {
		appName1 := appName + "-service"
		appDir1 := filepath.Join(testDir, appName1)
		jenkinsfile := filepath.Join(appDir1, "Jenkinsfile")
		tests.AssertFileExists(t, jenkinsfile)
		tests.AssertFileExists(t, filepath.Join(appDir1, "Dockerfile"))
		tests.AssertFileExists(t, filepath.Join(appDir1, "charts", appName1, "Chart.yaml"))
		tests.AssertFileExists(t, filepath.Join(appDir1, "charts", appName1, "Makefile"))
		tests.AssertFileDoesNotExist(t, filepath.Join(appDir1, "charts", appName, "Chart.yaml"))

		appName2 := appName + "-training"
		appDir2 := filepath.Join(testDir, appName2)
		jenkinsfile = filepath.Join(appDir2, "Jenkinsfile")
		tests.AssertFileExists(t, jenkinsfile)
		tests.AssertFileExists(t, filepath.Join(appDir2, "Dockerfile"))
		tests.AssertFileExists(t, filepath.Join(appDir2, "charts", appName2, "Chart.yaml"))
		tests.AssertFileExists(t, filepath.Join(appDir2, "charts", appName2, "Makefile"))
		tests.AssertFileDoesNotExist(t, filepath.Join(appDir2, "charts", appName, "Chart.yaml"))
	}
}
