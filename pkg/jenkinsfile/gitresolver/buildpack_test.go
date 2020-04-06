// +build unit

package gitresolver

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/gits/testhelpers"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestBuildPackInitClone(t *testing.T) {
	mainRepo, err := ioutil.TempDir("", uuid.New().String())
	assert.NoError(t, err)

	remoteRepo, err := ioutil.TempDir("", uuid.New().String())
	assert.NoError(t, err)

	defer func() {
		err := os.RemoveAll(mainRepo)
		err2 := os.RemoveAll(remoteRepo)
		if err != nil || err2 != nil {
			log.Logger().Errorf("Error cleaning up tmpdirs because %v", err)
		}
	}()

	err = os.Setenv("JX_HOME", mainRepo)
	assert.NoError(t, err)
	gitDir := mainRepo + "/draft/packs"
	err = os.MkdirAll(gitDir, 0755)
	assert.NoError(t, err)

	gitter := gits.NewGitCLI()
	assert.NoError(t, err)

	// Prepare a git repo to test - this is our "remote"
	err = gitter.Init(remoteRepo)
	assert.NoError(t, err)

	readme := "README"
	initialReadme := "Cheesy!"

	readmePath := filepath.Join(remoteRepo, readme)
	err = ioutil.WriteFile(readmePath, []byte(initialReadme), 0600)
	assert.NoError(t, err)
	err = gitter.Add(remoteRepo, readme)
	assert.NoError(t, err)
	err = gitter.CommitDir(remoteRepo, "Initial Commit")
	assert.NoError(t, err)

	// Prepare another git repo, this is local repo
	err = gitter.Init(gitDir)
	assert.NoError(t, err)
	// Set up the remote
	err = gitter.AddRemote(gitDir, "origin", remoteRepo)
	assert.NoError(t, err)
	err = gitter.FetchBranch(gitDir, "origin", "master")
	assert.NoError(t, err)
	err = gitter.Merge(gitDir, "origin/master")
	assert.NoError(t, err)

	// Removing the remote tracking information, after executing InitBuildPack, it should have not failed and it should've set a remote tracking branch
	testhelpers.GitCmd(func(s string, i ...int) {}, gitDir, "branch", "--unset-upstream")

	_, err = InitBuildPack(gitter, "", "master")

	testhelpers.GitCmd(func(s string, i ...int) {}, gitDir, "status", "-sb")

	args := []string{"status", "-sb"}
	cmd := util.Command{
		Dir:  gitDir,
		Name: "git",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	assert.NoError(t, err, "it should not fail to pull from the branch as remote tracking information has been set")

	// Check the current branch is tracking the origin/master one
	assert.Equal(t, "## master...origin/master", output)
}
