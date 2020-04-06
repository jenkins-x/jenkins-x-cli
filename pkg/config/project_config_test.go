// +build unit

package config_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/tests"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

var (
	testProjectConfigMaven = &config.ProjectConfig{
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
					Pipeline: &syntax.ParsedPipeline{},
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
)

func TestProjectConfigMarshal(t *testing.T) {
	t.Parallel()

	data, err := yaml.Marshal(testProjectConfigMaven)
	assert.NoError(t, err)

	if tests.IsDebugLog() {
		text := string(data)
		log.Logger().Infof("Generated YAML: %s", text)
	}

	copy := &config.ProjectConfig{}

	err = yaml.Unmarshal(data, copy)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(testProjectConfigMaven.Env), "len(testProjectConfigMaven.Env)")
	assert.NotNil(t, testProjectConfigMaven.PipelineConfig, "testProjectConfigMaven.PipelineConfig")
	assert.NotNil(t, testProjectConfigMaven.PipelineConfig.Pipelines.Release, "testProjectConfigMaven.PipelineConfig.Pipelines.Release")
	assert.Equal(t, 1, len(testProjectConfigMaven.PipelineConfig.Env), "len(testProjectConfigMaven.PipelineConfig.Env)")
}

func TestGetPipeline(t *testing.T) {
	releasePipeline, err := testProjectConfigMaven.GetPipeline(jenkinsfile.PipelineKindRelease)
	assert.NoError(t, err)
	assert.NotNil(t, releasePipeline)

	pullRequestPipeline, err := testProjectConfigMaven.GetPipeline(jenkinsfile.PipelineKindPullRequest)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "no pipeline defined for kind pullrequest")
	assert.Nil(t, pullRequestPipeline)

	featurePipeline, err := testProjectConfigMaven.GetPipeline(jenkinsfile.PipelineKindFeature)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "no pipeline defined for kind feature")
	assert.Nil(t, featurePipeline)
}
