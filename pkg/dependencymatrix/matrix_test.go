// +build unit

package dependencymatrix_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/tests"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/dependencymatrix"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/stretchr/testify/assert"
)

func TestUpdateSimpleDependencyMatrix(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	matrixDir := filepath.Join(dir, dependencymatrix.DependencyMatrixDirName)
	err = os.MkdirAll(matrixDir, 0700)
	assert.NoError(t, err)
	matrixYamlPath := filepath.Join(matrixDir, "matrix.yaml")
	err = util.CopyFile(filepath.Join("testdata", "simple_matrix", "dependency-matrix", "matrix.yaml"), matrixYamlPath)
	assert.NoError(t, err)
	owner := "acme"
	repo := "roadrunner"
	component := "cheese"
	toVersion := "0.0.2"
	fromVersion := "0.0.1"
	toTag := fmt.Sprintf("v%s", toVersion)
	fromTag := fmt.Sprintf("v%s", fromVersion)
	host := "fake.git"
	update := v1.DependencyUpdate{
		DependencyUpdateDetails: v1.DependencyUpdateDetails{
			Host:               host,
			Owner:              owner,
			Repo:               repo,
			Component:          component,
			URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo),
			ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, toTag),
			ToVersion:          toVersion,
			ToReleaseName:      toVersion,
			FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, fromTag),
			FromReleaseName:    fromVersion,
			FromVersion:        fromVersion,
		},
		Paths: make([]v1.DependencyUpdatePath, 0),
	}
	err = dependencymatrix.UpdateDependencyMatrix(dir, &update)
	assert.NoError(t, err)
	tests.AssertTextFileContentsEqual(t, filepath.Join("testdata", "simple_matrix", "matrix.golden.yaml"), matrixYamlPath)
}

func TestUpdateOneDegreeDependencyMatrix(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	matrixDir := filepath.Join(dir, dependencymatrix.DependencyMatrixDirName)
	err = os.MkdirAll(matrixDir, 0700)
	assert.NoError(t, err)
	matrixYamlPath := filepath.Join(matrixDir, "matrix.yaml")
	err = util.CopyFile(filepath.Join("testdata", "one_degree_matrix", "dependency-matrix", "matrix.yaml"), matrixYamlPath)
	assert.NoError(t, err)
	owner := "acme"
	repo := "roadrunner"
	viaRepo := "wiley"
	toVersion := "0.0.2"
	fromVersion := "0.0.1"
	toTag := fmt.Sprintf("v%s", toVersion)
	fromTag := fmt.Sprintf("v%s", fromVersion)
	host := "fake.git"
	update := v1.DependencyUpdate{
		DependencyUpdateDetails: v1.DependencyUpdateDetails{
			Host:               host,
			Owner:              owner,
			Repo:               repo,
			URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo),
			ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, toTag),
			ToVersion:          toVersion,
			ToReleaseName:      toVersion,
			FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, fromTag),
			FromReleaseName:    fromVersion,
			FromVersion:        fromVersion,
		},
		Paths: []v1.DependencyUpdatePath{
			{
				{
					Host:               host,
					Owner:              owner,
					Repo:               viaRepo,
					URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, viaRepo),
					ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, viaRepo, toTag),
					ToVersion:          toVersion,
					ToReleaseName:      toVersion,
					FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, viaRepo, fromTag),
					FromReleaseName:    fromVersion,
					FromVersion:        fromVersion,
				},
			},
		},
	}
	err = dependencymatrix.UpdateDependencyMatrix(dir, &update)
	assert.NoError(t, err)
	tests.AssertTextFileContentsEqual(t, filepath.Join("testdata", "one_degree_matrix", "matrix.golden.yaml"), matrixYamlPath)
}

func TestUpdateTwoPathsDependencyMatrix(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	matrixDir := filepath.Join(dir, dependencymatrix.DependencyMatrixDirName)
	err = os.MkdirAll(matrixDir, 0700)
	assert.NoError(t, err)
	matrixYamlPath := filepath.Join(matrixDir, "matrix.yaml")
	err = util.CopyFile(filepath.Join("testdata", "two_paths_matrix", "dependency-matrix", "matrix.yaml"), matrixYamlPath)
	assert.NoError(t, err)
	owner := "acme"
	repo := "roadrunner"
	viaRepo := "wiley"
	toVersion := "0.0.2"
	fromVersion := "0.0.1"
	toTag := fmt.Sprintf("v%s", toVersion)
	fromTag := fmt.Sprintf("v%s", fromVersion)
	host := "fake.git"
	update := v1.DependencyUpdate{
		DependencyUpdateDetails: v1.DependencyUpdateDetails{
			Host:               host,
			Owner:              owner,
			Repo:               repo,
			URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo),
			ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, toTag),
			ToVersion:          toVersion,
			ToReleaseName:      toVersion,
			FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, fromTag),
			FromReleaseName:    fromVersion,
			FromVersion:        fromVersion,
		},
		Paths: []v1.DependencyUpdatePath{
			{
				{
					Host:               host,
					Owner:              owner,
					Repo:               viaRepo,
					URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, viaRepo),
					ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, viaRepo, toTag),
					ToVersion:          toVersion,
					ToReleaseName:      toVersion,
					FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, viaRepo, fromTag),
					FromReleaseName:    fromVersion,
					FromVersion:        fromVersion,
				},
			},
		},
	}
	err = dependencymatrix.UpdateDependencyMatrix(dir, &update)
	assert.NoError(t, err)
	tests.AssertTextFileContentsEqual(t, filepath.Join("testdata", "two_paths_matrix", "matrix.golden.yaml"), matrixYamlPath)
}

func TestUpdateTwoDegreeDependencyMatrix(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	matrixDir := filepath.Join(dir, dependencymatrix.DependencyMatrixDirName)
	err = os.MkdirAll(matrixDir, 0700)
	assert.NoError(t, err)
	matrixYamlPath := filepath.Join(matrixDir, "matrix.yaml")
	err = util.CopyFile(filepath.Join("testdata", "two_degree_matrix", "dependency-matrix", "matrix.yaml"), matrixYamlPath)
	assert.NoError(t, err)
	owner := "acme"
	repo := "roadrunner"
	viaRepo := "wiley"
	via2Repo := "coyote"
	toVersion := "0.0.2"
	fromVersion := "0.0.1"
	toTag := fmt.Sprintf("v%s", toVersion)
	fromTag := fmt.Sprintf("v%s", fromVersion)
	host := "fake.git"
	update := v1.DependencyUpdate{
		DependencyUpdateDetails: v1.DependencyUpdateDetails{
			Host:               host,
			Owner:              owner,
			Repo:               repo,
			URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo),
			ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, toTag),
			ToVersion:          toVersion,
			ToReleaseName:      toVersion,
			FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, fromTag),
			FromReleaseName:    fromVersion,
			FromVersion:        fromVersion,
		},
		Paths: []v1.DependencyUpdatePath{
			{
				{
					Host:               host,
					Owner:              owner,
					Repo:               viaRepo,
					URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, viaRepo),
					ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, viaRepo, toTag),
					ToVersion:          toVersion,
					ToReleaseName:      toVersion,
					FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, viaRepo, fromTag),
					FromReleaseName:    fromVersion,
					FromVersion:        fromVersion,
				},
				{
					Host:               host,
					Owner:              owner,
					Repo:               via2Repo,
					URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, via2Repo),
					ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, via2Repo, toTag),
					ToVersion:          toVersion,
					ToReleaseName:      toVersion,
					FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, via2Repo, fromTag),
					FromReleaseName:    fromVersion,
					FromVersion:        fromVersion,
				},
			},
		},
	}
	err = dependencymatrix.UpdateDependencyMatrix(dir, &update)
	assert.NoError(t, err)
	tests.AssertTextFileContentsEqual(t, filepath.Join("testdata", "two_degree_matrix", "matrix.golden.yaml"), matrixYamlPath)
}

func TestCreateDependencyMatrix(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	matrixDir := filepath.Join(dir, dependencymatrix.DependencyMatrixDirName)
	matrixYamlPath := filepath.Join(matrixDir, "matrix.yaml")
	owner := "acme"
	repo := "roadrunner"
	component := "cheese"
	toVersion := "0.0.2"
	fromVersion := "0.0.1"
	toTag := fmt.Sprintf("v%s", toVersion)
	fromTag := fmt.Sprintf("v%s", fromVersion)
	host := "fake.git"
	update := v1.DependencyUpdate{
		DependencyUpdateDetails: v1.DependencyUpdateDetails{
			Host:               host,
			Owner:              owner,
			Repo:               repo,
			Component:          component,
			URL:                fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo),
			ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, toTag),
			ToVersion:          toVersion,
			ToReleaseName:      toVersion,
			FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, owner, repo, fromTag),
			FromReleaseName:    fromVersion,
			FromVersion:        fromVersion,
		},
		Paths: make([]v1.DependencyUpdatePath, 0),
	}
	err = dependencymatrix.UpdateDependencyMatrix(dir, &update)
	assert.NoError(t, err)
	tests.AssertTextFileContentsEqual(t, filepath.Join("testdata", "new_matrix", "matrix.golden.yaml"), matrixYamlPath)
}

func TestFindVersionForDependency(t *testing.T) {
	dir := filepath.Join("testdata", "two_degree_matrix")

	matrix, err := dependencymatrix.LoadDependencyMatrix(dir)
	assert.NoError(t, err)

	version, err := matrix.FindVersionForDependency("fake.git", "acme", "roadrunner")
	assert.NoError(t, err)
	assert.Equal(t, "0.0.1", version)

	_, err = matrix.FindVersionForDependency("doesnotexist", "doesnotexist", "doesnotexist")
	assert.NotNil(t, err)
	assert.Equal(t, "could not find a dependency on host doesnotexist, owner doesnotexist, repo doesnotexist in the dependency matrix", err.Error())
}
