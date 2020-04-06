// +build unit

package cmd_test

import (
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/stretchr/testify/assert"
)

func TestDockerImageGetsLabel(t *testing.T) {
	t.Parallel()

	versionsDir := path.Join("test_data", "common_versions")
	assert.DirExists(t, versionsDir)

	o := &opts.CommonOptions{}
	testhelpers.ConfigureTestOptions(o, gits.NewGitCLI(), helm.NewHelmCLI("helm", helm.V2, "", true))

	resolver := &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}

	testData := map[string]string{
		"alreadyversioned:7.8.9": "alreadyversioned:7.8.9",
		"maven":                  "maven:1.2.3",
		"docker.io/maven":        "maven:1.2.3",
		"gcr.io/cheese":          "gcr.io/cheese:4.5.6",
		"noversion":              "noversion",
	}

	for image, expected := range testData {
		actual, err := resolver.ResolveDockerImage(image)
		if assert.NoError(t, err, "resolving image %s", image) {
			assert.Equal(t, expected, actual, "resolving image %s", image)
		}
	}
}
