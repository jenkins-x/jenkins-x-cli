// +build integration

package upgrade

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeAvailable(t *testing.T) {
	o := UpgradeBootOptions{
		CommonOptions: &opts.CommonOptions{},
	}

	tests := []struct {
		name             string
		versionStreamRef string
		latestRelease    bool
		wantSha          bool
		wantTag          bool
		upgradeVSRef     string
	}{
		{
			name:             "TestUpgradeAvailableFromTaggedVersion",
			versionStreamRef: "v1.0.35",
			wantSha:          true,
		},
		{
			name:             "TestUpgradeAvailableFromSha",
			versionStreamRef: "282fd7579ef82df408ccd2d425f99779784f75a9",
			wantSha:          true,
		},
		{
			name:             "TestUpgradeAvailableLatestReleaseFromTaggedVersion",
			versionStreamRef: "v1.0.35",
			latestRelease:    true,
			wantTag:          true,
		},
		{
			name:             "TestUpgradeAvailableLatestReleaseFromSha",
			versionStreamRef: "282fd7579ef82df408ccd2d425f99779784f75a9",
			latestRelease:    true,
			wantTag:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.latestRelease {
				o.LatestRelease = true
			}

			upgradeRef, err := o.upgradeAvailable(config.DefaultVersionsURL, tt.versionStreamRef, config.DefaultVersionsRef)
			assert.NoError(t, err)

			if tt.wantSha {
				assert.Equal(t, len(upgradeRef), 40)
			}

			if tt.wantTag {
				upgradeRefText := strings.TrimPrefix(upgradeRef, "v")
				_, err = semver.Parse(upgradeRefText)
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateBootConfig(t *testing.T) {
	origJxHome := os.Getenv("JX_HOME")

	tmpJxHome, err := ioutil.TempDir("", "jx-test-"+t.Name())
	assert.NoError(t, err)

	err = os.Setenv("JX_HOME", tmpJxHome)
	assert.NoError(t, err)

	defer func() {
		_ = os.RemoveAll(tmpJxHome)
		err = os.Setenv("JX_HOME", origJxHome)
	}()

	o := UpgradeBootOptions{
		CommonOptions: &opts.CommonOptions{},
	}

	// Prep the local clone with the contents of the boot config version we're going to upgrade from
	tmpDir := initializeTempGitRepo(t, o.Git(), "v1.0.35")
	defer func() {
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err, "could not clean up temp boot clone")
	}()

	o.Dir = tmpDir

	err = o.updateBootConfig(config.DefaultVersionsURL, "v1.0.161", config.DefaultBootRepository, "282fd7579ef82df408ccd2d425f99779784f75a9")
	assert.NoError(t, err)
}

func initializeTempGitRepo(t *testing.T, gitter gits.Gitter, bootRef string) string {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	tmpCloneDir, err := ioutil.TempDir("", "update-boot-config-test-")
	assert.NoError(t, err)

	defer func() {
		err := os.RemoveAll(tmpCloneDir)
		require.NoError(t, err, "could not clean up temp boot clone")
	}()

	// Clone the boot config repo
	err = gitter.Clone(config.DefaultBootRepository, tmpCloneDir)
	assert.NoError(t, err)

	// Fetch tags.
	err = gitter.FetchTags(tmpCloneDir)
	assert.NoError(t, err)

	// Check out the boot ref.
	err = gitter.Checkout(tmpCloneDir, bootRef)
	assert.NoError(t, err)

	// Copy the contents of the boot config repo to the temp repo.
	err = util.CopyDir(tmpCloneDir, dir, true)
	assert.NoError(t, err)

	// Remove the .git directory
	err = os.RemoveAll(filepath.Join(dir, ".git"))
	assert.NoError(t, err)

	err = gitter.Init(dir)
	assert.NoError(t, err)

	err = gitter.Add(dir, ".")
	assert.NoError(t, err)

	err = gitter.AddCommit(dir, "Initial Commit")
	assert.NoError(t, err)

	return dir
}
