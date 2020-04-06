// +build integration

package boot

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/clients"

	cmd_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/stretchr/testify/require"

	"github.com/acarl005/stripansi"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

const (
	FirstTag  = "v1.0.0"
	SecondTag = "v2.0.0"
	ThirdTag  = "v3.0.0"

	testFileName = "some-file"
)

func Test_createBootClone_into_empty_directory_succeeds(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	factory := cmd_mocks.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(factory)
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
	}

	repoPath := filepath.Join(bootDir, "jenkins-x-boot-config")
	cloneDir, err := o.createBootClone(config.DefaultBootRepository, config.DefaultVersionsRef, repoPath)
	assert.NoError(t, err)
	assert.Contains(t, cloneDir, repoPath)
}

func Test_createBootClone_with_custom_ref_succeeds(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	factory := cmd_mocks.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(factory)
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
	}

	repoPath := filepath.Join(bootDir, "jenkins-x-boot-config")
	cloneDir, err := o.createBootClone(config.DefaultBootRepository, "v1.0.70", repoPath)
	assert.NoError(t, err)
	assert.Contains(t, cloneDir, repoPath)

	gitter := gits.NewGitCLI()
	sha, err := gitter.GetLatestCommitSha(repoPath)
	require.NoError(t, err)
	assert.Equal(t, "424f97a9e1ad4a0b5e518cd492d2b16d8b8f6705", sha)
}

func Test_createBootClone_into_existing_git_repo_fails(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	factory := cmd_mocks.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(factory)
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
	}

	repoPath := filepath.Join(bootDir, "my-repo")
	err = os.MkdirAll(repoPath, 0700)
	require.NoError(t, err)
	gitter := gits.NewGitCLI()
	err = gitter.Init(repoPath)
	require.NoError(t, err)

	cloneDir, err := o.createBootClone(config.DefaultBootRepository, config.DefaultVersionsRef, repoPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dir already exists")
	assert.Empty(t, cloneDir)
}

func Test_createBootClone_into_existing_directory_fails(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	factory := cmd_mocks.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(factory)
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
	}

	cloneDir, err := o.createBootClone(config.DefaultBootRepository, config.DefaultVersionsRef, bootDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dir already exists")
	assert.Empty(t, cloneDir)
}

func Test_jx_boot_in_non_boot_repo_fails(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	commonOpts := opts.NewCommonOptionsWithFactory(clients.NewFactory())
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
	}

	// make the tmp directory a git repo
	gitter := gits.NewGitCLI()
	err = gitter.Init(bootDir)
	require.NoError(t, err)
	err = gitter.SetRemoteURL(bootDir, "origin", "https://github.com/johndoe/jx.git")
	require.NoError(t, err)
	_, err = os.Create(filepath.Join(bootDir, "foo"))
	require.NoError(t, err)
	err = gitter.AddCommit(bootDir, "adding foo")
	require.NoError(t, err)

	err = o.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trying to execute 'jx boot' from a non requirements repo")
}

func TestUpdateBootCloneIfOutOfDate_Conflicts(t *testing.T) {
	gitter := gits.NewGitCLI()

	repoDir, err := initializeTempGitRepo(gitter)
	assert.NoError(t, err)

	defer func() {
		err := os.RemoveAll(repoDir)
		assert.NoError(t, err)
	}()

	testDir, err := ioutil.TempDir("", "update-local-boot-clone-test-clone-")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(testDir)
		assert.NoError(t, err)
	}()

	err = gitter.Clone(repoDir, testDir)
	assert.NoError(t, err)

	err = gitter.FetchTags(testDir)
	assert.NoError(t, err)

	err = gitter.Reset(testDir, FirstTag, true)
	assert.NoError(t, err)

	conflictingContent := []byte("something else")
	err = ioutil.WriteFile(filepath.Join(testDir, testFileName), conflictingContent, util.DefaultWritePermissions)
	assert.NoError(t, err)

	o := &BootOptions{
		CommonOptions: &opts.CommonOptions{},
		Dir:           testDir,
	}
	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	err = o.updateBootCloneIfOutOfDate(SecondTag)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Could not update local boot clone due to conflicts between local changes and v2.0.0.")

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.Contains(t, output, "It is based on v1.0.0, but the version stream is using v2.0.0.")
}

func TestUpdateBootCloneIfOutOfDate_NoConflicts(t *testing.T) {
	gitter := gits.NewGitCLI()

	repoDir, err := initializeTempGitRepo(gitter)
	assert.NoError(t, err)

	defer func() {
		err := os.RemoveAll(repoDir)
		assert.NoError(t, err)
	}()

	testDir, err := ioutil.TempDir("", "update-local-boot-clone-test-clone-")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(testDir)
		assert.NoError(t, err)
	}()

	err = gitter.Clone(repoDir, testDir)
	assert.NoError(t, err)

	err = gitter.FetchTags(testDir)
	assert.NoError(t, err)

	err = gitter.Reset(testDir, FirstTag, true)
	assert.NoError(t, err)

	conflictingContent := []byte("something else")
	err = ioutil.WriteFile(filepath.Join(testDir, "some-other-file"), conflictingContent, util.DefaultWritePermissions)
	assert.NoError(t, err)

	o := &BootOptions{
		CommonOptions: &opts.CommonOptions{},
		Dir:           testDir,
	}
	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	err = o.updateBootCloneIfOutOfDate(SecondTag)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.Contains(t, output, "It is based on v1.0.0, but the version stream is using v2.0.0.")
}

func TestUpdateBootCloneIfOutOfDate_UpToDate(t *testing.T) {
	gitter := gits.NewGitCLI()

	repoDir, err := initializeTempGitRepo(gitter)
	assert.NoError(t, err)

	defer func() {
		err := os.RemoveAll(repoDir)
		assert.NoError(t, err)
	}()

	testDir, err := ioutil.TempDir("", "update-local-boot-clone-test-clone-")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(testDir)
		assert.NoError(t, err)
	}()

	err = gitter.Clone(repoDir, testDir)
	assert.NoError(t, err)

	err = gitter.FetchTags(testDir)
	assert.NoError(t, err)

	err = gitter.Reset(testDir, SecondTag, true)
	assert.NoError(t, err)

	conflictingContent := []byte("something else")
	err = ioutil.WriteFile(filepath.Join(testDir, "some-other-file"), conflictingContent, util.DefaultWritePermissions)
	assert.NoError(t, err)

	o := &BootOptions{
		CommonOptions: &opts.CommonOptions{},
		Dir:           testDir,
	}
	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	err = o.updateBootCloneIfOutOfDate(SecondTag)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.Equal(t, "", output)
}

func TestUpdateBootCloneIfOutOfDate_NotAncestor(t *testing.T) {
	gitter := gits.NewGitCLI()

	repoDir, err := initializeTempGitRepo(gitter)
	assert.NoError(t, err)

	defer func() {
		err := os.RemoveAll(repoDir)
		assert.NoError(t, err)
	}()

	testDir, err := ioutil.TempDir("", "update-local-boot-clone-test-clone-")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(testDir)
		assert.NoError(t, err)
	}()

	err = gitter.Clone(repoDir, testDir)
	assert.NoError(t, err)

	err = gitter.FetchTags(testDir)
	assert.NoError(t, err)

	err = gitter.Reset(testDir, ThirdTag, true)
	assert.NoError(t, err)

	o := &BootOptions{
		CommonOptions: &opts.CommonOptions{},
		Dir:           testDir,
	}
	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	err = o.updateBootCloneIfOutOfDate(SecondTag)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.Contains(t, output, "Current HEAD v3.0.0 in")
}

func initializeTempGitRepo(gitter gits.Gitter) (string, error) {
	dir, err := ioutil.TempDir("", "update-local-boot-clone-test-repo-")
	if err != nil {
		return "", err
	}

	err = gitter.Init(dir)
	if err != nil {
		return "", err
	}

	err = gitter.AddCommit(dir, "Initial Commit")
	if err != nil {
		return "", err
	}

	testFile := filepath.Join(dir, testFileName)
	testFileContents := []byte("foo")
	err = ioutil.WriteFile(testFile, testFileContents, util.DefaultWritePermissions)
	if err != nil {
		return "", err
	}

	err = gitter.Add(dir, ".")
	if err != nil {
		return "", err
	}
	err = gitter.AddCommit(dir, "Adding foo")
	if err != nil {
		return "", err
	}

	err = gitter.CreateTag(dir, FirstTag, "First Tag")
	if err != nil {
		return "", err
	}

	testFileContents = []byte("bar")
	err = ioutil.WriteFile(testFile, testFileContents, util.DefaultWritePermissions)
	if err != nil {
		return "", err
	}

	err = gitter.AddCommit(dir, "Adding bar")
	if err != nil {
		return "", err
	}

	err = gitter.CreateTag(dir, SecondTag, "Second Tag")
	if err != nil {
		return "", err
	}

	testFileContents = []byte("baz")
	err = ioutil.WriteFile(testFile, testFileContents, util.DefaultWritePermissions)
	if err != nil {
		return "", err
	}

	err = gitter.AddCommit(dir, "Adding baz")
	if err != nil {
		return "", err
	}

	err = gitter.CreateTag(dir, ThirdTag, "Third Tag")
	if err != nil {
		return "", err
	}

	return dir, nil
}
