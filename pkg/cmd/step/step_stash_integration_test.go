// +build integration

package step_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	step2 "github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/stretchr/testify/assert"
)

func TestStepStash(t *testing.T) {
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

	tempDir, err := ioutil.TempDir("", "test-step-collect")
	assert.NoError(t, err)

	testData := "test_data/step_collect/junit.xml"

	o := &step.StepStashOptions{
		StepOptions: step2.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	o.StorageLocation.Classifier = "tests"
	o.StorageLocation.BucketURL = "file://" + tempDir
	o.ToPath = "output"
	o.Pattern = []string{testData}
	o.ProjectGitURL = "https://github.com/jenkins-x/dummy-repo.git"
	o.ProjectBranch = "master"
	testhelpers.ConfigureTestOptions(o.CommonOptions, &gits.GitFake{}, helm_test.NewMockHelmer())

	err = o.Run()
	assert.NoError(t, err)

	generatedFile := filepath.Join(tempDir, o.ToPath, testData)
	assert.FileExists(t, generatedFile)

	tests.AssertTextFileContentsEqual(t, testData, generatedFile)
}

func TestStepStashBranchNameFromEnv(t *testing.T) {
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

	origBranchName := os.Getenv(util.EnvVarBranchName)
	_ = os.Setenv(util.EnvVarBranchName, "master")
	origBuildID := os.Getenv("BUILD_ID")
	_ = os.Unsetenv("BUILD_ID")
	origJxBuildNum := os.Getenv("JX_BUILD_NUMBER")
	_ = os.Unsetenv("JX_BUILD_NUMBER")
	origBuildNum := os.Getenv("BUILD_NUMBER")
	_ = os.Setenv("BUILD_NUMBER", "4")
	defer func() {
		if origBranchName != "" {
			_ = os.Setenv(util.EnvVarBranchName, origBranchName)
		} else {
			_ = os.Unsetenv(util.EnvVarBranchName)
		}
		if origBuildID != "" {
			_ = os.Setenv("BUILD_ID", origBuildID)
		} else {
			_ = os.Unsetenv("BUILD_ID")
		}
		if origBuildNum != "" {
			_ = os.Setenv("BUILD_NUMBER", origBuildNum)
		} else {
			_ = os.Unsetenv("BUILD_NUMBER")
		}
		if origJxBuildNum != "" {
			_ = os.Setenv("JX_BUILD_NUMBER", origJxBuildNum)
		} else {
			_ = os.Unsetenv("JX_BUILD_NUMBER")
		}
	}()

	tempDir, err := ioutil.TempDir("", "test-step-collect")
	assert.NoError(t, err)

	testData := "test_data/step_collect/junit.xml"

	o := &step.StepStashOptions{
		StepOptions: step2.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	o.StorageLocation.Classifier = "tests"
	o.StorageLocation.BucketURL = "file://" + tempDir
	o.StorageLocation.GitBranch = "gh-pages"
	o.Pattern = []string{testData}
	o.ProjectGitURL = "https://github.com/jenkins-x/dummy-repo.git"
	testhelpers.ConfigureTestOptions(o.CommonOptions, &gits.GitFake{}, helm_test.NewMockHelmer())

	err = o.Run()
	assert.NoError(t, err)

	outputPath := filepath.Join("jenkins-x", o.StorageLocation.Classifier, "jenkins-x", "dummy-repo", "master", "4")
	generatedFile := filepath.Join(tempDir, outputPath, testData)
	assert.FileExists(t, generatedFile)

	tests.AssertTextFileContentsEqual(t, testData, generatedFile)
}
