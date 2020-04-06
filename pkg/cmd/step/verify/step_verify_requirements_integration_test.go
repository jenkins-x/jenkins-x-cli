// +build integration

package verify_test

import (
	"path"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/step/verify"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/require"

	"io/ioutil"
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/stretchr/testify/assert"
)

func TestStepVerifyRequirements(t *testing.T) {
	t.Parallel()

	tempDir, err := ioutil.TempDir("", "test-step-verify-requirements")
	require.NoError(t, err)

	testData := path.Join("test_data", "verify_requirements", "boot-config")
	_, err = os.Stat(testData)
	require.NoError(t, err)

	err = util.CopyDir(testData, tempDir, true)
	require.NoError(t, err)

	options := verify.StepVerifyRequirementsOptions{
		Dir: tempDir,
	}

	// fake the output stream to be checked later
	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	commonOpts.Out = os.Stdout
	commonOpts.Err = os.Stderr
	options.CommonOptions = &commonOpts

	testhelpers.ConfigureTestOptions(options.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())
	err = options.Run()
	assert.NoError(t, err, "Command failed: %#v", options)

	// lets assert we have requirements populated nicely

	reqFile := filepath.Join(tempDir, "env", helm.RequirementsFileName)
	assert.FileExists(t, reqFile)

	req, err := helm.LoadRequirementsFile(reqFile)
	require.NoError(t, err, "failed to load file %s", reqFile)

	dep := assertHasDependency(t, req, "jenkins-x-platform")
	if dep != nil {
		t.Logf("found version %s for jenkins-x-platform requirement", dep.Version)
		assert.True(t, dep.Version != "", "missing version of jenkins-x-platform dependency")
	}
}

func assertHasDependency(t *testing.T, requirements *helm.Requirements, dependencyName string) *helm.Dependency {
	for _, dep := range requirements.Dependencies {
		if dep.Name == dependencyName {
			return dep
		}
	}
	assert.Fail(t, "could not find dependency %s in requirements file")
	return nil
}
