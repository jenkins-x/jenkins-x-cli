// +build unit

package metapipeline

import (
	"os"
	"path/filepath"
	"testing"

	jxv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

type testTypeAndPullRef struct {
	pipelineKind PipelineKind
	pullRef      PullRef
	expected     string
	errorMessage string
}

func Test_determine_branch_identifier_from_pull_refs(t *testing.T) {
	testCases := []testTypeAndPullRef{
		{
			ReleasePipeline, NewPullRefWithPullRequest("http://foo", "master", "0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815", PullRequestRef{}), "master", "",
		},
		{
			PullRequestPipeline, NewPullRefWithPullRequest("http://foo", "master", "0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815", PullRequestRef{ID: "4554", MergeSHA: "1c313425db5b014271d0d074dd5aac635ffc617e"}), "PR-4554", "",
		},
		{
			PullRequestPipeline, NewPullRef("http://foo", "master", "0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815"), "", "pullrequest pipeline requested, but no pull requests specified",
		},
	}

	clientFactory := clientFactory{}

	for _, testCase := range testCases {
		actualBranchIdentifier, err := clientFactory.determineBranchIdentifier(testCase.pipelineKind, testCase.pullRef)
		if testCase.errorMessage == "" {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
			assert.Equal(t, testCase.errorMessage, err.Error())
		}
		assert.Equal(t, testCase.expected, actualBranchIdentifier)
	}
}

func Test_default_version_stream_and_url(t *testing.T) {
	ns := "jx"
	jxClient := fake.NewSimpleClientset()

	url, ref, err := versionStreamURLAndRef(jxClient, ns)
	assert.NoError(t, err)
	assert.Equal(t, config.DefaultVersionsURL, url)
	assert.Equal(t, config.DefaultVersionsRef, ref)
}

func Test_version_stream_and_url_from_team_setting(t *testing.T) {
	var jxObjects []runtime.Object
	expectedURL := "https://github.com/jenkins-x/my-jenkins-x-versions.git"
	expectedVersion := "v1.0.0"

	ns := "jx"
	devEnv := kube.NewPermanentEnvironment("dev")
	devEnv.Spec.Namespace = ns
	devEnv.Spec.Kind = jxv1.EnvironmentKindTypeDevelopment
	devEnv.Spec.TeamSettings.VersionStreamURL = expectedURL
	devEnv.Spec.TeamSettings.VersionStreamRef = expectedVersion

	jxObjects = append(jxObjects, devEnv)
	jxClient := fake.NewSimpleClientset(jxObjects...)

	url, ref, err := versionStreamURLAndRef(jxClient, ns)
	assert.NoError(t, err)
	assert.Equal(t, expectedURL, url)
	assert.Equal(t, expectedVersion, ref)
}

func Test_clone_version_stream_from_tag(t *testing.T) {
	ref := "v1.0.8"
	dir, err := cloneVersionStream("https://github.com/jenkins-x/jenkins-x-versions.git", ref)
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	assert.NoError(t, err)
	assert.DirExists(t, dir)

	args := []string{"describe", "--tags", "--abbrev=0"}
	cmd := util.Command{
		Dir:  dir,
		Name: "git",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	assert.NoError(t, err)
	assert.Equal(t, ref, output)
}

func Test_clone_version_stream_from_sha(t *testing.T) {
	// This ref is the HEAD on https://github.com/jenkins-x/jenkins-x-versions/pull/417, which is closed and won't change.
	ref := "72d36667196e2bfbb52b8220d55ef79747283a5b"
	dir, err := cloneVersionStream("https://github.com/jenkins-x/jenkins-x-versions.git", ref)
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	assert.NoError(t, err)
	assert.DirExists(t, dir)

	args := []string{"rev-parse", "HEAD"}
	cmd := util.Command{
		Dir:  dir,
		Name: "git",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	assert.NoError(t, err)
	assert.Equal(t, ref, output)
}

func Test_verify_pull_refs_on_activity(t *testing.T) {
	branchIdentifier := "master"
	buildNumber := "1"
	gitInfo, _ := gits.NewGitFake().Info("/acme")
	pullRef := NewPullRefWithPullRequest("https://github.com/jenkins-x/jx", "master", "0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815", PullRequestRef{ID: "42", MergeSHA: "db8e2d275df53477b1c6871f7d7f4281dacf3169"})

	testParams := CRDCreationParameters{
		PipelineName:     "test-pipeline",
		PipelineKind:     PullRequestPipeline,
		BranchIdentifier: branchIdentifier,
		PullRef:          pullRef,
		GitInfo:          *gitInfo,
		Labels:           map[string]string{"someLabel": "someValue"},
		EnvVars:          map[string]string{"SOME_VAR": "SOME_VAL", "OTHER_VAR": "OTHER_VAL"},
		BuildNumber:      buildNumber,
		SourceDir:        "source",
		ServiceAccount:   "tekton-bot",
		VersionsDir:      filepath.Join("test_data", "stable_versions"),
	}

	clientFactory := clientFactory{}

	activityKey, _, err := clientFactory.createActualCRDs(buildNumber, branchIdentifier, "", pullRef, testParams)
	assert.NoError(t, err)
	assert.NotNil(t, activityKey)
	assert.NotNil(t, activityKey.PullRefs)

	for _, prRef := range pullRef.pullRequests {
		assert.Equal(t, prRef.MergeSHA, activityKey.PullRefs[prRef.ID])
	}
}
