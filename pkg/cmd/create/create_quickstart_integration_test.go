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

func TestCreateQuickstartProjects(t *testing.T) {
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

	testDir, err := ioutil.TempDir("", "test-create-quickstart")
	assert.NoError(t, err)

	appName := "myvets"

	o := &create.CreateQuickstartOptions{
		CreateProjectOptions: create.CreateProjectOptions{
			ImportOptions: importcmd.ImportOptions{
				CommonOptions: &opts.CommonOptions{},
			},
		},
		GitHubOrganisations: []string{"petclinic-gcp"},
		Filter: quickstarts.QuickstartFilter{
			Text:        "petclinic-gcp/spring-petclinic-vets-service",
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
		appDir := filepath.Join(testDir, appName)
		jenkinsfile := filepath.Join(appDir, "Jenkinsfile")
		tests.AssertFileExists(t, jenkinsfile)
		tests.AssertFileExists(t, filepath.Join(appDir, "Dockerfile"))
		tests.AssertFileExists(t, filepath.Join(appDir, "charts", appName, "Chart.yaml"))
		tests.AssertFileExists(t, filepath.Join(appDir, "charts", appName, "Makefile"))
		tests.AssertFileDoesNotExist(t, filepath.Join(appDir, "charts", "spring-petclinic-vets-service", "Chart.yaml"))
	}
}
