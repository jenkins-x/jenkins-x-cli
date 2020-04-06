// +build integration

package versionstreamrepo_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream/versionstreamrepo"
	"github.com/stretchr/testify/assert"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
)

const (
	TagFromDefaultURL = "v1.0.114"
	FirstTag          = "v0.0.1"
	SecondTag         = "v0.0.2"
	BranchRef         = "master"
)

func TestCloneJXVersionsRepoWithDefaultURL(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	_, _ = assertClonesCorrectly(t, "", TagFromDefaultURL, TagFromDefaultURL, gitter, nil)
}

func initializeTempGitRepo(gitter gits.Gitter) (string, string, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", "", err
	}

	err = gitter.Init(dir)
	if err != nil {
		return "", "", err
	}

	err = gitter.AddCommit(dir, "Initial Commit")
	if err != nil {
		return "", "", err
	}

	testFile, err := ioutil.TempFile(dir, "versionstreams-test-")
	if err != nil {
		return "", "", err
	}

	testFileContents := []byte("foo")
	_, err = testFile.Write(testFileContents)
	if err != nil {
		return "", "", err
	}

	err = gitter.Add(dir, ".")
	if err != nil {
		return "", "", err
	}
	err = gitter.AddCommit(dir, "Adding foo")
	if err != nil {
		return "", "", err
	}

	err = gitter.CreateTag(dir, FirstTag, "First Tag")
	if err != nil {
		return "", "", err
	}

	testFileContents = []byte("bar")
	_, err = testFile.Write(testFileContents)
	if err != nil {
		return "", "", err
	}

	err = gitter.AddCommit(dir, "Adding bar")
	if err != nil {
		return "", "", err
	}

	err = gitter.CreateTag(dir, SecondTag, "Second Tag")
	if err != nil {
		return "", "", err
	}

	testFileContents = []byte("baz")
	_, err = testFile.Write(testFileContents)
	if err != nil {
		return "", "", err
	}

	err = gitter.AddCommit(dir, "Adding baz")
	if err != nil {
		return "", "", err
	}

	return fmt.Sprint(dir), filepath.Base(testFile.Name()), nil
}

func TestCloneJXVersionsRepoWithOverriddenDefault(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	originalDefaultVersionsURL := config.DefaultVersionsURL
	originalDefaultVersionsRef := config.DefaultVersionsRef

	config.DefaultVersionsURL = gitDir
	config.DefaultVersionsRef = FirstTag

	defer func() {
		config.DefaultVersionsRef = originalDefaultVersionsRef
		config.DefaultVersionsURL = originalDefaultVersionsURL
	}()

	_ = assertClonesCorrectlyWithCorrectFileContents(t, "", "", FirstTag, gitter, testFileName, "foo", nil)
}

func TestCloneJXVersionsRepoReplacingCurrent(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	// First, clone the default URL so we can make sure it gets removed.
	_, _ = assertClonesCorrectly(t, "", TagFromDefaultURL, TagFromDefaultURL, gitter, nil)

	// Sleep briefly so that git GC in the background has finished
	time.Sleep(5 * time.Second)

	// Next, switch to using the temp repo and make sure that replaces the current one.
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	settings := &v1.TeamSettings{
		VersionStreamURL: gitDir,
		VersionStreamRef: FirstTag,
	}

	_ = assertClonesCorrectlyWithCorrectFileContents(t, "", "", FirstTag, gitter, testFileName, "foo", settings)
}

func TestCloneJXVersionsRepoWithTeamSettings(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	settings := &v1.TeamSettings{
		VersionStreamURL: gitDir,
		VersionStreamRef: FirstTag,
	}

	_ = assertClonesCorrectlyWithCorrectFileContents(t, "", "", FirstTag, gitter, testFileName, "foo", settings)
}

func TestCloneJXVersionsRepoWithATag(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	_ = assertClonesCorrectlyWithCorrectFileContents(t, gitDir, FirstTag, FirstTag, gitter, testFileName, "foo", nil)
}

func TestCloneJXVersionsRepoWithABranch(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	_ = assertClonesCorrectlyWithCorrectFileContents(t, gitDir, BranchRef, BranchRef, gitter, testFileName, "foobarbaz", nil)
}

func TestCloneJXVersionsRepoWithACommit(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	headMinusOne, err := gitter.RevParse(gitDir, "HEAD~1")

	_ = assertClonesCorrectlyWithCorrectFileContents(t, gitDir, headMinusOne, SecondTag, gitter, testFileName, "foobar", nil)
}

func TestCloneJXVersionsRepoWithAnUntaggedCommit(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	head, err := gitter.RevParse(gitDir, "HEAD")

	_ = assertClonesCorrectlyWithCorrectFileContents(t, gitDir, head, head, gitter, testFileName, "foobarbaz", nil)
}

func TestCloneJXVersionsRepoWithNonFastForward(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	gitter := gits.NewGitCLI()
	gitDir, testFileName, err := initializeTempGitRepo(gitter)
	defer func() {
		err := os.RemoveAll(gitDir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	dir := assertClonesCorrectlyWithCorrectFileContents(t, gitDir, BranchRef, BranchRef, gitter, testFileName, "foobarbaz", nil)

	// Update the git repo with a new commit
	testFileContents := []byte("banana")
	err = ioutil.WriteFile(filepath.Join(gitDir, testFileName), testFileContents, util.DefaultWritePermissions)
	assert.NoError(t, err)

	err = gitter.AddCommit(gitDir, "changing to banana")
	assert.NoError(t, err)

	// Make a different change to the local clone of the repo
	testFileContents = []byte("apple")
	err = ioutil.WriteFile(filepath.Join(dir, testFileName), testFileContents, util.DefaultWritePermissions)
	assert.NoError(t, err)

	err = gitter.AddCommit(dir, "changing to apple")
	assert.NoError(t, err)

	// Run CloneJXVersionsRepo and verify that it does checkout the latest of the branch.
	_ = assertClonesCorrectlyWithCorrectFileContents(t, gitDir, BranchRef, BranchRef, gitter, testFileName, "banana", nil)
}

func assertClonesCorrectlyWithCorrectFileContents(t *testing.T, gitDir string, versionRefToCheckout string, expectedRef string, gitter gits.Gitter, testFileName string, expectedFileContent string, settings *v1.TeamSettings) string {
	dir, actualVersionRef := assertClonesCorrectly(t, gitDir, versionRefToCheckout, expectedRef, gitter, settings)
	err := gitter.Checkout(dir, actualVersionRef)
	assert.NoError(t, err)

	actualFileContents, err := ioutil.ReadFile(filepath.Join(dir, testFileName))
	assert.NoError(t, err)
	assert.Equal(t, expectedFileContent, string(actualFileContents))

	return dir
}

func assertClonesCorrectly(t *testing.T, gitDir string, versionRefToCheckout string, expectedRef string, gitter gits.Gitter, settings *v1.TeamSettings) (string, string) {
	dir, actualVersionRef, err := versionstreamrepo.CloneJXVersionsRepo(
		gitDir,
		versionRefToCheckout,
		settings,
		gitter,
		true,
		false,
		util.IOFileHandles{},
	)
	assert.NoError(t, err)
	assert.NotNil(t, dir)
	assert.NotNil(t, actualVersionRef)
	assert.Equal(t, expectedRef, actualVersionRef)

	return dir, actualVersionRef
}
