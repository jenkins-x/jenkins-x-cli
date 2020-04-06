// +build unit

package versionstream_test

import (
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/stretchr/testify/assert"
)

func TestVersionGitRepository(t *testing.T) {
	t.Parallel()

	versionsDir := path.Join("test_data", "jenkins-x-versions-git-repo")
	assert.DirExists(t, versionsDir)

	resolver := &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}

	testData := map[string]string{
		"https://github.com/jenkins-x/jenkins-x-boot-config":     "1.2.3",
		"https://github.com/jenkins-x/jenkins-x-boot-config.git": "1.2.3",
	}

	for gitURL, expected := range testData {
		actual, err := resolver.ResolveGitVersion(gitURL)
		if assert.NoError(t, err, "resolving git URL version %s", gitURL) {
			assert.Equal(t, expected, actual, "resolving git URL version %s", gitURL)
		}
	}
}
