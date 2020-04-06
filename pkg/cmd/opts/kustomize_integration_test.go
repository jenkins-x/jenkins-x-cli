// +build integration

package opts_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/log"
)

// TestInstallKustomize tests that Kustomize gets properly installed into JX_HOME
func TestInstallKustomize(t *testing.T) {
	origJXHome, testJXHome, err := testhelpers.CreateTestJxHomeDir()
	assert.NoError(t, err, "failed to create a test JX Home directory")

	defer func() {
		err := testhelpers.CleanupTestJxHomeDir(origJXHome, testJXHome)
		if err != nil {
			log.Logger().Warnf("unable to remove temporary directory %s: %s", testJXHome, err)
		}
	}()

	o := opts.NewCommonOptionsWithFactory(clients.NewFactory())
	o.NoBrew = true

	err = o.InstallKustomize()
	assert.NoError(t, err, "error installing kustomize")

	// test that tmpJXHome/bin contains kustomize binary and retrieve its version
	// unset PATH to ensure no unwanted binary will be used
	origPath := os.Getenv("PATH")
	err = os.Setenv("PATH", "")
	assert.NoError(t, err, "unable to temporarily unset PATH: %s", err)

	version, err := o.Kustomize().Version()
	assert.FileExists(t, filepath.Join(testJXHome, "bin", "kustomize"))
	assert.NoError(t, err, "kustomize was not installed in the temp JX_HOME %s", testJXHome)

	err = os.Setenv("PATH", origPath)
	assert.NoError(t, err, "unable to reset PATH: %s", err)

	// get the stable jx supported version of kustomize to be install
	versionResolver, err := o.GetVersionResolver()
	assert.NoError(t, err, "unable to retrieve version resolver")

	stableVersion, err := versionResolver.StableVersion(versionstream.KindPackage, "kustomize")
	assert.NoError(t, err, "unable to retrieve stable version for kustomize from version resolver")
	assert.Contains(t, version, stableVersion.Version)
}
