// +build unit

package step_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/step"

	"github.com/stretchr/testify/assert"
)

func TestStepWaitForArtifact(t *testing.T) {
	t.Parallel()
	options := &step.StepWaitForArtifactOptions{
		RepoURL:    step.DefaultMavenCentralRepo,
		GroupId:    "io.jenkins.updatebot",
		ArtifactId: "updatebot-core",
		Version:    "1.1.10",
		Extension:  "pom",
	}

	err := options.Run()
	assert.NoError(t, err)

	options = &step.StepWaitForArtifactOptions{
		RepoURL:    step.DefaultMavenCentralRepo,
		GroupId:    "io.jenkins.updatebot",
		ArtifactId: "does-not-exist",
		Version:    "1.1.10",
		Extension:  "pom",
		Timeout:    "1s",
		PollTime:   "100ms",
	}

	err = options.Run()
	assert.Errorf(t, err, "Should have failed to find the artifact")
}
