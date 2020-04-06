// +build unit

package get_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/cmd/step/get"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/acarl005/stripansi"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStepGetVersionChangeSetOptionsBranch(t *testing.T) {
	t.Parallel()
	testDir, _ := ioutil.TempDir("", "test-version-changesetbranch")
	defer func() {
		err := os.RemoveAll(testDir)
		assert.NoError(t, err)
	}()
	repoOwnerUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	repoOwner := repoOwnerUUID.String()
	repoNameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	repoName := repoNameUUID.String()
	fakeRepo, _ := gits.NewFakeRepository(repoOwner, repoName, nil, nil)
	fakeGitProvider := gits.NewFakeProvider(fakeRepo)
	testBranch := "test-app-version-bump"
	stableBranch := "master"
	r, fakeStdout, _ := os.Pipe()
	options := &get.StepGetVersionChangeSetOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
		VersionsDir:   testDir,
		TestingBranch: testBranch,
		StableBranch:  stableBranch,
	}
	options.CommonOptions.Out = fakeStdout
	testhelpers.ConfigureTestOptionsWithResources(options.CommonOptions,
		[]runtime.Object{},
		[]runtime.Object{},
		gits.NewGitLocal(),
		fakeGitProvider,
		helm.NewHelmCLI("helm", helm.V2, "", true),
		resources_test.NewMockInstaller(),
	)
	gitter := options.Git()
	gitter.Init(testDir)
	stableVersion := &versionstream.StableVersion{
		Version: "1.0.0",
		GitURL:  "fake",
	}
	gitter.AddRemote(testDir, "origin", fakeRepo.GitRepo.CloneURL)
	versionstream.SaveStableVersion(testDir, versionstream.KindChart, "test-app", stableVersion)
	gitter.Add(testDir, ".")
	gitter.AddCommit(testDir, "Initial Commit")
	gitter.Push(testDir, "origin", false, "HEAD")
	gitter.CreateBranch(testDir, testBranch)
	gitter.Checkout(testDir, testBranch)
	stableVersion.Version = "1.0.1"
	versionstream.SaveStableVersion(testDir, versionstream.KindChart, "test-app", stableVersion)
	gitter.AddCommit(testDir, "Bump version")
	gitter.Push(testDir, "origin", false, "HEAD")
	gitter.Checkout(testDir, stableBranch)

	err = options.Run()
	assert.NoError(t, err)
	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.Contains(t, output, `JX_CHANGED_VERSIONS="charts:test-app:1.0.1"`)
	assert.Contains(t, output, `JX_STABLE_VERSIONS="charts:test-app:1.0.0"`)

}

func TestStepGetVersionChangeSetOptionsPR(t *testing.T) {
	t.Parallel()
	testDir, _ := ioutil.TempDir("", "test-version-changesetpr")
	defer func() {
		err := os.RemoveAll(testDir)
		assert.NoError(t, err)
	}()
	repoOwnerUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	repoOwner := repoOwnerUUID.String()
	repoNameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	repoName := repoNameUUID.String()
	fakeRepo, _ := gits.NewFakeRepository(repoOwner, repoName, nil, nil)
	fakeGitProvider := gits.NewFakeProvider(fakeRepo)
	testBranch := "test-app-version-bump"
	stableBranch := "master"
	r, fakeStdout, _ := os.Pipe()
	options := &get.StepGetVersionChangeSetOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
		VersionsDir:  testDir,
		PR:           "1",
		StableBranch: stableBranch,
	}
	options.CommonOptions.Out = fakeStdout
	testhelpers.ConfigureTestOptionsWithResources(options.CommonOptions,
		[]runtime.Object{},
		[]runtime.Object{},
		gits.NewGitLocal(),
		fakeGitProvider,
		helm.NewHelmCLI("helm", helm.V2, "", true),
		resources_test.NewMockInstaller(),
	)
	gitter := options.Git()
	gitter.Init(testDir)
	stableVersion := &versionstream.StableVersion{
		Version: "1.0.0",
		GitURL:  "fake",
	}
	gitter.AddRemote(testDir, "origin", fakeRepo.GitRepo.CloneURL)
	versionstream.SaveStableVersion(testDir, versionstream.KindChart, "test-app", stableVersion)
	gitter.Add(testDir, ".")
	gitter.AddCommit(testDir, "Initial Commit")
	gitter.Push(testDir, "origin", false, "HEAD")
	gitter.CreateBranch(testDir, testBranch)
	gitter.Checkout(testDir, testBranch)
	stableVersion.Version = "1.0.1"
	versionstream.SaveStableVersion(testDir, versionstream.KindChart, "test-app", stableVersion)
	gitter.AddCommit(testDir, "Bump version")
	gitter.CreateBranch(testDir, "1")
	gitter.Checkout(testDir, stableBranch)

	err = options.Run()
	assert.NoError(t, err)
	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.Contains(t, output, `JX_CHANGED_VERSIONS="charts:test-app:1.0.1"`)
	assert.Contains(t, output, `JX_STABLE_VERSIONS="charts:test-app:1.0.0"`)

}
