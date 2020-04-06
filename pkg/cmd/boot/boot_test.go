// +build unit

package boot

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	cmd_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBootOptions options for the command
type TestBootOptions struct {
	BootOptions
}

const (
	defaultBootRequirements = "test_data/requirements/jx-requirements.yml"
)

func (o *TestBootOptions) setup(bootRequirements string, dir string) {
	o.BootOptions = BootOptions{
		CommonOptions:    &opts.CommonOptions{},
		RequirementsFile: bootRequirements,
		Dir:              dir,
	}
}

func TestDetermineGitRef_DefaultGitUrl(t *testing.T) {
	t.Parallel()

	o := TestBootOptions{}
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()
	o.setup(defaultBootRequirements, bootDir)

	dir := o.createTmpRequirements(t)
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	requirements, _, err := config.LoadRequirementsConfig(dir)
	require.NoError(t, err, "unable to load tmp jx-requirements")
	resolver := &versionstream.VersionResolver{
		VersionsDir: filepath.Join("test_data", "jenkins-x-versions"),
	}

	gitRef, err := o.determineGitRef(resolver, requirements, config.DefaultBootRepository)
	require.NoError(t, err, "unable to determineGitRef")
	assert.Equal(t, "1.0.32", gitRef, "determineGitRef")
}

func TestDetermineGitRef_GitURLNotInVersionStream(t *testing.T) {
	t.Parallel()

	o := TestBootOptions{}
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()
	o.setup(defaultBootRequirements, bootDir)

	dir := o.createTmpRequirements(t)
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	requirements, _, err := config.LoadRequirementsConfig(dir)
	require.NoError(t, err, "unable to load tmp jx-requirements")
	resolver := &versionstream.VersionResolver{
		VersionsDir: filepath.Join("test_data", "jenkins-x-versions"),
	}

	gitRef, err := o.determineGitRef(resolver, requirements, "https://github.com/my-fork/my-boot-config.git")
	require.NoError(t, err, "unable to determineGitRef")
	assert.Equal(t, "master", gitRef, "determineGitRef")
}

func TestCloneDevEnvironment(t *testing.T) {
	t.Parallel()

	url := "https://github.com/jenkins-x/jenkins-x-boot-config"
	o := TestBootOptions{}
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()
	o.setup(defaultBootRequirements, bootDir)

	cloned, dir, err := o.cloneDevEnvironment(url)
	assert.Nil(t, err, "error should not be nil")

	assert.True(t, cloned)
	assert.Equal(t, "jenkins-x-boot-config", filepath.Base(dir), "cloned dir is incorrect")
}

func TestCloneDevEnvironmentIncorrectParam(t *testing.T) {
	t.Parallel()

	o := TestBootOptions{}
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()
	o.setup(defaultBootRequirements, bootDir)
	url := "not-a-url"

	cloned, dir, err := o.cloneDevEnvironment(url)
	assert.NotNil(t, err, "error should not be nil")
	assert.False(t, cloned)
	assert.Empty(t, dir)
}

func Test_determineGitURLAndRef_uses_git_repo_settings(t *testing.T) {
	t.Parallel()

	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	commonOpts := opts.NewCommonOptionsWithFactory(cmd_mocks.NewMockFactory())
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
	}

	// make the tmp directory a git repo
	gitter := gits.NewGitCLI()
	err = gitter.Init(bootDir)
	require.NoError(t, err)
	err = gitter.SetRemoteURL(bootDir, "origin", "https://github.com/acme/jenkins-x-boot-config.git")
	require.NoError(t, err)
	_, err = os.Create(filepath.Join(bootDir, "foo"))
	require.NoError(t, err)
	err = gitter.AddCommit(bootDir, "adding foo")
	require.NoError(t, err)
	err = gitter.CreateBranch(bootDir, "foo")
	require.NoError(t, err)
	err = gitter.Checkout(bootDir, "foo")
	require.NoError(t, err)

	gitURL, gitRef := o.determineGitURLAndRef()
	assert.Equal(t, "https://github.com/acme/jenkins-x-boot-config", gitURL)
	assert.Equal(t, "foo", gitRef)
}

func Test_determineGitURLAndRef_defaults_to_jenkins_x_boot_config(t *testing.T) {
	t.Parallel()

	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	commonOpts := opts.NewCommonOptionsWithFactory(cmd_mocks.NewMockFactory())
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
	}

	gitURL, gitRef := o.determineGitURLAndRef()
	assert.Equal(t, "https://github.com/jenkins-x/jenkins-x-boot-config.git", gitURL)
	assert.Equal(t, "master", gitRef)
}

func Test_determineGitURLAndRef_explicit_provided_git_options_win(t *testing.T) {
	t.Parallel()

	log.SetOutput(ioutil.Discard)
	bootDir, err := ioutil.TempDir("", "boot-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(bootDir)
	}()

	commonOpts := opts.NewCommonOptionsWithFactory(cmd_mocks.NewMockFactory())
	commonOpts.BatchMode = true
	o := BootOptions{
		CommonOptions: &commonOpts,
		Dir:           bootDir,
		GitURL:        "https://github.com/johndoe/jenkins-x-boot-config.git",
		GitRef:        "bar",
	}

	// make the tmp directory a git repo
	gitter := gits.NewGitCLI()
	err = gitter.Init(bootDir)
	require.NoError(t, err)
	err = gitter.SetRemoteURL(bootDir, "origin", "https://github.com/acme/jenkins-x-boot-config.git")
	require.NoError(t, err)
	_, err = os.Create(filepath.Join(bootDir, "foo"))
	require.NoError(t, err)
	err = gitter.AddCommit(bootDir, "adding foo")
	require.NoError(t, err)
	err = gitter.CreateBranch(bootDir, "foo")
	require.NoError(t, err)
	err = gitter.Checkout(bootDir, "foo")
	require.NoError(t, err)

	gitURL, gitRef := o.determineGitURLAndRef()
	assert.Equal(t, "https://github.com/johndoe/jenkins-x-boot-config.git", gitURL)
	assert.Equal(t, "bar", gitRef)
}

func (o *TestBootOptions) createTmpRequirements(t *testing.T) string {
	from, err := os.Open(o.RequirementsFile)
	require.NoError(t, err, "unable to open test jx-requirements")

	tmpDir, err := ioutil.TempDir("", "")
	err = os.MkdirAll(tmpDir, util.DefaultWritePermissions)
	to, err := os.Create(filepath.Join(tmpDir, "jx-requirements.yml"))
	require.NoError(t, err, "unable to create tmp jx-requirements")

	_, err = io.Copy(to, from)
	require.NoError(t, err, "unable to copy test jx-requirements to tmp")
	return tmpDir
}
