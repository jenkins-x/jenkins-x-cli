// +build unit

package builds

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestJenkinsfileGenerator(t *testing.T) {
	t.Parallel()
	projectConfig := &config.ProjectConfig{
		BuildPack: "maven",
		Env: []corev1.EnvVar{
			{
				Name:  "ORG",
				Value: "myorg",
			},
			{
				Name:  "APP_NAME",
				Value: "thingy",
			},
		},
		PipelineConfig: &jenkinsfile.PipelineConfig{
			Pipelines: jenkinsfile.Pipelines{
				PullRequest: &jenkinsfile.PipelineLifecycles{
					Build: &jenkinsfile.PipelineLifecycle{
						Steps: []*syntax.Step{
							{
								Command: "mvn test",
							},
						},
					},
				},
				Release: &jenkinsfile.PipelineLifecycles{
					Build: &jenkinsfile.PipelineLifecycle{
						Steps: []*syntax.Step{
							{
								Command: "mvn test",
							},
							{
								Command: "mvn deploy",
							},
							{
								Command: "jx promote --all-auto",
							},
						},
					},
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "PREVIEW_VERSION",
					Value: "0.0.0-SNAPSHOT-$BRANCH_NAME-$BUILD_NUMBER",
				},
			},
		},
	}

	text, err := NewJenkinsConverter(projectConfig).ToJenkinsfile()
	assert.NoError(t, err)

	t.Logf("Generated: %s\n", text)
}
