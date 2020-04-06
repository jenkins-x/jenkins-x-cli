// +build integration

package post_test

import (
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/post"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/jenkins/fake"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/jenkins-x/jx/pkg/testkube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStepPostInstall(t *testing.T) {
	originalJxHome, tempJxHome, err := testhelpers.CreateTestJxHomeDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestJxHomeDir(originalJxHome, tempJxHome)
		assert.NoError(t, err)
	}()
	originalKubeCfg, tempKubeCfg, err := testhelpers.CreateTestKubeConfigDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestKubeConfigDir(originalKubeCfg, tempKubeCfg)
		assert.NoError(t, err)
	}()

	dev := kube.CreateDefaultDevEnvironment("jx")
	testOrg := "mytestorg"
	testRepo := "mytestrepo"
	stagingGitURL := "https://fake.git/" + testOrg + "/" + testRepo + ".git"
	staging := kube.NewPermanentEnvironmentWithGit("staging", stagingGitURL)

	o := post.StepPostInstallOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			},
		},
	}
	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kube.ConfigMapJenkinsX,
					Namespace: "jx",
				},
				Data: map[string]string{},
			},
			testkube.CreateFakeGitSecret(),
		},
		[]runtime.Object{
			dev,
			staging,
		},
		gits.NewGitCLI(),
		nil,
		helm.NewHelmCLI("helm", helm.V2, "", true),
		resources_test.NewMockInstaller(),
	)

	o.BatchMode = true
	jenkinsClient := fake.NewFakeJenkins()
	o.SetJenkinsClient(jenkinsClient)
	o.SetGit(&gits.GitFake{})

	err = o.Run()
	require.NoError(t, err, "failed to run jx step post install")

	// assert we have a jenkins job for the staging env repo
	AssertJenkinsJobExists(t, jenkinsClient, testOrg, testRepo)

	// assert we have a webhook for the staging env repo
	gitProvider := o.Results.GitProviders["staging"]
	require.NotNil(t, gitProvider, "no GitProvider is registered for staging")

	webhooks, err := gitProvider.ListWebHooks(testOrg, testRepo)
	require.NoError(t, err, "failed to list webhooks for staging git repository %s", stagingGitURL)

	t.Logf("found %d webhooks\n", len(webhooks))

	assert.True(t, len(webhooks) > 0, "should have at least 1 WebHook for staging git repository %s", stagingGitURL)

	found := false
	for _, webhook := range webhooks {
		repo := webhook.Repo
		if repo != nil && repo.Organisation == testOrg && repo.Name == testRepo {
			t.Logf("found WebHook for staging git repository %s: %#v", stagingGitURL, webhook)
			found = true
		}
	}
	assert.True(t, found, "did not find WebHook for staging git repository %s", stagingGitURL)
}

// AssertJenkinsJobExists asserts that the job exists for the given organisation and repo
func AssertJenkinsJobExists(t *testing.T, jenkinsClient *fake.FakeJenkins, testOrg string, testRepo string) {
	job, err := jenkinsClient.GetJobByPath(testOrg, testRepo)
	if !assert.NoError(t, err, "failed to query Jenkins Job for %s/%s", testOrg, testRepo) {
		DumpJenkinsJobs(t, jenkinsClient)
		return
	}
	if !assert.Equal(t, job.Name, testRepo, "job.Name") {
		DumpJenkinsJobs(t, jenkinsClient)
		return
	}

	t.Logf("Found Jenkins Job at URL: %s\n", job.Url)
}

// DumpJenkinsJobs dumps the current jenkins jobs in the given client to aid debugging a failing test
func DumpJenkinsJobs(t *testing.T, jenkinsClient gojenkins.JenkinsClient) {
	jobs, err := jenkinsClient.GetJobs()
	require.NoError(t, err, "failed to get jobs")

	for _, job := range jobs {
		t.Logf("Jenkins Job: %s at %s\n", job.Name, job.Url)
		for _, cj := range job.Jobs {
			t.Logf("\t child Job: %s at %s\n", cj.Name, cj.Url)
		}
	}
}
