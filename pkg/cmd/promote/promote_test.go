// +build unit

package promote_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"

	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/jenkins-x/jx/pkg/helm"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/controller"
	"github.com/jenkins-x/jx/pkg/cmd/promote"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/petergtz/pegomock"

	"k8s.io/helm/pkg/proto/hapi/chart"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"

	resources_mock "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPromoteToProductionRun(t *testing.T) {

	// prepare the initial setup for testing
	testEnv, err := prepareInitialPromotionEnv(t, true)
	assert.NoError(t, err)

	// jx promote --batch-mode --app my-app --env production --version 1.2.0 --no-helm-update --no-poll

	version := "1.2.0"

	promoteOptions := &promote.PromoteOptions{
		Environment:         "production",                   // --env production
		Application:         "my-app",                       // --app my-app
		Pipeline:            testEnv.Activity.Spec.Pipeline, // needed for the test to pass on CI, otherwise it takes the actual CI build value
		Build:               testEnv.Activity.Spec.Build,    // needed for the test to pass on CI, otherwise it takes the actual CI build value
		Version:             version,                        // --version 1.2.0
		ReleaseName:         "",
		LocalHelmRepoName:   "",
		HelmRepositoryURL:   "",
		NoHelmUpdate:        true, // --no-helm-update
		AllAutomatic:        false,
		NoMergePullRequest:  false,
		NoPoll:              true, // --no-poll
		NoWaitAfterMerge:    false,
		IgnoreLocalFiles:    true,
		Timeout:             "1h",
		PullRequestPollTime: "20s",
		Filter:              "",
		Alias:               "",
		Namespace:           "jx",
	}
	commonOpts := *testEnv.CommonOptions
	promoteOptions.CommonOptions = &commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources
	promoteOptions.BatchMode = true            // --batch-mode

	// Check there is no PR for production env yet
	jxClient, ns, err := promoteOptions.JXClientAndDevNamespace()
	activities := jxClient.JenkinsV1().PipelineActivities(ns)
	testhelpers.AssertHasNoPullRequestForEnv(t, activities, testEnv.Activity.Name, "production")

	// Run the promotion
	err = promoteOptions.Run()
	assert.NoError(t, err)

	// The PR has been created
	testhelpers.AssertHasPullRequestForEnv(t, activities, testEnv.Activity.Name, "production")
	testhelpers.AssertHasPipelineStatus(t, activities, testEnv.Activity.Name, v1.ActivityStatusTypeRunning)
	// merge
	testhelpers.AssertSetPullRequestMerged(t, testEnv.FakeGitProvider, testEnv.ProdRepo.Owner, testEnv.ProdRepo.Name(), 1)
	testhelpers.AssertSetPullRequestComplete(t, testEnv.FakeGitProvider, testEnv.ProdRepo, 1)

	// retry the workflow to actually check the PR was merged and the app is in production
	pollGitStatusAndReactToPipelineChanges(t, testEnv.WorkflowOptions, jxClient, ns)
	testhelpers.AssertHasPromoteStatus(t, activities, testEnv.Activity.Name, "production", v1.ActivityStatusTypeSucceeded)
	assert.Equal(t, version, promoteOptions.ReleaseInfo.Version)

}

func TestPromoteToProductionNoMergeRun(t *testing.T) {

	// prepare the initial setup for testing
	testEnv, err := prepareInitialPromotionEnv(t, true)
	assert.NoError(t, err)

	// jx promote --batch-mode --app my-app --env production --no-merge --no-helm-update

	promoteOptions := &promote.PromoteOptions{
		Environment:         "production",                   // --env production
		Application:         "my-app",                       // --app my-app
		Pipeline:            testEnv.Activity.Spec.Pipeline, // needed for the test to pass on CI, otherwise it takes the actual CI build value
		Build:               testEnv.Activity.Spec.Build,    // needed for the test to pass on CI, otherwise it takes the actual CI build value
		Version:             "",
		ReleaseName:         "",
		LocalHelmRepoName:   "",
		HelmRepositoryURL:   "",
		NoHelmUpdate:        true, // --no-helm-update
		AllAutomatic:        false,
		NoMergePullRequest:  true,  // --no-merge
		NoPoll:              false, // note polling enabled
		NoWaitAfterMerge:    false,
		IgnoreLocalFiles:    true,
		Timeout:             "1h",
		PullRequestPollTime: "20s",
		Filter:              "",
		Alias:               "",
		Namespace:           "jx",
	}

	commonOpts := *testEnv.CommonOptions
	promoteOptions.CommonOptions = &commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources
	promoteOptions.BatchMode = true            // --batch-mode

	jxClient, ns, err := promoteOptions.JXClientAndDevNamespace()
	activities := jxClient.JenkinsV1().PipelineActivities(ns)

	testhelpers.AssertHasNoPullRequestForEnv(t, activities, testEnv.Activity.Name, "production")

	ch := make(chan int)

	// run the promote command in parallel
	go func() {
		err = promoteOptions.Run()
		assert.NoError(t, err)
		close(ch)
	}()

	// wait for the PR the be created by the promote command
	testhelpers.WaitForPullRequestForEnv(t, activities, testEnv.Activity.Name, "production")
	testhelpers.AssertHasPipelineStatus(t, activities, testEnv.Activity.Name, v1.ActivityStatusTypeRunning)

	// merge the PR created by promote command...
	testhelpers.AssertSetPullRequestMerged(t, testEnv.FakeGitProvider, testEnv.ProdRepo.Owner, testEnv.ProdRepo.Name(), 1)
	testhelpers.AssertSetPullRequestComplete(t, testEnv.FakeGitProvider, testEnv.ProdRepo, 1)

	// ...and wait for the Run routine to finish (it was polling on the PR to be merged)
	<-ch

	// retry the workflow to actually check the PR was merged and the app is in production
	pollGitStatusAndReactToPipelineChanges(t, testEnv.WorkflowOptions, jxClient, ns)
	testhelpers.AssertHasPromoteStatus(t, activities, testEnv.Activity.Name, "production", v1.ActivityStatusTypeSucceeded)

	//TODO: promoteOptions.ReleaseInfo.Version is empty here. Is this a bug?
	//assert.Equal(t, "1.0.1", promoteOptions.ReleaseInfo.Version) // default next version

	// however it looks like the activity contains the correct version...
	assert.Equal(t, "1.0.1", testEnv.Activity.Spec.Version)
}

func TestPromoteToProductionPRPollingRun(t *testing.T) {

	// prepare the initial setup for testing
	testEnv, err := prepareInitialPromotionEnv(t, true)
	assert.NoError(t, err)

	// jx promote --batch-mode --app my-app --env production --no-helm-update

	promoteOptions := &promote.PromoteOptions{
		Environment:         "production",                   // --env production
		Application:         "my-app",                       // --app my-app
		Pipeline:            testEnv.Activity.Spec.Pipeline, // needed for the test to pass on CI, otherwise it takes the actual CI build value
		Build:               testEnv.Activity.Spec.Build,    // needed for the test to pass on CI, otherwise it takes the actual CI build value
		Version:             "",
		ReleaseName:         "",
		LocalHelmRepoName:   "",
		HelmRepositoryURL:   "",
		NoHelmUpdate:        true, // --no-helm-update
		AllAutomatic:        false,
		NoMergePullRequest:  false, // note auto-merge enabled
		NoPoll:              false, // note polling enabled
		NoWaitAfterMerge:    false,
		IgnoreLocalFiles:    true,
		Timeout:             "1h",
		PullRequestPollTime: "20s",
		Filter:              "",
		Alias:               "",
		Namespace:           "jx",
	}

	commonOpts := *testEnv.CommonOptions
	promoteOptions.CommonOptions = &commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources
	promoteOptions.BatchMode = true            // --batch-mode

	jxClient, ns, err := promoteOptions.JXClientAndDevNamespace()
	activities := jxClient.JenkinsV1().PipelineActivities(ns)

	testhelpers.AssertHasNoPullRequestForEnv(t, activities, testEnv.Activity.Name, "production")

	ch := make(chan int)

	// run the promote command in parallel
	go func() {
		err = promoteOptions.Run()
		assert.NoError(t, err)
		close(ch)
	}()

	// wait for the PR the be created by the promote command
	testhelpers.WaitForPullRequestForEnv(t, activities, testEnv.Activity.Name, "production")
	testhelpers.AssertHasPipelineStatus(t, activities, testEnv.Activity.Name, v1.ActivityStatusTypeRunning)

	// mark latest commit as success tu unblock the promotion (PR will be automatically merged)
	testhelpers.SetSuccessCommitStatusInPR(t, testEnv.ProdRepo, 1)

	// ...and wait for the Run routine to finish (it was polling on the PR last commit status success to auto-merge)
	<-ch

	// retry the workflow to actually check the PR was merged and the app is in production
	pollGitStatusAndReactToPipelineChanges(t, testEnv.WorkflowOptions, jxClient, ns)
	testhelpers.AssertHasPromoteStatus(t, activities, testEnv.Activity.Name, "production", v1.ActivityStatusTypeSucceeded)

	//TODO: promoteOptions.ReleaseInfo.Version is empty here. Is this a bug?
	//assert.Equal(t, "1.0.1", promoteOptions.ReleaseInfo.Version) // default next version

	// however it looks like the activity contains the correct version...
	assert.Equal(t, "1.0.1", testEnv.Activity.Spec.Version)
}

func fakeSearchForChart(f string) (string, error) {
	return "mySearchedApp", nil
}

func fakeDiscoverAppName() (string, error) {
	return "myDiscoveredApp", nil
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagWithArgs(t *testing.T) {
	promoteOptions := &promote.PromoteOptions{
		Environment: "production", // --env production
	}

	commonOpts := &opts.CommonOptions{}
	promoteOptions.CommonOptions = commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources
	commonOpts.Args = []string{"myArgumentApp"}

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName)
	assert.NoError(t, err)

	assert.Equal(t, "myArgumentApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagWithFilterFlag(t *testing.T) {
	promoteOptions := &promote.PromoteOptions{
		Environment: "production", // --env production
		Filter:      "something",
	}

	commonOpts := &opts.CommonOptions{}
	promoteOptions.CommonOptions = commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName)
	assert.NoError(t, err)

	assert.Equal(t, "mySearchedApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagWithBatchFlag(t *testing.T) {
	promoteOptions := &promote.PromoteOptions{
		Environment: "production", // --env production
	}

	commonOpts := &opts.CommonOptions{}
	promoteOptions.CommonOptions = commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources
	promoteOptions.BatchMode = true           // --batch-mode

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName)
	assert.NoError(t, err)

	assert.Equal(t, "myDiscoveredApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlag(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")

	console := tests.NewTerminal(t, nil)
	defer console.Cleanup()

	promoteOptions := &promote.PromoteOptions{
		Environment: "production", // --env production
	}

	commonOpts := &opts.CommonOptions{}
	promoteOptions.CommonOptions = commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources
	promoteOptions.Out = console.Out
	promoteOptions.In = console.In

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		// Test boolean type
		console.ExpectString("Are you sure you want to promote the application named: myDiscoveredApp?")
		console.SendLine("Y")
		console.ExpectEOF()
	}()

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName)

	console.Close()
	<-donec

	assert.NoError(t, err)
	assert.Equal(t, "myDiscoveredApp", promoteOptions.Application)
}

func TestEnsureApplicationNameIsDefinedWithoutApplicationFlagUserSaysNo(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")

	console := tests.NewTerminal(t, nil)
	defer console.Cleanup()

	promoteOptions := &promote.PromoteOptions{
		Environment: "production", // --env production
	}

	commonOpts := &opts.CommonOptions{}
	promoteOptions.CommonOptions = commonOpts // Factory and other mocks initialized by cmd.ConfigureTestOptionsWithResources
	promoteOptions.Out = console.Out
	promoteOptions.In = console.In

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		// Test boolean type
		console.ExpectString("Are you sure you want to promote the application named: myDiscoveredApp?")
		console.SendLine("N")
		console.ExpectEOF()
	}()

	err := promoteOptions.EnsureApplicationNameIsDefined(fakeSearchForChart, fakeDiscoverAppName)

	console.Close()
	<-donec

	assert.Error(t, err)
	assert.Equal(t, "", promoteOptions.Application)
}

func TestGetEnvChartValues(t *testing.T) {
	tests := []struct {
		ns           string
		env          v1.Environment
		values       []string
		valueStrings []string
	}{{
		"jx-test-preview-pr-6",
		v1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-preview",
			},
			Spec: v1.EnvironmentSpec{
				Namespace:         "jx-test-preview-pr-6",
				Label:             "Test preview",
				Kind:              v1.EnvironmentKindTypePreview,
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
				PullRequestURL:    "https://github.com/my-project/my-app/pull/6",
				Order:             999,
				Source: v1.EnvironmentRepository{
					Kind: v1.EnvironmentRepositoryTypeGit,
					URL:  "https://github.com/my-project/my-app",
					Ref:  "my-branch",
				},
				PreviewGitSpec: v1.PreviewGitSpec{
					ApplicationName: "my-app",
					Name:            "6",
					URL:             "https://github.com/my-project/my-app/pull/6",
				},
			},
		},
		[]string{
			"tags.jx-preview=true",
			"tags.jx-env-test-preview=true",
			"tags.jx-ns-jx-test-preview-pr-6=true",
			"global.jxPreview=true",
			"global.jxEnvTestPreview=true",
			"global.jxNsJxTestPreviewPr6=true",
		},
		[]string{
			"global.jxTypeEnv=preview",
			"global.jxEnv=test-preview",
			"global.jxNs=jx-test-preview-pr-6",
			"global.jxPreviewApp=my-app",
			"global.jxPreviewPr=6",
		},
	}, {
		"jx-custom-env",
		v1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "custom-env",
			},
			Spec: v1.EnvironmentSpec{
				Namespace:         "jx-custom-env",
				Label:             "Custom environment",
				Kind:              v1.EnvironmentKindTypePermanent,
				PromotionStrategy: v1.PromotionStrategyTypeManual,
				Order:             5,
				Source: v1.EnvironmentRepository{
					Kind: v1.EnvironmentRepositoryTypeGit,
					URL:  "https://github.com/my-project/jx-environment-custom-env",
					Ref:  "master",
				},
			},
		},
		[]string{
			"tags.jx-permanent=true",
			"tags.jx-env-custom-env=true",
			"tags.jx-ns-jx-custom-env=true",
			"global.jxPermanent=true",
			"global.jxEnvCustomEnv=true",
			"global.jxNsJxCustomEnv=true",
		},
		[]string{
			"global.jxTypeEnv=permanent",
			"global.jxEnv=custom-env",
			"global.jxNs=jx-custom-env",
		},
	}, {
		"ns-rand",
		v1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "random-env",
			},
			Spec: v1.EnvironmentSpec{
				Namespace:         "ns-other",
				Label:             "Random environment",
				Kind:              v1.EnvironmentKindTypeEdit,
				PromotionStrategy: v1.PromotionStrategyTypeNever,
				Order:             666,
				Source: v1.EnvironmentRepository{
					Kind: v1.EnvironmentRepositoryTypeGit,
					URL:  "https://github.com/my-project/random",
					Ref:  "master",
				},
				PreviewGitSpec: v1.PreviewGitSpec{
					ApplicationName: "random",
					Name:            "2",
					URL:             "https://github.com/my-project/random/pull/6",
				},
			},
		},
		[]string{
			"tags.jx-edit=true",
			"tags.jx-env-random-env=true",
			"tags.jx-ns-ns-rand=true",
			"global.jxEdit=true",
			"global.jxEnvRandomEnv=true",
			"global.jxNsNsRand=true",
		},
		[]string{
			"global.jxTypeEnv=edit",
			"global.jxEnv=random-env",
			"global.jxNs=ns-rand",
		},
	}}

	for _, test := range tests {
		promoteOptions := &promote.PromoteOptions{}
		values, valueStrings := promoteOptions.GetEnvChartValues(test.ns, &test.env)
		sort.Strings(values)
		sort.Strings(test.values)
		assert.Equal(t, values, test.values)
		sort.Strings(valueStrings)
		sort.Strings(test.valueStrings)
		assert.Equal(t, valueStrings, test.valueStrings)
	}
}

// Contains all useful data from the test environment initialized by `prepareInitialPromotionEnv`
type TestEnv struct {
	Activity        *v1.PipelineActivity
	WorkflowOptions *controller.ControllerWorkflowOptions
	CommonOptions   *opts.CommonOptions
	FakeGitProvider *gits.FakeProvider
	DevRepo         *gits.FakeRepository
	StagingRepo     *gits.FakeRepository
	ProdRepo        *gits.FakeRepository
}

// Prepares an initial configuration with a typical environment setup.
// After a call to this function, version 1.0.1 of my-app is in staging, waiting to be promoted to production.
// It also prepare fakes of kube, jxClient, etc.
func prepareInitialPromotionEnv(t *testing.T, productionManualPromotion bool) (*TestEnv, error) {
	testOrgName := "myorg"
	testRepoName := "my-app"
	stagingRepoName := "environment-staging"
	prodRepoName := "environment-production"

	staging := kube.NewPermanentEnvironmentWithGit("staging", "https://fake.git/"+testOrgName+"/"+stagingRepoName+"."+
		"git")
	production := kube.NewPermanentEnvironmentWithGit("production",
		"https://fake.git/"+testOrgName+"/"+prodRepoName+".git")
	if productionManualPromotion {
		production.Spec.PromotionStrategy = v1.PromotionStrategyTypeManual
	}

	gitter := gits.NewGitCLI()
	commonOpts := &opts.CommonOptions{}
	commonOpts.SetFactory(fake.NewFakeFactory())

	err := testhelpers.CreateTestEnvironmentDir(commonOpts)
	assert.NoError(t, err)
	addFiles := func(dir string) error {
		// Really we should have a dummy environment chart but for now let's just mock it out as needed
		err = os.MkdirAll(filepath.Join(dir, "templates"), 0700)
		if err != nil {
			return err
		}
		data, err := json.Marshal(staging)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(dir, "templates", "environment-staging.yaml"), data, 0755)
		if err != nil {
			return err
		}
		data, err = json.Marshal(production)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(dir, "templates", "environment-production.yaml"), data, 0755)
		if err != nil {
			return err
		}
		return nil
	}

	fakeRepo, _ := gits.NewFakeRepository(testOrgName, testRepoName, nil, nil)
	stagingRepo, _ := gits.NewFakeRepository(testOrgName, stagingRepoName, addFiles, gitter)
	prodRepo, _ := gits.NewFakeRepository(testOrgName, prodRepoName, addFiles, gitter)

	// Needed for another helpe

	fakeGitProvider := gits.NewFakeProvider(fakeRepo, stagingRepo, prodRepo)
	fakeGitProvider.User.Username = testOrgName

	o := &controller.ControllerWorkflowOptions{
		CommonOptions: commonOpts,
		NoWatch:       true,
		Namespace:     "jx",
	}

	workflowName := "default"

	mockHelmer := helm_test.NewMockHelmer()
	testhelpers.ConfigureTestOptionsWithResources(commonOpts,
		[]runtime.Object{},
		[]runtime.Object{
			staging,
			production,
			kube.NewPreviewEnvironment("preview-pr-1"),
		},
		gitter,
		fakeGitProvider,
		mockHelmer,
		resources_mock.NewMockInstaller(),
	)

	//Mock out the helm repository fetch operation
	helm_test.StubFetchChart(testRepoName, "", kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    testRepoName,
			Version: "1.0.1",
		},
	}, mockHelmer)

	// Mock out the search versions operation

	pegomock.When(mockHelmer.SearchCharts(testRepoName, true)).ThenReturn([]helm.ChartSummary{
		{
			Name:         testRepoName,
			ChartVersion: "1.0.1",
			AppVersion:   "1.0.1",
		},
	}, nil)

	jxClient, ns, err := o.JXClientAndDevNamespace()
	assert.NoError(t, err)

	a, err := testhelpers.CreateTestPipelineActivity(jxClient, ns, testOrgName, testRepoName, "master", "1", workflowName)
	assert.NoError(t, err)
	if err != nil {
		return nil, err
	}

	err = o.Run()
	assert.NoError(t, err)
	if err != nil {
		return nil, err
	}
	activities := jxClient.JenkinsV1().PipelineActivities(ns)
	testhelpers.AssertHasPullRequestForEnv(t, activities, a.Name, "staging")
	testhelpers.AssertWorkflowStatus(t, activities, a.Name, v1.ActivityStatusTypeRunning)

	// react to the new PR in staging
	pollGitStatusAndReactToPipelineChanges(t, o, jxClient, ns)

	// lets make sure we don't create a PR for production as its manual
	testhelpers.AssertHasNoPullRequestForEnv(t, activities, a.Name, "production")

	// merge PR in staging repo
	if !testhelpers.AssertSetPullRequestMerged(t, fakeGitProvider, stagingRepo.Owner, stagingRepo.Name(), 1) {
		return nil, err
	}
	if !testhelpers.AssertSetPullRequestComplete(t, fakeGitProvider, stagingRepo, 1) {
		return nil, err
	}

	// react to the PR merge in staging
	pollGitStatusAndReactToPipelineChanges(t, o, jxClient, ns)

	// the pipeline activity succeeded
	testhelpers.AssertWorkflowStatus(t, activities, a.Name, v1.ActivityStatusTypeSucceeded)

	// There is no PR for production, as it is manual
	testhelpers.AssertHasNoPullRequestForEnv(t, activities, a.Name, "production")

	// Promote to staging succeeded...
	testhelpers.AssertHasPromoteStatus(t, activities, a.Name, "staging", v1.ActivityStatusTypeSucceeded)
	// ...and all promote-to-staging steps were successful
	testhelpers.AssertAllPromoteStepsSuccessful(t, activities, a.Name)

	return &TestEnv{
		Activity:        a,
		CommonOptions:   o.CommonOptions,
		WorkflowOptions: o,
		FakeGitProvider: fakeGitProvider,
		DevRepo:         fakeRepo,
		StagingRepo:     stagingRepo,
		ProdRepo:        prodRepo,
	}, nil
}

func pollGitStatusAndReactToPipelineChanges(t *testing.T, o *controller.ControllerWorkflowOptions, jxClient versioned.Interface, ns string) error {
	o.ReloadAndPollGitPipelineStatuses(jxClient, ns)
	err := o.Run()
	assert.NoError(t, err, "Failed to react to PipelineActivity changes")
	return err
}
