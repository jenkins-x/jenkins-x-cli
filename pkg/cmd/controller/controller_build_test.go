// +build unit

package controller

import (
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/tekton"
	"github.com/jenkins-x/jx/pkg/tekton/tekton_helpers_test"
	"github.com/stretchr/testify/assert"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDigitSuffix(t *testing.T) {
	testData := map[string]string{
		"nosuffix": "",
		"build1":   "1",
		"build123": "123",
	}

	for input, expected := range testData {
		actual := DigitSuffix(input)
		assert.Equal(t, expected, actual, "digitSuffix for %s", input)
	}
}

func TestCompleteBuildSourceInfo(t *testing.T) {
	o := &ControllerBuildOptions{
		gitHubProvider: gits.NewFakeProvider(getFakeRepository()),
		Namespace:      "test",

		ControllerOptions: ControllerOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}

	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{
			getGitHubSecret(),
		},
		[]runtime.Object{},
		nil,
		nil,
		nil,
		nil,
	)

	act := &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-org-my-repo-master-1",
		},
		Spec: v1.PipelineActivitySpec{
			GitURL:    "https://github.com/my-org/my-repo.git",
			GitBranch: "master",
		},
	}

	o.completeBuildSourceInfo(act)

	assert.Equal(t, act.Spec.Author, "john.doe")
	assert.Equal(t, act.Spec.LastCommitMessage, "This is a commit message")

	act = &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-org-my-repo-pr-4",
		},
		Spec: v1.PipelineActivitySpec{
			GitURL:    "https://github.com/my-org/my-repo.git",
			GitBranch: "PR-4",
		},
	}

	o.completeBuildSourceInfo(act)

	assert.Equal(t, act.Spec.Author, "john.doe.pr")
	assert.Equal(t, act.Spec.PullTitle, "This is the PR title")
}

func TestUpdateForStage(t *testing.T) {
	pod := tekton_helpers_test.AssertLoadSinglePod(t, path.Join("test_data", "controller_build", "update_stage_info"))
	si := &tekton.StageInfo{
		Name:           "ci",
		CreatedTime:    *parseTime(t, "2019-06-07T18:14:19-00:00"),
		FirstStepImage: "gcr.io/abayer-jx-experiment/creds-init:v20190508-91b53326",
		PodName:        "jenkins-x-jx-pr-4135-integratio-42-ci-kdjkp-pod-d964e6",
		TaskRun:        "jenkins-x-jx-pr-4135-integratio-42-ci-kdjkp",
		Task:           "jenkins-x-jx-pr-4135-integratio-ci-42",
		Parents:        []string{},
		Pod:            pod,
	}

	act := &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "jenkins-x-jx-pr-4135-42",
		},
		Spec: v1.PipelineActivitySpec{
			GitURL:    "https://github.com/jenkins-x/jx",
			GitBranch: "PR-4135",
		},
	}

	updateForStage(si, act)

	containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)

	assert.Len(t, act.Spec.Steps, 1, "No steps/stages found on activity")
	assert.NotNil(t, act.Spec.Steps[0].Stage, "First step on activity is not a stage")

	stage := act.Spec.Steps[0].Stage
	assert.Equal(t, len(containers), len(stage.Steps), "%d containers found in pod, but %d steps found in stage", len(containers), len(stage.Steps))

	for i, c := range containers {
		step := stage.Steps[i]
		name := strings.Replace(strings.TrimPrefix(c.Name, "step-"), "-", " ", -1)
		title := strings.Title(name)
		if title != step.Name {
			// Using t.Errorf instead of an assert here because we want to see all the misordered names, not just the first one.
			t.Errorf("For step %d, expected step with name %s, but found %s", i, title, step.Name)
		}
		desc := createStepDescription(c.Name, pod)
		if desc != step.Description {
			// Using t.Errorf instead of an assert here because we want to see all the wrong descriptions, not just the first one.
			t.Errorf("For step %d, expected step with description %s, but found %s", i, desc, step.Description)
		}
		if i > 0 {
			previousStep := stage.Steps[i-1]
			assert.False(t, step.StartedTimestamp.IsZero(), "step %s has a nil start time", step.Name)
			assert.Equal(t, previousStep.CompletedTimestamp, step.StartedTimestamp, "step %s has start time %s but should have start time %s", step.Name, previousStep.CompletedTimestamp, step.StartedTimestamp)
		}
	}
}

func TestCreateReportTargetURL(t *testing.T) {
	params := ReportParams{
		Owner:      "jstrachan",
		Repository: "myapp",
		Branch:     "PR-5",
		Build:      "3",
		Context:    "jenkins-x",
		Namespace:  "jx",
		BaseURL:    "https://myconsole.acme.com",
	}
	actual := CreateReportTargetURL(defaultTargetURLTemplate, params)
	assert.Equal(t, "https://myconsole.acme.com/teams/jx/projects/jstrachan/myapp/PR-5/3", actual, "created git report URL for params %#v", params)
}

func TestUpdateForStagePreTekton051(t *testing.T) {
	pod := tekton_helpers_test.AssertLoadSinglePod(t, path.Join("test_data", "controller_build", "update_stage_info_pre_tekton_0.5.1"))
	si := &tekton.StageInfo{
		Name:           "ci",
		CreatedTime:    *parseTime(t, "2019-06-07T18:14:19-00:00"),
		FirstStepImage: "gcr.io/abayer-jx-experiment/creds-init:v20190508-91b53326",
		PodName:        "jenkins-x-jx-pr-4135-integratio-42-ci-kdjkp-pod-d964e6",
		TaskRun:        "jenkins-x-jx-pr-4135-integratio-42-ci-kdjkp",
		Task:           "jenkins-x-jx-pr-4135-integratio-ci-42",
		Parents:        []string{},
		Pod:            pod,
	}

	act := &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "jenkins-x-jx-pr-4135-42",
		},
		Spec: v1.PipelineActivitySpec{
			GitURL:    "https://github.com/jenkins-x/jx",
			GitBranch: "PR-4135",
		},
	}

	updateForStage(si, act)

	containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)

	assert.Len(t, act.Spec.Steps, 1, "No steps/stages found on activity")
	assert.NotNil(t, act.Spec.Steps[0].Stage, "First step on activity is not a stage")

	stage := act.Spec.Steps[0].Stage

	assert.Equal(t, len(containers), len(stage.Steps), "%d containers found in pod, but %d steps found in stage", len(containers), len(stage.Steps))

	for i, c := range containers {
		step := stage.Steps[i]
		name := strings.Replace(c.Name, "-", " ", -1)
		title := strings.Title(name)
		if title != step.Name {
			// Using t.Errorf instead of an assert here because we want to see all the misordered names, not just the first one.
			t.Errorf("For step %d, expected step with name %s, but found %s", i, title, step.Name)
		}
	}
}

func TestOnPipelinePod(t *testing.T) {
	testCases := []struct {
		name    string
		podName string
	}{{
		name:    "completed_metapipeline",
		podName: "meta-cb-kubecd-bdd-spring-15681-1-meta-pipeline-rwb55-pod-e44008",
	}, {
		name:    "failed_metapipeline",
		podName: "meta-cb-kubecd-bdd-spring-15681-1-meta-pipeline-rwb55-pod-e44008",
	}, {
		name:    "completed_generated",
		podName: "cb-kubecd-bdd-spring-1568135191-1-from-build-pack-mrz2r-pod-8d000a",
	}, {
		name:    "running_generated",
		podName: "cb-kubecd-bdd-spring-1568135191-1-from-build-pack-mrz2r-pod-8d000a",
	}, {
		name:    "failed_multistage_generated",
		podName: "cb-kubecd-bdd-spring-1568135191-1-from-build-pack-mrz2r-pod-8d000a",
	}}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			testCaseDir := path.Join("test_data", "controller_build", tt.name)
			stateDir := path.Join(testCaseDir, "cluster_state")
			expectedDir := path.Join(testCaseDir, "expected")

			originalActivity := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, stateDir)
			structures := tekton_helpers_test.AssertLoadPipelineStructures(t, stateDir)

			tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadPipelineRuns(t, stateDir), tekton_helpers_test.AssertLoadPipelines(t, stateDir)}
			tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

			podList := tekton_helpers_test.AssertLoadPods(t, stateDir)

			o := &ControllerBuildOptions{
				Namespace:          "jx",
				InitGitCredentials: false,
				GitReporting:       false,
				DryRun:             true,
				ControllerOptions: ControllerOptions{
					CommonOptions: &opts.CommonOptions{},
				},
			}

			testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
				[]runtime.Object{
					getGitHubSecret(),
					podList,
				},
				[]runtime.Object{
					originalActivity,
					structures,
				},
				nil,
				nil,
				nil,
				nil,
			)
			kubeClient, err := o.KubeClient()
			assert.NoError(t, err)
			jxClient, ns, err := o.JXClient()
			assert.NoError(t, err)

			inputPod, err := kubeClient.CoreV1().Pods(ns).Get(tt.podName, metav1.GetOptions{})
			assert.NoError(t, err)

			o.onPipelinePod(inputPod, kubeClient, jxClient, tektonClient, ns)

			foundActivity, err := jxClient.JenkinsV1().PipelineActivities(ns).Get(originalActivity.Name, metav1.GetOptions{})
			assert.NoError(t, err)

			expectedActivity := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, expectedDir)

			d := cmp.Diff(expectedActivity, foundActivity)
			assert.Empty(t, d, "Expected PipelineActivitys to be equal, but diff was %s", d)
		})
	}
}

func getGitHubSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-pipeline-git-github-github",
			Namespace: "test",
			Labels: map[string]string{
				kube.LabelCredentialsType: kube.ValueCredentialTypeUsernamePassword,
				kube.LabelCreatedBy:       kube.ValueCreatedByJX,
				kube.LabelKind:            "git",
				kube.LabelServiceKind:     "github",
			},
			Annotations: map[string]string{
				kube.AnnotationCredentialsDescription: fmt.Sprintf("Configuration and credentials for server https://github.com"),
				kube.AnnotationURL:                    "https://github.com",
				kube.AnnotationName:                   "GitHub",
			},
		},
		Data: map[string][]byte{
			"username": []byte("test1"),
			"password": []byte("test1"),
		},
	}
}

func getFakeRepository() *gits.FakeRepository {
	return &gits.FakeRepository{
		Owner: "my-org",
		GitRepo: &gits.GitRepository{
			Name:         "my-repo",
			Organisation: "my-org",
		},
		Commits: []*gits.FakeCommit{
			{
				Status: gits.CommitSatusSuccess,
				Commit: &gits.GitCommit{
					SHA:     "ba3ae9923ecc1264ccaa668317fb9276d8b1b5ff",
					Message: "This is a commit message",
					Author: &gits.GitUser{
						Login: "john.doe",
					},
				},
			},
		},
		PullRequests: map[int]*gits.FakePullRequest{
			4: {
				PullRequest: &gits.GitPullRequest{
					Author: &gits.GitUser{
						Login: "john.doe.pr",
					},
					Title: "This is the PR title",
				},
			},
		},
	}
}

func parseTime(t *testing.T, timeString string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, timeString)
	if assert.NoError(t, err, "Failed to parse date %s", timeString) {
		return &parsed
	}
	return nil
}
