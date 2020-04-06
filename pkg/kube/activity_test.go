// +build unit

package kube_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	jenkinsio_v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	jxfake "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	appsv1 "k8s.io/api/apps/v1"
	k8s_v1 "k8s.io/api/core/v1"

	typev1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube_mocks "k8s.io/client-go/kubernetes/fake"
)

func TestGenerateBuildNumber(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(clients.NewFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	jxClient, ns, err := options.JXClientAndDevNamespace()
	assert.NoError(t, err, "Creating JX client")
	if err != nil {
		return
	}

	activities := jxClient.JenkinsV1().PipelineActivities(ns)

	org := "jstrachan"
	repo := "cheese"
	branch := "master"

	results := []string{}
	expected := []string{}
	for i := 1; i < 4; i++ {
		buildNumberText := strconv.Itoa(i)
		pID := kube.NewPipelineID(repo, org, branch)
		pipelines := getPipelines(activities)
		build, activity, err := kube.GenerateBuildNumber(activities, pipelines, pID)
		if assert.NoError(t, err, "GenerateBuildNumber %d", i) {
			if assert.NotNil(t, activity, "No PipelineActivity returned!") {
				results = append(results, build)
				assert.Equal(t, buildNumberText, activity.Spec.Build, "Build number for PipelineActivity %s", activity.Name)
			}
		}
		expected = append(expected, buildNumberText)
	}
	assert.Equal(t, expected, results, "generated build numbers")
}

func getPipelines(activities typev1.PipelineActivityInterface) []*v1.PipelineActivity {
	pipelineList, _ := activities.List(metav1.ListOptions{})
	pipelines := []*v1.PipelineActivity{}
	for _, pipeline := range pipelineList.Items {
		copy := pipeline
		pipelines = append(pipelines, &copy)
	}
	return pipelines
}

func TestCreateOrUpdateActivities(t *testing.T) {
	t.Parallel()

	nsObj := &k8s_v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "testing_ns",
		},
	}

	secret := &k8s_v1.Secret{}
	mockKubeClient := kube_mocks.NewSimpleClientset(nsObj, secret)

	ingressConfig := &k8s_v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: kube.ConfigMapIngressConfig,
		},
		Data: map[string]string{"key1": "value1", "domain": "test-domain", "config.yml": ""},
	}

	mockKubeClient.CoreV1().ConfigMaps(nsObj.Namespace).Create(ingressConfig)
	mockTektonDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: kube.DeploymentTektonController,
		},
	}
	mockKubeClient.AppsV1().Deployments(nsObj.Namespace).Create(mockTektonDeployment)
	jxClient := jxfake.NewSimpleClientset()

	const (
		expectedName         = "demo-2"
		expectedBuild        = "2"
		expectedEnvironment  = "staging"
		expectedOrganisation = "test-org"
	)
	expectedPipeline := expectedOrganisation + "/" + expectedName + "/master"

	key := kube.PipelineActivityKey{
		Name:     expectedName,
		Pipeline: expectedPipeline,
		Build:    expectedBuild,
		GitInfo: &gits.GitRepository{
			Name:         expectedName,
			Organisation: expectedOrganisation,
			URL:          "https://github.com/" + expectedOrganisation + "/" + expectedName,
		},
	}

	for i := 1; i < 3; i++ {
		a, _, err := key.GetOrCreate(jxClient, nsObj.Namespace)
		assert.Nil(t, err)
		assert.Equal(t, expectedName, a.Name)
		spec := &a.Spec
		assert.Equal(t, expectedPipeline, spec.Pipeline)
		assert.Equal(t, expectedBuild, spec.Build)
	}

	// lazy add a PromotePullRequest
	promoteKey := kube.PromoteStepActivityKey{
		PipelineActivityKey: key,
		Environment:         expectedEnvironment,
	}

	promotePullRequestStarted := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
		assert.NotNil(t, a)
		assert.NotNil(t, p)
		if p.StartedTimestamp == nil {
			p.StartedTimestamp = &metav1.Time{
				Time: time.Now(),
			}
		}
		return nil
	}

	err := promoteKey.OnPromotePullRequest(mockKubeClient, jxClient, nsObj.Namespace, promotePullRequestStarted)
	assert.Nil(t, err)

	promoteStarted := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromoteUpdateStep) error {
		assert.NotNil(t, a)
		assert.NotNil(t, p)
		kube.CompletePromotionUpdate(a, s, ps, p)
		return nil
	}

	err = promoteKey.OnPromoteUpdate(mockKubeClient, jxClient, nsObj.Namespace, promoteStarted)
	assert.Nil(t, err)

	// lets validate that we added a PromotePullRequest step
	a, err := jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Get(expectedName, metav1.GetOptions{})
	assert.NotNil(t, a, "should have a PipelineActivity for %s", expectedName)
	steps := a.Spec.Steps
	assert.Equal(t, 2, len(steps), "Should have 2 steps!")
	step := a.Spec.Steps[0]
	stage := step.Stage
	assert.NotNil(t, stage, "step 0 should have a Stage")
	assert.Equal(t, v1.ActivityStepKindTypeStage, step.Kind, "step - kind")
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, stage.Status, "step 0 Stage status")
	assert.NotNil(t, stage.StartedTimestamp, "stage should have a StartedTimestamp")
	assert.NotNil(t, stage.CompletedTimestamp, "stage should have a CompletedTimestamp")

	step = a.Spec.Steps[1]
	promote := step.Promote
	assert.NotNil(t, promote, "step 1 should have a Promote")
	assert.Equal(t, v1.ActivityStepKindTypePromote, step.Kind, "step 1 kind")

	pullRequestStep := promote.PullRequest
	assert.NotNil(t, pullRequestStep, "Promote should have a PullRequest")
	assert.NotNil(t, pullRequestStep.StartedTimestamp, "Promote should have a PullRequest.StartedTimestamp")
	assert.NotNil(t, pullRequestStep.CompletedTimestamp, "Promote should not have a PullRequest.CompletedTimestamp")

	updateStep := promote.Update
	assert.NotNil(t, updateStep, "Promote should have an Update")
	assert.NotNil(t, updateStep.StartedTimestamp, "Promote should have a Update.StartedTimestamp")
	assert.NotNil(t, updateStep.CompletedTimestamp, "Promote should have a Update.CompletedTimestamp")

	assert.NotNil(t, promote.StartedTimestamp, "promote should have a StartedTimestamp")
	assert.NotNil(t, promote.CompletedTimestamp, "promote should have a CompletedTimestamp")

	assert.Equal(t, v1.ActivityStatusTypeSucceeded, pullRequestStep.Status, "pullRequestStep status")
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, updateStep.Status, "updateStep status")
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, promote.Status, "promote status")
	assert.Equal(t, v1.ActivityStatusTypeSucceeded, a.Spec.Status, "activity status")

	//tests.Debugf("Has Promote %#v\n", promote)
}

func TestCreateOrUpdateActivityForBatchBuild(t *testing.T) {
	t.Parallel()

	nsObj := &k8s_v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "testing_ns",
		},
	}

	jxClient := jxfake.NewSimpleClientset()

	const (
		expectedName         = "demo-2"
		expectedBuild        = "2"
		expectedOrganisation = "test-org"
	)
	expectedPipeline := expectedOrganisation + "/" + expectedName + "/master"

	key := kube.PipelineActivityKey{
		Name:     expectedName,
		Pipeline: expectedPipeline,
		Build:    expectedBuild,
		GitInfo: &gits.GitRepository{
			Name:         expectedName,
			Organisation: expectedOrganisation,
			URL:          "https://github.com/" + expectedOrganisation + "/" + expectedName,
		},
		PullRefs: map[string]string{
			"1": "sha1",
			"2": "sha2",
		},
	}

	_, err := jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "PA0",
			Labels: map[string]string{
				"lastCommitSha": "sha1",
				"branch":        "PR-1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Build: "1",
		},
	})
	assert.NoError(t, err)

	// lets create a build PA for the same PR but with a different SHA so we can check we discard it later
	_, err = jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "PA0-2",
			Labels: map[string]string{
				"lastCommitSha": "sha3",
				"branch":        "PR-1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Build: "5",
		},
	})
	assert.NoError(t, err)

	//lets create a few "builds" for PR-2 with the same SHA so we can check if we choose the right one
	for i := 1; i < 4; i++ {
		_, err = jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Create(&v1.PipelineActivity{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("PA%d", i),
				Labels: map[string]string{
					"lastCommitSha": "sha2",
					"branch":        "PR-2",
				},
			},
			Spec: v1.PipelineActivitySpec{
				Build: strconv.Itoa(i),
			},
		})
		assert.NoError(t, err)
	}

	a, _, err := key.GetOrCreate(jxClient, nsObj.Namespace)
	assert.Nil(t, err)
	assert.Equal(t, expectedName, a.Name)
	spec := &a.Spec
	assert.Equal(t, expectedPipeline, spec.Pipeline)
	assert.Equal(t, expectedBuild, spec.Build)

	pa1, err := jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Get("PA0", metav1.GetOptions{})
	assert.NoError(t, err)

	pa3, err := jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Get("PA3", metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Len(t, a.Spec.BatchPipelineActivity.ComprisingPulLRequests, 2, "There should be %d PRs information in the ComprisingPullRequests property", 2)
	exists := false
	for _, i := range a.Spec.BatchPipelineActivity.ComprisingPulLRequests {
		if i.PullRequestNumber == "PR-1" {
			assert.NotEqual(t, "5", i.LastBuildNumberForCommit)
		}
		if i.PullRequestNumber == "PR-2" {
			exists = true
			assert.Equal(t, "3", i.LastBuildNumberForCommit)
			assert.Equal(t, "sha2", i.LastBuildSHA)
		}
	}
	assert.True(t, exists, "There should be a Pull Request called PR-2 within the ComprisingPullRequests property")
	assert.Equal(t, expectedBuild, pa1.Spec.BatchPipelineActivity.BatchBuildNumber, "The batch build number that is going to merge this PR should be %s", expectedBuild)
	assert.Equal(t, expectedBuild, pa3.Spec.BatchPipelineActivity.BatchBuildNumber, "The batch build number that is going to merge this PR should be %s", expectedBuild)
}

func TestCreateOrUpdateActivityForBatchBuildWithoutExistingActivities(t *testing.T) {
	t.Parallel()

	nsObj := &k8s_v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "testing_ns",
		},
	}

	jxClient := jxfake.NewSimpleClientset()

	const (
		expectedName         = "demo-2"
		expectedBuild        = "2"
		expectedOrganisation = "test-org"
	)
	expectedPipeline := expectedOrganisation + "/" + expectedName + "/master"

	key := kube.PipelineActivityKey{
		Name:     expectedName,
		Pipeline: expectedPipeline,
		Build:    expectedBuild,
		GitInfo: &gits.GitRepository{
			Name:         expectedName,
			Organisation: expectedOrganisation,
			URL:          "https://github.com/" + expectedOrganisation + "/" + expectedName,
		},
		PullRefs: map[string]string{
			"1": "sha1",
			"2": "sha2",
		},
	}

	//lets create a few "builds" for PR-2 with the same SHA so we can check if we choose the right one
	for i := 1; i < 4; i++ {
		_, err := jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Create(&v1.PipelineActivity{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("PA%d", i),
				Labels: map[string]string{
					"lastCommitSha": "sha2",
					"branch":        "PR-2",
				},
			},
			Spec: v1.PipelineActivitySpec{
				Build: strconv.Itoa(i),
			},
		})
		assert.NoError(t, err)
	}

	a, _, err := key.GetOrCreate(jxClient, nsObj.Namespace)
	assert.Nil(t, err)
	assert.Equal(t, expectedName, a.Name)
	spec := &a.Spec
	assert.Equal(t, expectedPipeline, spec.Pipeline)
	assert.Equal(t, expectedBuild, spec.Build)

	pa3, err := jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Get("PA3", metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Len(t, a.Spec.BatchPipelineActivity.ComprisingPulLRequests, 1, "There should be %d PRs information in the ComprisingPullRequests property", 1)
	exists := false
	for _, i := range a.Spec.BatchPipelineActivity.ComprisingPulLRequests {
		if i.PullRequestNumber == "PR-2" {
			exists = true
			assert.Equal(t, "3", i.LastBuildNumberForCommit)
		}
	}
	assert.True(t, exists, "There should be a Pull Request called PR-2 within the ComprisingPullRequests property")
	assert.Equal(t, expectedBuild, pa3.Spec.BatchPipelineActivity.BatchBuildNumber, "The batch build number that is going to merge this PR should be %s", expectedBuild)
}

func TestCreateOrUpdatePRActivityWithLastCommitSHA(t *testing.T) {
	t.Parallel()

	nsObj := &k8s_v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "testing_ns",
		},
	}

	jxClient := jxfake.NewSimpleClientset()

	const (
		expectedName         = "demo-2"
		expectedBuild        = "2"
		expectedOrganisation = "test-org"
	)
	expectedPipeline := expectedOrganisation + "/" + expectedName + "/master"

	key := kube.PipelineActivityKey{
		Name:     expectedName,
		Pipeline: expectedPipeline,
		Build:    expectedBuild,
		GitInfo: &gits.GitRepository{
			Name:         expectedName,
			Organisation: expectedOrganisation,
			URL:          "https://github.com/" + expectedOrganisation + "/" + expectedName,
		},
		PullRefs: map[string]string{
			"1": "sha1",
		},
	}

	a, _, err := key.GetOrCreate(jxClient, nsObj.Namespace)
	assert.Nil(t, err)
	assert.Equal(t, expectedName, a.Name)
	spec := &a.Spec
	assert.Equal(t, expectedPipeline, spec.Pipeline)
	assert.Equal(t, expectedBuild, spec.Build)

	assert.Equal(t, "sha1", a.ObjectMeta.Labels[v1.LabelLastCommitSha])
}

func TestBatchReconciliationWithTwoPRBuildExecutions(t *testing.T) {
	t.Parallel()

	nsObj := &k8s_v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-testing",
			Namespace: "testing_ns",
		},
	}

	jxClient := jxfake.NewSimpleClientset()

	const (
		expectedName         = "demo-2"
		expectedBuild        = "2"
		expectedOrganisation = "test-org"
		expectedBatchBuild   = "1"
	)

	prPAName := fmt.Sprintf("%s-%s-pr1-1", expectedOrganisation, expectedName)
	pr1PA := &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: prPAName,
			Labels: map[string]string{
				v1.LabelBranch:        "PR-1",
				v1.LabelLastCommitSha: "sha1",
				v1.LabelBuild:         "1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			GitOwner:      expectedOrganisation,
			GitRepository: expectedName,
			Build:         "1",
			BatchPipelineActivity: v1.BatchPipelineActivity{
				BatchBuildNumber: expectedBatchBuild,
			},
		},
	}

	_, err := jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Create(pr1PA)
	assert.NoError(t, err)

	batchPAName := fmt.Sprintf("%s-%s-batch-1", expectedOrganisation, expectedName)
	batchPA := &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: batchPAName,
			Labels: map[string]string{
				v1.LabelBranch:        "batch",
				v1.LabelLastCommitSha: "testSha",
				v1.LabelBuild:         expectedBatchBuild,
			},
		},
		Spec: v1.PipelineActivitySpec{
			GitOwner:      expectedOrganisation,
			GitRepository: expectedName,
			Build:         expectedBatchBuild,
			BatchPipelineActivity: v1.BatchPipelineActivity{
				ComprisingPulLRequests: []v1.PullRequestInfo{
					{PullRequestNumber: "PR-1", LastBuildNumberForCommit: "1"},
				},
			},
		},
	}

	_, err = jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Create(batchPA)
	assert.NoError(t, err)

	expectedPipeline := expectedOrganisation + "/" + expectedName + "/PR-1"
	key := kube.PipelineActivityKey{
		Name:     expectedName,
		Pipeline: expectedPipeline,
		Build:    expectedBuild,
		GitInfo: &gits.GitRepository{
			Name:         expectedName,
			Organisation: expectedOrganisation,
			URL:          "https://github.com/" + expectedOrganisation + "/" + expectedName,
		},
		PullRefs: map[string]string{
			expectedBuild: "sha1",
		},
	}

	a, _, err := key.GetOrCreate(jxClient, nsObj.Namespace)
	assert.Nil(t, err)
	assert.Equal(t, expectedBatchBuild, a.Spec.BatchPipelineActivity.BatchBuildNumber, "The batch build in the BatchPipeline of the PR should be 1")

	o := metav1.GetOptions{}
	batchPA, err = jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Get(batchPAName, o)
	assert.Equal(t, expectedBuild, batchPA.Spec.BatchPipelineActivity.ComprisingPulLRequests[0].LastBuildNumberForCommit, "The build number for the comprising PR should be 2")

	pr1PA, err = jxClient.JenkinsV1().PipelineActivities(nsObj.Namespace).Get(prPAName, o)
	assert.Empty(t, pr1PA.Spec.BatchPipelineActivity.BatchBuildNumber, "The batch build number for the second PR should be empty")

	assert.Equal(t, expectedBatchBuild, a.Spec.BatchPipelineActivity.BatchBuildNumber, "The batch build number for the second execution of the PR should be 1")
}

func TestCreatePipelineDetails(t *testing.T) {
	expectedGitOwner := "jstrachan"
	expectedGitRepo := "myapp"
	expectedBranch := "master"
	expectedPipeline := expectedGitOwner + "/" + expectedGitRepo + "/" + expectedBranch
	expectedBuild := "3"
	expectedContext := "release"

	pipelines := []*v1.PipelineActivity{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a1",
			},
			Spec: v1.PipelineActivitySpec{
				Pipeline: expectedPipeline,
				Build:    expectedBuild,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a2",
			},
			Spec: v1.PipelineActivitySpec{
				GitOwner:      expectedGitOwner,
				GitRepository: expectedGitRepo,
				Build:         expectedBuild,
				Context:       expectedContext,
			},
		},
	}
	for _, pipeline := range pipelines {
		d1 := kube.CreatePipelineDetails(pipeline)
		name := pipeline.Name
		if assert.NotNil(t, d1, "%s did not create a PipelineDetails", name) {
			assert.Equal(t, expectedGitOwner, d1.GitOwner, "%s GitOwner", name)
			assert.Equal(t, expectedGitRepo, d1.GitRepository, "%s GitRepository", name)
			assert.Equal(t, expectedBranch, d1.BranchName, "%s BranchName", name)
			assert.Equal(t, expectedPipeline, d1.Pipeline, "%s Pipeline", name)
			assert.Equal(t, expectedBuild, d1.Build, "%s Build", name)
			if pipeline.Spec.Context != "" {
				assert.Equal(t, expectedContext, d1.Context, "%s Context", name)
			}
		}
	}
}

func TestPipelineID(t *testing.T) {
	t.Parallel()

	// A simple ID.
	pID := kube.NewPipelineID("o1", "r1", "b1")
	validatePipelineID(t, pID, "o1/r1/b1", "o1-r1-b1")

	// Upper case allowed in our ID, but not in the K8S 'name'.
	pID = kube.NewPipelineID("OwNeR1", "rEpO1", "BrAnCh1")
	validatePipelineID(t, pID, "OwNeR1/rEpO1/BrAnCh1", "owner1-repo1-branch1")

	//Punctuation other than '-' and '.' not allowed in K8S 'name'. Note that this isn't currently handled by the
	//system - the illegal characters are not yet encoded & will be rejected by K8S.
	pID = kube.NewPipelineID("O/N!R@1", "therepo", "thebranch")
	validatePipelineID(t, pID, "O/N!R@1/therepo/thebranch", "o-n!r@1-therepo-thebranch")
}

func TestSortActivities(t *testing.T) {
	t.Parallel()
	date1 := metav1.Date(2009, time.September, 10, 23, 0, 0, 0, time.UTC)
	date2 := metav1.Date(2009, time.October, 10, 23, 0, 0, 0, time.UTC)
	date3 := metav1.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	date4 := metav1.Date(2009, time.December, 10, 23, 0, 0, 0, time.UTC)

	activities := []jenkinsio_v1.PipelineActivity{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a1",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: &date3,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a2",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: &date2,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a3",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: &date1,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a4",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: &date4,
			},
		},
	}

	kube.SortActivities(activities)

	assert.Equal(t, "a3", activities[0].Name, "Activity 0")
	assert.Equal(t, "a2", activities[1].Name, "Activity 1")
	assert.Equal(t, "a1", activities[2].Name, "Activity 2")
	assert.Equal(t, "a4", activities[3].Name, "Activity 3")
}

func TestSortActivitiesWithPendingCases(t *testing.T) {
	t.Parallel()
	date1 := metav1.Date(2009, time.September, 10, 23, 0, 0, 0, time.UTC)
	date2 := metav1.Date(2009, time.October, 10, 23, 0, 0, 0, time.UTC)
	date3 := metav1.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	activities := []jenkinsio_v1.PipelineActivity{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a1",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: &date3,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a2",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: &date2,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a3",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: &date1,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "p",
			},
			Spec: v1.PipelineActivitySpec{
				StartedTimestamp: nil,
			},
		},
	}

	kube.SortActivities(activities)

	assert.Equal(t, "a3", activities[0].Name, "Activity 0")
	assert.Equal(t, "a2", activities[1].Name, "Activity 1")
	assert.Equal(t, "a1", activities[2].Name, "Activity 2")
	assert.Equal(t, "p", activities[3].Name, "Activity 3")
}

func validatePipelineID(t *testing.T, pID kube.PipelineID, expectedID string, expectedName string) {
	assert.Equal(t, expectedID, pID.ID)
	assert.Equal(t, expectedName, pID.Name)
}
