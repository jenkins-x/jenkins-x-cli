// +build unit

package syntax_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/syntax"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	jxsyntax "github.com/jenkins-x/jx/pkg/tekton/syntax"
	sht "github.com/jenkins-x/jx/pkg/tekton/syntax/syntax_helpers_test"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/knative/pkg/kmp"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

var (
	overrideAfter  = jxsyntax.StepOverrideAfter
	overrideBefore = jxsyntax.StepOverrideBefore
)

func TestCreateCanonicalPipeline(t *testing.T) {
	t.Parallel()

	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stderr)

	testData := path.Join("..", "create", "test_data", "step_create_task")
	_, err := os.Stat(testData)
	assert.NoError(t, err)

	testVersionsDir := path.Join(testData, "stable_versions")
	packsDir := path.Join(testData, "packs")
	_, err = os.Stat(packsDir)
	assert.NoError(t, err)

	resolver := func(importFile *jenkinsfile.ImportFile) (string, error) {
		dirPath := []string{packsDir, "import_dir", importFile.Import}
		// lets handle cross platform paths in `importFile.File`
		path := append(dirPath, strings.Split(importFile.File, "/")...)
		return filepath.Join(path...), nil
	}

	cases := []struct {
		name         string
		pack         string
		repoName     string
		organization string
		branch       string
		customEnvs   []string
		expected     *config.ProjectConfig
	}{{
		name:         "js_build_pack_with_yaml",
		pack:         "javascript",
		repoName:     "js-test-repo",
		organization: "abayer",
		branch:       "build-pack",
		customEnvs:   []string{"DOCKER_REGISTRY=gcr.io"},
		expected: &config.ProjectConfig{
			BuildPack: "javascript",
			PipelineConfig: &jenkinsfile.PipelineConfig{
				Agent: &jxsyntax.Agent{
					Container: "nodejs",
					Label:     "jenkins-nodejs",
				},
				Env: []corev1.EnvVar{
					{
						Name:  "DOCKER_REGISTRY",
						Value: "gcr.io",
					},
				},
				Extends: &jenkinsfile.PipelineExtends{
					File:   "javascript/pipeline.yaml",
					Import: "classic",
				},
				Pipelines: jenkinsfile.Pipelines{
					Post: &jenkinsfile.PipelineLifecycle{
						Steps:    []*jxsyntax.Step{},
						PreSteps: []*jxsyntax.Step{},
					},
					PullRequest: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineEnvVar("DOCKER_REGISTRY", "gcr.io"),
							sht.PipelineOptions(
								sht.PipelineContainerOptions(
									sht.ContainerResourceRequests("400m", "512Mi"),
									tb.EnvVar("DOCKER_CONFIG", "/home/jenkins/.docker/"),
									sht.EnvVar("DOCKER_REGISTRY", "gcr.io"),
									tb.EnvVar("GIT_AUTHOR_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_AUTHOR_NAME", "jenkins-x-bot"),
									tb.EnvVar("GIT_COMMITTER_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_COMMITTER_NAME", "jenkins-x-bot"),
									tb.EnvVar("JENKINS_URL", "http://jenkins:8080"),
									tb.EnvVar("TILLER_NAMESPACE", "kube-system"),
									tb.EnvVar("XDG_CONFIG_HOME", "/home/jenkins"),
									sht.ContainerSecurityContext(true),
									tb.VolumeMount("workspace-volume", "/home/jenkins"),
									tb.VolumeMount("docker-daemon", "/var/run/docker.sock"),
									tb.VolumeMount("volume-0", "/home/jenkins/.docker"),
								),
								sht.PipelineVolume(&corev1.Volume{
									Name: "workspace-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "docker-daemon",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/var/run/docker.sock",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-0",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-docker-cfg",
										},
									},
								}),
							),
							sht.PipelineStage("from-build-pack",
								sht.StageAgent("nodejs"),
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("npm install"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-npm-install")),
								sht.StageStep(sht.StepCmd("CI=true DISPLAY=:99 npm test"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-step3")),
								sht.StageStep(sht.StepCmd("/kaniko/executor"), sht.StepDir("/workspace/source"),
									sht.StepImage(jxsyntax.KanikoDockerImage), sht.StepName("build-container-build"),
									sht.StepArg("--cache=true"), sht.StepArg("--cache-dir=/workspace"),
									sht.StepArg("--context=/workspace/source"), sht.StepArg("--dockerfile=/workspace/source/Dockerfile"),
									sht.StepArg("--destination=gcr.io/abayer/js-test-repo:${inputs.params.version}"),
									sht.StepArg("--cache-repo=gcr.io//cache")),
								sht.StageStep(sht.StepCmd("jx step post build --image $DOCKER_REGISTRY/$ORG/$APP_NAME:$PREVIEW_VERSION"),
									sht.StepDir("/workspace/source"), sht.StepImage("nodejs"), sht.StepName("postbuild-post-build")),
								sht.StageStep(sht.StepCmd("make preview"), sht.StepDir("/workspace/source/charts/preview"),
									sht.StepImage("nodejs"), sht.StepName("promote-make-preview")),
								sht.StageStep(sht.StepCmd("jx preview --app $APP_NAME --dir ../.."), sht.StepDir("/workspace/source/charts/preview"),
									sht.StepImage("nodejs"), sht.StepName("promote-jx-preview")),
							),
						),
					},
					Release: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineEnvVar("DOCKER_REGISTRY", "gcr.io"),
							sht.PipelineOptions(
								sht.PipelineContainerOptions(
									sht.ContainerResourceRequests("400m", "512Mi"),
									tb.EnvVar("DOCKER_CONFIG", "/home/jenkins/.docker/"),
									sht.EnvVar("DOCKER_REGISTRY", "gcr.io"),
									tb.EnvVar("GIT_AUTHOR_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_AUTHOR_NAME", "jenkins-x-bot"),
									tb.EnvVar("GIT_COMMITTER_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_COMMITTER_NAME", "jenkins-x-bot"),
									tb.EnvVar("JENKINS_URL", "http://jenkins:8080"),
									tb.EnvVar("TILLER_NAMESPACE", "kube-system"),
									tb.EnvVar("XDG_CONFIG_HOME", "/home/jenkins"),
									sht.ContainerSecurityContext(true),
									tb.VolumeMount("workspace-volume", "/home/jenkins"),
									tb.VolumeMount("docker-daemon", "/var/run/docker.sock"),
									tb.VolumeMount("volume-0", "/home/jenkins/.docker"),
								),
								sht.PipelineVolume(&corev1.Volume{
									Name: "workspace-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "docker-daemon",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/var/run/docker.sock",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-0",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-docker-cfg",
										},
									},
								}),
							),
							sht.PipelineStage("from-build-pack",
								sht.StageAgent("nodejs"),
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("jx step git credentials"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("setup-jx-git-credentials")),
								sht.StageStep(sht.StepCmd("npm install"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-npm-install")),
								sht.StageStep(sht.StepCmd("CI=true DISPLAY=:99 npm test"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-npm-test")),
								sht.StageStep(sht.StepCmd("/kaniko/executor"), sht.StepDir("/workspace/source"),
									sht.StepImage(jxsyntax.KanikoDockerImage), sht.StepName("build-container-build"),
									sht.StepArg("--cache=true"), sht.StepArg("--cache-dir=/workspace"),
									sht.StepArg("--context=/workspace/source"), sht.StepArg("--dockerfile=/workspace/source/Dockerfile"),
									sht.StepArg("--destination=gcr.io/abayer/js-test-repo:${inputs.params.version}"),
									sht.StepArg("--cache-repo=gcr.io//cache")),
								sht.StageStep(sht.StepCmd("jx step post build --image $DOCKER_REGISTRY/$ORG/$APP_NAME:${VERSION}"),
									sht.StepDir("/workspace/source"), sht.StepImage("nodejs"), sht.StepName("build-post-build")),
								sht.StageStep(sht.StepCmd("jx step changelog --batch-mode --version v${VERSION}"),
									sht.StepDir("/workspace/source/charts/js-test-repo"), sht.StepImage("nodejs"),
									sht.StepName("promote-changelog")),
								sht.StageStep(sht.StepCmd("jx step helm release"), sht.StepDir("/workspace/source/charts/js-test-repo"),
									sht.StepImage("nodejs"), sht.StepName("promote-helm-release")),
								sht.StageStep(sht.StepCmd("jx promote -b --all-auto --timeout 1h --version ${VERSION}"),
									sht.StepDir("/workspace/source/charts/js-test-repo"),
									sht.StepImage("nodejs"), sht.StepName("promote-jx-promote")),
							),
						),
						SetVersion: &jenkinsfile.PipelineLifecycle{
							Steps: []*jxsyntax.Step{{
								Image: "nodejs",
								Steps: []*jxsyntax.Step{{
									Comment: "so we can retrieve the version in later steps",
									Name:    "next-version",
									Sh:      "echo \\$(jx-release-version) > VERSION",
									Steps:   []*jxsyntax.Step{},
								}, {
									Name:  "tag-version",
									Sh:    "jx step tag --version \\$(cat VERSION)",
									Steps: []*jxsyntax.Step{},
								}},
							}},
							PreSteps: []*jxsyntax.Step{},
						},
					},
				},
			},
		},
	}, {
		name:         "js_build_pack",
		pack:         "javascript",
		repoName:     "js-test-repo",
		organization: "abayer",
		branch:       "build-pack",
		expected: &config.ProjectConfig{
			PipelineConfig: &jenkinsfile.PipelineConfig{
				Agent: &jxsyntax.Agent{
					Container: "nodejs",
					Label:     "jenkins-nodejs",
				},
				Env: []corev1.EnvVar{},
				Extends: &jenkinsfile.PipelineExtends{
					File:   "javascript/pipeline.yaml",
					Import: "classic",
				},
				Pipelines: jenkinsfile.Pipelines{
					Post: &jenkinsfile.PipelineLifecycle{
						Steps:    []*jxsyntax.Step{},
						PreSteps: []*jxsyntax.Step{},
					},
					PullRequest: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineOptions(
								sht.PipelineContainerOptions(
									sht.ContainerResourceRequests("400m", "512Mi"),
									tb.EnvVar("DOCKER_CONFIG", "/home/jenkins/.docker/"),
									sht.EnvVarFrom("DOCKER_REGISTRY", &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											Key: "docker.registry",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jenkins-x-docker-registry",
											}}}),
									tb.EnvVar("GIT_AUTHOR_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_AUTHOR_NAME", "jenkins-x-bot"),
									tb.EnvVar("GIT_COMMITTER_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_COMMITTER_NAME", "jenkins-x-bot"),
									tb.EnvVar("JENKINS_URL", "http://jenkins:8080"),
									tb.EnvVar("TILLER_NAMESPACE", "kube-system"),
									tb.EnvVar("XDG_CONFIG_HOME", "/home/jenkins"),
									sht.ContainerSecurityContext(true),
									tb.VolumeMount("workspace-volume", "/home/jenkins"),
									tb.VolumeMount("docker-daemon", "/var/run/docker.sock"),
									tb.VolumeMount("volume-0", "/home/jenkins/.docker"),
								),
								sht.PipelineVolume(&corev1.Volume{
									Name: "workspace-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "docker-daemon",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/var/run/docker.sock",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-0",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-docker-cfg",
										},
									},
								}),
							),
							sht.PipelineStage("from-build-pack",
								sht.StageAgent("nodejs"),
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("npm install"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-npm-install")),
								sht.StageStep(sht.StepCmd("CI=true DISPLAY=:99 npm test"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-step3")),
								sht.StageStep(sht.StepCmd("/kaniko/executor"), sht.StepDir("/workspace/source"),
									sht.StepImage(jxsyntax.KanikoDockerImage), sht.StepName("build-container-build"),
									sht.StepArg("--cache=true"), sht.StepArg("--cache-dir=/workspace"),
									sht.StepArg("--context=/workspace/source"), sht.StepArg("--dockerfile=/workspace/source/Dockerfile"),
									sht.StepArg("--destination=gcr.io/abayer/js-test-repo:${inputs.params.version}"),
									sht.StepArg("--cache-repo=gcr.io//cache")),
								sht.StageStep(sht.StepCmd("jx step post build --image $DOCKER_REGISTRY/$ORG/$APP_NAME:$PREVIEW_VERSION"),
									sht.StepDir("/workspace/source"), sht.StepImage("nodejs"), sht.StepName("postbuild-post-build")),
								sht.StageStep(sht.StepCmd("make preview"), sht.StepDir("/workspace/source/charts/preview"),
									sht.StepImage("nodejs"), sht.StepName("promote-make-preview")),
								sht.StageStep(sht.StepCmd("jx preview --app $APP_NAME --dir ../.."), sht.StepDir("/workspace/source/charts/preview"),
									sht.StepImage("nodejs"), sht.StepName("promote-jx-preview")),
							),
						),
					},
					Release: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineOptions(
								sht.PipelineContainerOptions(
									sht.ContainerResourceRequests("400m", "512Mi"),
									tb.EnvVar("DOCKER_CONFIG", "/home/jenkins/.docker/"),
									sht.EnvVarFrom("DOCKER_REGISTRY", &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											Key: "docker.registry",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jenkins-x-docker-registry",
											}}}),
									tb.EnvVar("GIT_AUTHOR_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_AUTHOR_NAME", "jenkins-x-bot"),
									tb.EnvVar("GIT_COMMITTER_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_COMMITTER_NAME", "jenkins-x-bot"),
									tb.EnvVar("JENKINS_URL", "http://jenkins:8080"),
									tb.EnvVar("TILLER_NAMESPACE", "kube-system"),
									tb.EnvVar("XDG_CONFIG_HOME", "/home/jenkins"),
									sht.ContainerSecurityContext(true),
									tb.VolumeMount("workspace-volume", "/home/jenkins"),
									tb.VolumeMount("docker-daemon", "/var/run/docker.sock"),
									tb.VolumeMount("volume-0", "/home/jenkins/.docker"),
								),
								sht.PipelineVolume(&corev1.Volume{
									Name: "workspace-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "docker-daemon",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/var/run/docker.sock",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-0",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-docker-cfg",
										},
									},
								}),
							),
							sht.PipelineStage("from-build-pack",
								sht.StageAgent("nodejs"),
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("jx step git credentials"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("setup-jx-git-credentials")),
								sht.StageStep(sht.StepCmd("npm install"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-npm-install")),
								sht.StageStep(sht.StepCmd("CI=true DISPLAY=:99 npm test"), sht.StepDir("/workspace/source"),
									sht.StepImage("nodejs"), sht.StepName("build-npm-test")),
								sht.StageStep(sht.StepCmd("/kaniko/executor"), sht.StepDir("/workspace/source"),
									sht.StepImage(jxsyntax.KanikoDockerImage), sht.StepName("build-container-build"),
									sht.StepArg("--cache=true"), sht.StepArg("--cache-dir=/workspace"),
									sht.StepArg("--context=/workspace/source"), sht.StepArg("--dockerfile=/workspace/source/Dockerfile"),
									sht.StepArg("--destination=gcr.io/abayer/js-test-repo:${inputs.params.version}"),
									sht.StepArg("--cache-repo=gcr.io//cache")),
								sht.StageStep(sht.StepCmd("jx step post build --image $DOCKER_REGISTRY/$ORG/$APP_NAME:${VERSION}"),
									sht.StepDir("/workspace/source"), sht.StepImage("nodejs"), sht.StepName("build-post-build")),
								sht.StageStep(sht.StepCmd("jx step changelog --batch-mode --version v${VERSION}"),
									sht.StepDir("/workspace/source/charts/js-test-repo"), sht.StepImage("nodejs"),
									sht.StepName("promote-changelog")),
								sht.StageStep(sht.StepCmd("jx step helm release"), sht.StepDir("/workspace/source/charts/js-test-repo"),
									sht.StepImage("nodejs"), sht.StepName("promote-helm-release")),
								sht.StageStep(sht.StepCmd("jx promote -b --all-auto --timeout 1h --version ${VERSION}"),
									sht.StepDir("/workspace/source/charts/js-test-repo"),
									sht.StepImage("nodejs"), sht.StepName("promote-jx-promote")),
							),
						),
						SetVersion: &jenkinsfile.PipelineLifecycle{
							Steps: []*jxsyntax.Step{{
								Image: "nodejs",
								Steps: []*jxsyntax.Step{{
									Comment: "so we can retrieve the version in later steps",
									Name:    "next-version",
									Sh:      "echo \\$(jx-release-version) > VERSION",
									Steps:   []*jxsyntax.Step{},
								}, {
									Name:  "tag-version",
									Sh:    "jx step tag --version \\$(cat VERSION)",
									Steps: []*jxsyntax.Step{},
								}},
							}},
							PreSteps: []*jxsyntax.Step{},
						},
					},
				},
			},
		},
	}, {
		name:         "default-in-buildpack",
		pack:         "default-pipeline",
		repoName:     "golang-qs-test",
		organization: "abayer",
		branch:       "master",
		expected: &config.ProjectConfig{
			PipelineConfig: &jenkinsfile.PipelineConfig{
				Agent: &jxsyntax.Agent{
					Container: "go",
					Label:     "builder-go",
				},
				Env: []corev1.EnvVar{},
				Pipelines: jenkinsfile.Pipelines{
					Default: sht.ParsedPipeline(
						sht.PipelineAgent("go"),
						sht.PipelineStage("from-build-pack",
							sht.StageStep(sht.StepCmd("make build"), sht.StepName("build")),
							sht.StageStep(sht.StepCmd("jx step helm build"), sht.StepName("helm-build")),
						),
					),
					Feature: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("go"),
							sht.PipelineStage("from-build-pack",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("make build"), sht.StepName("build")),
								sht.StageStep(sht.StepCmd("jx step helm build"), sht.StepName("helm-build")),
							),
						),
					},
					PullRequest: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("go"),
							sht.PipelineStage("from-build-pack",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("make build"), sht.StepName("build")),
								sht.StageStep(sht.StepCmd("jx step helm build"), sht.StepName("helm-build")),
							),
						),
					},
					Release: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("go"),
							sht.PipelineStage("from-build-pack",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("make build"), sht.StepName("build")),
								sht.StageStep(sht.StepCmd("jx step helm build"), sht.StepName("helm-build")),
							),
						),
					},
				},
			},
		},
	}, {
		name:         "add-env-to-default-in-buildpack",
		pack:         "default-pipeline",
		repoName:     "golang-qs-test",
		organization: "abayer",
		branch:       "master",
		expected: &config.ProjectConfig{
			PipelineConfig: &jenkinsfile.PipelineConfig{
				Agent: &jxsyntax.Agent{
					Image: "go",
					Label: "builder-go",
				},
				Env: []corev1.EnvVar{{
					Name:  "FRUIT",
					Value: "BANANA",
				}, {
					Name:  "GIT_AUTHOR_NAME",
					Value: "somebodyelse",
				}},
				Pipelines: jenkinsfile.Pipelines{
					Feature: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("go"),
							sht.PipelineEnvVar("FRUIT", "BANANA"),
							sht.PipelineEnvVar("GIT_AUTHOR_NAME", "somebodyelse"),
							sht.PipelineStage("from-build-pack",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("make build"), sht.StepName("build")),
								sht.StageStep(sht.StepCmd("jx step helm build"), sht.StepName("helm-build")),
							),
						),
					},
					PullRequest: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("go"),
							sht.PipelineEnvVar("FRUIT", "BANANA"),
							sht.PipelineEnvVar("GIT_AUTHOR_NAME", "somebodyelse"),
							sht.PipelineStage("from-build-pack",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("make build"), sht.StepName("build")),
								sht.StageStep(sht.StepCmd("jx step helm build"), sht.StepName("helm-build")),
							),
						),
					},
					Release: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("go"),
							sht.PipelineEnvVar("FRUIT", "BANANA"),
							sht.PipelineEnvVar("GIT_AUTHOR_NAME", "somebodyelse"),
							sht.PipelineStage("from-build-pack",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("make build"), sht.StepName("build")),
								sht.StageStep(sht.StepCmd("jx step helm build"), sht.StepName("helm-build")),
							),
						),
					},
				},
			},
		},
	}, {
		name:         "append-and-prepend-stage-steps",
		pack:         "maven",
		repoName:     "jx-demo-qs",
		organization: "abayer",
		branch:       "master",
		expected: &config.ProjectConfig{
			PipelineConfig: &jenkinsfile.PipelineConfig{
				Agent: &jxsyntax.Agent{
					Image: "maven",
					Label: "jenkins-maven",
				},
				Env: []corev1.EnvVar{{
					Name:  "FRUIT",
					Value: "BANANA",
				}, {
					Name:  "GIT_AUTHOR_NAME",
					Value: "somebodyelse",
				}},
				Pipelines: jenkinsfile.Pipelines{
					Overrides: []*jxsyntax.PipelineOverride{{
						Pipeline: "release",
						Stage:    "build",
						Steps: []*jxsyntax.Step{{
							Sh: "echo hi there",
						}},
						Type: &overrideBefore,
					}, {
						Pipeline: "release",
						Stage:    "build",
						Steps: []*jxsyntax.Step{{
							Sh: "echo goodbye",
						}, {
							Sh: "echo wait why am i here",
						}},
						Type: &overrideAfter,
					}},
					Post: &jenkinsfile.PipelineLifecycle{
						Steps:    []*jxsyntax.Step{},
						PreSteps: []*jxsyntax.Step{},
					},
					PullRequest: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineEnvVar("FRUIT", "BANANA"),
							sht.PipelineEnvVar("GIT_AUTHOR_NAME", "somebodyelse"),
							sht.PipelineOptions(
								sht.PipelineContainerOptions(
									tb.EnvVar("DOCKER_CONFIG", "/home/jenkins/.docker/"),
									sht.EnvVarFrom("DOCKER_REGISTRY", &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											Key: "docker.registry",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jenkins-x-docker-registry",
											}}}),
									tb.EnvVar("FRUIT", "BANANA"),
									tb.EnvVar("GIT_AUTHOR_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_AUTHOR_NAME", "somebodyelse"),
									tb.EnvVar("GIT_COMMITTER_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_COMMITTER_NAME", "jenkins-x-bot"),
									tb.EnvVar("JENKINS_URL", "http://jenkins:8080"),
									tb.EnvVar("TILLER_NAMESPACE", "kube-system"),
									tb.EnvVar("XDG_CONFIG_HOME", "/home/jenkins"),
									tb.EnvVar("_JAVA_OPTIONS", "-XX:+UnlockExperimentalVMOptions -Dsun.zip.disableMemoryMapping=true -XX:+UseParallelGC -XX:MinHeapFreeRatio=5 -XX:MaxHeapFreeRatio=10 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 -Xms10m -Xmx192m"),
									sht.ContainerSecurityContext(true),
									tb.VolumeMount("workspace-volume", "/home/jenkins"),
									tb.VolumeMount("docker-daemon", "/var/run/docker.sock"),
									tb.VolumeMount("volume-0", "/root/.m2/"),
									tb.VolumeMount("volume-1", "/home/jenkins/.docker"),
									tb.VolumeMount("volume-2", "/home/jenkins/.gnupg"),
								),
								sht.PipelineVolume(&corev1.Volume{
									Name: "workspace-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "docker-daemon",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/var/run/docker.sock",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-0",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-maven-settings",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-1",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-docker-cfg",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-2",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-release-gpg",
										},
									},
								}),
							),
							sht.PipelineStage("from-build-pack",
								sht.StageAgent("maven"),
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("mvn versions:set -DnewVersion=$PREVIEW_VERSION"), sht.StepImage("maven"),
									sht.StepDir("/workspace/source"), sht.StepName("build-set-version")),
								sht.StageStep(sht.StepCmd("mvn install"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("build-mvn-install")),
								sht.StageStep(sht.StepCmd("skaffold version"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("build-skaffold-version")),
								sht.StageStep(sht.StepCmd("/kaniko/executor"), sht.StepDir("/workspace/source"),
									sht.StepImage(jxsyntax.KanikoDockerImage), sht.StepName("build-container-build"),
									sht.StepArg("--cache=true"), sht.StepArg("--cache-dir=/workspace"),
									sht.StepArg("--context=/workspace/source"), sht.StepArg("--dockerfile=/workspace/source/Dockerfile"),
									sht.StepArg("--destination=gcr.io/abayer/jx-demo-qs:${inputs.params.version}"),
									sht.StepArg("--cache-repo=gcr.io//cache")),
								sht.StageStep(sht.StepCmd("jx step post build --image $DOCKER_REGISTRY/$ORG/$APP_NAME:$PREVIEW_VERSION"),
									sht.StepDir("/workspace/source"), sht.StepImage("maven"), sht.StepName("postbuild-post-build")),
								sht.StageStep(sht.StepCmd("make preview"), sht.StepDir("/workspace/source/charts/preview"),
									sht.StepImage("maven"), sht.StepName("promote-make-preview")),
								sht.StageStep(sht.StepCmd("jx preview --app $APP_NAME --dir ../.."), sht.StepDir("/workspace/source/charts/preview"),
									sht.StepImage("maven"), sht.StepName("promote-jx-preview")),
							),
						),
					},
					Release: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineEnvVar("FRUIT", "BANANA"),
							sht.PipelineEnvVar("GIT_AUTHOR_NAME", "somebodyelse"),
							sht.PipelineOptions(
								sht.PipelineContainerOptions(
									tb.EnvVar("DOCKER_CONFIG", "/home/jenkins/.docker/"),
									sht.EnvVarFrom("DOCKER_REGISTRY", &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											Key: "docker.registry",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jenkins-x-docker-registry",
											}}}),
									tb.EnvVar("FRUIT", "BANANA"),
									tb.EnvVar("GIT_AUTHOR_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_AUTHOR_NAME", "somebodyelse"),
									tb.EnvVar("GIT_COMMITTER_EMAIL", "jenkins-x@googlegroups.com"),
									tb.EnvVar("GIT_COMMITTER_NAME", "jenkins-x-bot"),
									tb.EnvVar("JENKINS_URL", "http://jenkins:8080"),
									tb.EnvVar("TILLER_NAMESPACE", "kube-system"),
									tb.EnvVar("XDG_CONFIG_HOME", "/home/jenkins"),
									tb.EnvVar("_JAVA_OPTIONS", "-XX:+UnlockExperimentalVMOptions -Dsun.zip.disableMemoryMapping=true -XX:+UseParallelGC -XX:MinHeapFreeRatio=5 -XX:MaxHeapFreeRatio=10 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 -Xms10m -Xmx192m"),
									sht.ContainerSecurityContext(true),
									tb.VolumeMount("workspace-volume", "/home/jenkins"),
									tb.VolumeMount("docker-daemon", "/var/run/docker.sock"),
									tb.VolumeMount("volume-0", "/root/.m2/"),
									tb.VolumeMount("volume-1", "/home/jenkins/.docker"),
									tb.VolumeMount("volume-2", "/home/jenkins/.gnupg"),
								),
								sht.PipelineVolume(&corev1.Volume{
									Name: "workspace-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "docker-daemon",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/var/run/docker.sock",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-0",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-maven-settings",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-1",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-docker-cfg",
										},
									},
								}),
								sht.PipelineVolume(&corev1.Volume{
									Name: "volume-2",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "jenkins-release-gpg",
										},
									},
								}),
							),
							sht.PipelineStage("from-build-pack",
								sht.StageAgent("maven"),
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("jx step git credentials"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("setup-jx-git-credentials")),
								sht.StageStep(sht.StepCmd("echo hi there"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("build-step3")),
								sht.StageStep(sht.StepCmd("mvn clean deploy"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("build-mvn-deploy")),
								sht.StageStep(sht.StepCmd("skaffold version"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("build-skaffold-version")),
								sht.StageStep(sht.StepCmd("/kaniko/executor"), sht.StepDir("/workspace/source"),
									sht.StepImage(jxsyntax.KanikoDockerImage), sht.StepName("build-container-build"),
									sht.StepArg("--cache=true"), sht.StepArg("--cache-dir=/workspace"),
									sht.StepArg("--context=/workspace/source"), sht.StepArg("--dockerfile=/workspace/source/Dockerfile"),
									sht.StepArg("--destination=gcr.io/abayer/jx-demo-qs:${inputs.params.version}"),
									sht.StepArg("--cache-repo=gcr.io//cache")),
								sht.StageStep(sht.StepCmd("jx step post build --image $DOCKER_REGISTRY/$ORG/$APP_NAME:${VERSION}"),
									sht.StepDir("/workspace/source"), sht.StepImage("maven"), sht.StepName("build-post-build")),
								sht.StageStep(sht.StepCmd("echo goodbye"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("build-step8")),
								sht.StageStep(sht.StepCmd("echo wait why am i here"), sht.StepImage("maven"), sht.StepDir("/workspace/source"),
									sht.StepName("build-step9")),
								sht.StageStep(sht.StepCmd("jx step changelog --version v${VERSION}"),
									sht.StepDir("/workspace/source/charts/jx-demo-qs"), sht.StepImage("maven"),
									sht.StepName("promote-changelog")),
								sht.StageStep(sht.StepCmd("jx step helm release"), sht.StepDir("/workspace/source/charts/jx-demo-qs"),
									sht.StepImage("maven"), sht.StepName("promote-helm-release")),
								sht.StageStep(sht.StepCmd("jx promote -b --all-auto --timeout 1h --version ${VERSION}"),
									sht.StepDir("/workspace/source/charts/jx-demo-qs"),
									sht.StepImage("maven"), sht.StepName("promote-jx-promote")),
							),
						),
						SetVersion: &jenkinsfile.PipelineLifecycle{
							Steps: []*jxsyntax.Step{{
								Image: "maven",
								Steps: []*jxsyntax.Step{{
									Comment: "so we can retrieve the version in later steps",
									Name:    "next-version",
									Sh:      "echo \\$(jx-release-version) > VERSION",
									Steps:   []*jxsyntax.Step{},
								}, {
									Name:  "set-version",
									Sh:    "mvn versions:set -DnewVersion=\\$(cat VERSION)",
									Steps: []*jxsyntax.Step{},
								}, {
									Name:  "tag-version",
									Sh:    "jx step tag --version \\$(cat VERSION)",
									Steps: []*jxsyntax.Step{},
								}},
							}},
							PreSteps: []*jxsyntax.Step{},
						},
					},
				},
			},
		},
	}, {
		name: "overrides-with-buildpack-using-jenkins-x-syntax",
		pack: "jx-syntax-in-buildpack",
		// Upper-casing to verify fix for #6983
		repoName:     "Jx-demo-qs",
		organization: "abayer",
		branch:       "master",
		expected: &config.ProjectConfig{
			BuildPack: "jx-syntax-in-buildpack",
			PipelineConfig: &jenkinsfile.PipelineConfig{
				Agent: &jxsyntax.Agent{
					Image: "maven",
					Label: "jenkins-maven",
				},
				Env: []corev1.EnvVar{},
				Pipelines: jenkinsfile.Pipelines{
					Overrides: []*jxsyntax.PipelineOverride{{
						Pipeline: "release",
						Stage:    "build",
						Step: &jxsyntax.Step{
							Command: "echo hi there",
							Name:    "hi-there",
						},
						Name: "flake8",
						Type: &overrideAfter,
					}},
					PullRequest: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("maven"),
							sht.PipelineStage("build",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("source /root/.bashrc && flake8"), sht.StepImage("maven"),
									sht.StepDir("/workspace/source/jx-demo-qs"), sht.StepName("flake8")),
							),
						),
					},
					Release: &jenkinsfile.PipelineLifecycles{
						Pipeline: sht.ParsedPipeline(
							sht.PipelineAgent("maven"),
							sht.PipelineStage("build",
								sht.StageDir("/workspace/source"),
								sht.StageStep(sht.StepCmd("source /root/.bashrc && flake8"), sht.StepImage("maven"), sht.StepDir("/workspace/source/jx-demo-qs"),
									sht.StepName("flake8")),
								sht.StageStep(sht.StepCmd("echo hi there"), sht.StepName("hi-there")),
							),
							sht.PipelineStage("promote",
								sht.StageDir("/workspace/source/charts/jx-demo-qs"),
								sht.StageStep(sht.StepCmd("jx step changelog --version v${inputs.params.version}"),
									sht.StepName("changelog")),
								sht.StageStep(sht.StepCmd("jx step helm release"),
									sht.StepName("helm-release")),
								sht.StageStep(sht.StepCmd("jx promote -b --all-auto --timeout 1h --version ${inputs.params.version}"),
									sht.StepName("jx-promote"),
									sht.StepDir("/workspace/source"))),
						),
					},
				},
			},
		},
	}}

	k8sObjects := []runtime.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kube.ConfigMapJenkinsDockerRegistry,
				Namespace: "jx",
			},
			Data: map[string]string{
				"docker.registry": "gcr.io",
			},
		},
		// Dummy secrets created for validation purposes
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "jenkins-docker-cfg",
				Namespace: "jx",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "jenkins-maven-settings",
				Namespace: "jx",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "jenkins-release-gpg",
				Namespace: "jx",
			},
		},
	}
	jxObjects := []runtime.Object{}
	repoOwnerUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	repoOwner := repoOwnerUUID.String()
	repoNameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	repoName := repoNameUUID.String()
	fakeRepo, _ := gits.NewFakeRepository(repoOwner, repoName, nil, nil)
	fakeGitProvider := gits.NewFakeProvider(fakeRepo)

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {

			caseDir := path.Join(testData, tt.name)
			_, err = os.Stat(caseDir)
			assert.NoError(t, err)

			projectConfig, projectConfigFile, err := config.LoadProjectConfig(caseDir)
			if err != nil {
				t.Fatalf("Error loading %s/jenkins-x.yml: %s", caseDir, err)
			}

			createCanonical := &syntax.StepSyntaxEffectiveOptions{
				Pack:         tt.pack,
				CustomEnvs:   tt.customEnvs,
				DefaultImage: "maven",
				KanikoImage:  "gcr.io/kaniko-project/executor:v0.19.0",
				UseKaniko:    true,
				PodTemplates: assertLoadPodTemplates(t),
				GitInfo: &gits.GitRepository{
					Host:         "github.com",
					Name:         tt.repoName,
					Organisation: tt.organization,
				},
				VersionResolver: &versionstream.VersionResolver{
					VersionsDir: testVersionsDir,
				},
				SourceName: "source",
				StepOptions: step.StepOptions{
					CommonOptions: &opts.CommonOptions{
						ServiceAccount: "tekton-bot",
					},
				},
				ValidateInCluster: true,
			}
			testhelpers.ConfigureTestOptionsWithResources(createCanonical.CommonOptions, k8sObjects, jxObjects, gits_test.NewMockGitter(), fakeGitProvider, helm_test.NewMockHelmer(), nil)

			newConfig, err := createCanonical.CreateEffectivePipeline(packsDir, projectConfig, projectConfigFile, resolver)
			if err != nil {
				t.Fatalf("Error creating canonical pipeline: %s", err)
			}
			if d, _ := kmp.SafeDiff(tt.expected, newConfig, cmpopts.IgnoreFields(corev1.ResourceRequirements{}, "Requests")); d != "" {
				t.Errorf("Generated canonical pipeline does not match expected:\n%s", d)
			}
		})
	}
}

func assertLoadPodTemplates(t *testing.T) map[string]*corev1.Pod {
	fileName := filepath.Join("..", "create", "test_data", "step_create_task", "PodTemplates.yml")
	if tests.AssertFileExists(t, fileName) {
		configMap := &corev1.ConfigMap{}
		data, err := ioutil.ReadFile(fileName)
		if assert.NoError(t, err, "Failed to load file %s", fileName) {
			err = yaml.Unmarshal(data, configMap)
			if assert.NoError(t, err, "Failed to unmarshall YAML file %s", fileName) {
				podTemplates := make(map[string]*corev1.Pod)
				for k, v := range configMap.Data {
					pod := &corev1.Pod{}
					if v != "" {
						err := yaml.Unmarshal([]byte(v), pod)
						if assert.NoError(t, err, "Failed to parse pod template") {
							podTemplates[k] = pod
						}
					}
				}
				return podTemplates
			}
		}
	}
	return nil
}
