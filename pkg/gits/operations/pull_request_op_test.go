// +build unit

package operations_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acarl005/stripansi"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/kube"

	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/jenkins-x/jx/pkg/helm"

	vault_test "github.com/jenkins-x/jx/pkg/vault/mocks"

	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"

	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/gits/operations"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/dependencymatrix"
	"github.com/jenkins-x/jx/pkg/gits"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

func setupTestPullRequestOperation(t *testing.T) operations.PullRequestOperation {
	_, _, _, commonOpts, _ := getFakeClientsAndNs(t)

	testOrgName := "testowner"
	testRepoName := "testrepo"

	commonOpts.SetGit(gits.NewGitFake())

	o := operations.PullRequestOperation{
		CommonOptions: &commonOpts,
	}

	gitter := gits_test.NewMockGitter()

	fakeRepo, _ := gits.NewFakeRepository(testOrgName, testRepoName, nil, nil)
	fakeGitProvider := gits.NewFakeProvider(fakeRepo)
	fakeGitProvider.User.Username = testOrgName

	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{},
		[]runtime.Object{},
		gitter,
		fakeGitProvider,
		nil,
		resources_test.NewMockInstaller(),
	)

	err := testhelpers.CreateTestEnvironmentDir(o.CommonOptions)
	assert.NoError(t, err)

	pegomock.When(gitter.HasChanges(pegomock.AnyString())).ThenReturn(true, nil)

	return o
}

func TestCreatePullRequest(t *testing.T) {
	prOpts := setupTestPullRequestOperation(t)

	prOpts.GitURLs = []string{"testowner/testrepo"}
	prOpts.SrcGitURL = "testowner/testrepo"
	prOpts.Version = "3.0.0"

	var results *gits.PullRequestInfo
	var err error

	logOutput := log.CaptureOutput(func() {
		results, err = prOpts.CreatePullRequest("test", func(dir string, gitInfo *gits.GitRepository) (strings []string, e error) {
			return []string{"1.0.0", "v1.0.1", "2.0.0"}, nil
		})
		assert.NoError(t, err)
		assert.NotNil(t, results, "we must have results coming out of the PR creation")
	})

	assert.Contains(t, logOutput, "Added label updatebot to Pull Request https://fake.git/testowner/testrepo/pulls/1",
		"Updatebot label should be added to the PR")

	assert.Equal(t, "chore(deps): bump testowner/testrepo from 1.0.0, 2.0.0 and v1.0.1 to 3.0.0",
		results.PullRequestArguments.Title, "The PR title should contain the old and new versions")
}

func TestCreatePullRequestsWithLabels(t *testing.T) {
	prOpts := setupTestPullRequestOperation(t)

	prOpts.GitURLs = []string{"testowner/testrepo"}
	prOpts.SrcGitURL = "testowner/testrepo"
	prOpts.Version = "2.0.0"
	fromVersion := "1.0.0"

	results, err := prOpts.CreatePullRequest("test", func(dir string, gitInfo *gits.GitRepository) (strings []string, e error) {
		return []string{fromVersion}, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, results, "we must have results coming out of the PR creation")

	prNumber := 1
	assert.Equal(t, prNumber, *results.PullRequest.Number, "This should be PR number %d", prNumber)

	assert.Equal(t, fmt.Sprintf("chore(deps): bump %s from %s to %s", prOpts.SrcGitURL, fromVersion, prOpts.Version),
		results.PullRequestArguments.Title, "The PR title should contain the old and new versions")

	assert.Equal(t, 1, len(results.PullRequest.Labels), "One label expected")
	assert.Equal(t, "updatebot", *results.PullRequest.Labels[0].Name, "updatebot label should exist")

	//create second PR with additional label
	prOpts.GitURLs = []string{"testowner/testrepo"}
	prOpts.Version = "4.0.0"
	prOpts.Labels = []string{"test-label"}

	labelResults, err := prOpts.CreatePullRequest("test", func(dir string, gitInfo *gits.GitRepository) (strings []string, e error) {
		return []string{fromVersion}, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, labelResults, "we must have results coming out of the second PR creation")

	prNumber = 2
	assert.Equal(t, prNumber, *labelResults.PullRequest.Number, "This should be PR number %d", prNumber)

	assert.Equal(t, fmt.Sprintf("chore(deps): bump %s from %s to %s", prOpts.SrcGitURL, fromVersion, prOpts.Version),
		labelResults.PullRequestArguments.Title, "The PR title should contain the old and new versions")

	assert.Equal(t, 2, len(labelResults.PullRequest.Labels), "Two labels expected")
	prLabels := []string{}
	for _, label := range labelResults.PullRequest.Labels {
		prLabels = append(prLabels, *label.Name)
	}
	assert.Contains(t, prLabels, "updatebot", "updatebot label should exist")
	assert.Contains(t, prLabels, prOpts.Labels[0], fmt.Sprintf("%s label should exist", prOpts.Labels[0]))
}

func TestCreatePullRequestWithMatrixUpdatePaths(t *testing.T) {

	_, _, _, commonOpts, _ := getFakeClientsAndNs(t)

	testOrgName := "testowner"
	testRepoName := "testrepo"

	commonOpts.SetGit(gits.NewGitFake())
	o := operations.PullRequestOperation{
		CommonOptions: &commonOpts,
	}

	viaRepo := "wiley"
	toVersion := "3.0.0"
	fromVersion := "1.0.0"
	toTag := fmt.Sprintf("v%s", toVersion)
	fromTag := fmt.Sprintf("v%s", fromVersion)
	host := "fake.git"
	updates := dependencymatrix.DependencyUpdates{
		Updates: []v1.DependencyUpdate{
			{
				DependencyUpdateDetails: v1.DependencyUpdateDetails{
					Host:               host,
					Owner:              testOrgName,
					Repo:               testRepoName,
					URL:                fmt.Sprintf("https://%s/%s/%s.git", host, testOrgName, testRepoName),
					ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, testRepoName, toTag),
					ToVersion:          toVersion,
					ToReleaseName:      toVersion,
					FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, testRepoName, fromTag),
					FromReleaseName:    fromVersion,
					FromVersion:        fromVersion,
				},
				Paths: []v1.DependencyUpdatePath{
					{
						{
							Host:               host,
							Owner:              testOrgName,
							Repo:               viaRepo,
							URL:                fmt.Sprintf("https://%s/%s/%s.git", host, testOrgName, viaRepo),
							ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, viaRepo, toTag),
							ToVersion:          toVersion,
							ToReleaseName:      toVersion,
							FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, viaRepo, fromTag),
							FromReleaseName:    fromVersion,
							FromVersion:        fromVersion,
						},
					},
				},
			},
		},
	}

	updateBytes, err := yaml.Marshal(updates)
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(200)
		fmt.Fprintf(w, string(updateBytes))
	}))

	gitter := gits_test.NewMockGitter()

	fakeRepo, _ := gits.NewFakeRepository(testOrgName, testRepoName, nil, nil)
	fakeRepo.Releases = map[string]*gits.GitRelease{
		"release-1": {
			Name:    "Release 1",
			TagName: "1.0.0",
			HTMLURL: "fakeUrlv1",
		},
		"release-3": {
			Name:    "Release 3",
			TagName: "3.0.0",
			HTMLURL: "fakeUrlv3",
			Assets: &[]gits.GitReleaseAsset{
				{
					ID:                 1,
					Name:               dependencymatrix.DependencyUpdatesAssetName,
					BrowserDownloadURL: server.URL,
					ContentType:        "application/json",
				},
			},
		},
	}
	fakeGitProvider := gits.NewFakeProvider(fakeRepo)
	fakeGitProvider.User.Username = testOrgName

	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{},
		[]runtime.Object{},
		gitter,
		fakeGitProvider,
		nil,
		resources_test.NewMockInstaller(),
	)

	err = testhelpers.CreateTestEnvironmentDir(o.CommonOptions)
	assert.NoError(t, err)

	o.GitURLs = []string{"testowner/testrepo"}
	o.SrcGitURL = "testowner/testrepo"
	o.Version = "3.0.0"

	pegomock.When(gitter.HasChanges(pegomock.AnyString())).ThenReturn(true, nil)

	var results *gits.PullRequestInfo

	logOutput := log.CaptureOutput(func() {
		results, err = o.CreatePullRequest("test", func(dir string, gitInfo *gits.GitRepository) (strings []string, e error) {
			return []string{"1.0.0", "v1.0.1", "2.0.0"}, nil
		})
		assert.NoError(t, err)
		assert.NotNil(t, results)
	})

	assert.Contains(t, stripansi.Strip(logOutput), "Added label updatebot to Pull Request https://fake.git/testowner/testrepo/pulls/1",
		"Updatebot label should be added to the PR")

	assert.NotNil(t, results, "we must have results coming out of the PR creation")
	assert.Equal(t, "chore(deps): bump testowner/testrepo from 1.0.0, 2.0.0 and v1.0.1 to 3.0.0",
		results.PullRequestArguments.Title, "The PR title should contain the old and new versions")
}

func TestCreateDependencyUpdatePRDetails(t *testing.T) {
	_, _, _, commonOpts, _ := getFakeClientsAndNs(t)

	commonOpts.SetGit(gits.NewGitFake())
	o := operations.PullRequestOperation{
		CommonOptions: &commonOpts,
	}

	gitter := gits_test.NewMockGitter()

	testOrgName := "testowner"
	testRepoName := "testrepo"
	gitRepo := fmt.Sprintf("%s/%s", testOrgName, testRepoName)
	fakeRepo, _ := gits.NewFakeRepository(testOrgName, testRepoName, nil, nil)
	fakeRepo.Releases = map[string]*gits.GitRelease{
		"release-1": {
			Name:    "Release 1",
			TagName: "1.0.0",
			HTMLURL: "fakeUrlv1",
		},
		"release-2": {
			Name:    "Release 2",
			TagName: "2.0.0",
			HTMLURL: "fakeUrlv2",
			Assets: &[]gits.GitReleaseAsset{
				{
					ID:                 1,
					Name:               "Asset1",
					BrowserDownloadURL: "fakeURL",
					ContentType:        "application/json",
				},
			},
		},
	}
	fakeGitProvider := gits.NewFakeProvider(fakeRepo)
	fakeGitProvider.User.Username = testOrgName

	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{},
		[]runtime.Object{},
		gitter,
		fakeGitProvider,
		nil,
		resources_test.NewMockInstaller(),
	)

	err := testhelpers.CreateTestEnvironmentDir(o.CommonOptions)
	assert.NoError(t, err)

	componentName := "testComponent"
	fromVersion := "1.0.0"
	toVersion := "2.0.0"
	kind := "fakekind"
	_, details, _, assets, err := o.CreateDependencyUpdatePRDetails(kind, gitRepo, "", fromVersion, toVersion, componentName)

	assert.Contains(t, details.BranchName, fmt.Sprintf("bump-%s-version", kind))
	assert.Contains(t, details.Message, fmt.Sprintf("Update [%s](%s):%s from [%s](fakeUrlv1) to [%s](fakeUrlv2)", gitRepo, gitRepo, componentName, fromVersion, toVersion))
	assert.Len(t, assets, 1)
}

func TestAddDependencyMatrixUpdatePaths(t *testing.T) {
	testOrgName := "testowner"
	testRepoName := "testrepo"
	viaRepo := "wiley"
	toVersion := "3.0.0"
	fromVersion := "1.0.0"
	toTag := fmt.Sprintf("v%s", toVersion)
	fromTag := fmt.Sprintf("v%s", fromVersion)
	host := "fake.git"
	updates := dependencymatrix.DependencyUpdates{
		Updates: []v1.DependencyUpdate{
			{
				DependencyUpdateDetails: v1.DependencyUpdateDetails{
					Host:               host,
					Owner:              testOrgName,
					Repo:               testRepoName,
					URL:                fmt.Sprintf("https://%s/%s/%s.git", host, testOrgName, testRepoName),
					ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, testRepoName, toTag),
					ToVersion:          toVersion,
					ToReleaseName:      toVersion,
					FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, testRepoName, fromTag),
					FromReleaseName:    fromVersion,
					FromVersion:        fromVersion,
				},
				Paths: []v1.DependencyUpdatePath{
					{
						{
							Host:               host,
							Owner:              testOrgName,
							Repo:               viaRepo,
							URL:                fmt.Sprintf("https://%s/%s/%s.git", host, testOrgName, viaRepo),
							ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, viaRepo, toTag),
							ToVersion:          toVersion,
							ToReleaseName:      toVersion,
							FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, viaRepo, fromTag),
							FromReleaseName:    fromVersion,
							FromVersion:        fromVersion,
						},
					},
				},
			},
		},
	}

	updateBytes, err := yaml.Marshal(updates)
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(200)
		fmt.Fprintf(w, string(updateBytes))
	}))

	asset := &gits.GitReleaseAsset{
		ID:                 1,
		BrowserDownloadURL: server.URL,
		ContentType:        "application/yaml",
		Name:               dependencymatrix.DependencyUpdatesAssetName,
	}

	update := &v1.DependencyUpdate{
		DependencyUpdateDetails: v1.DependencyUpdateDetails{
			Host:               host,
			Owner:              testOrgName,
			Repo:               testRepoName,
			URL:                fmt.Sprintf("https://%s/%s/%s.git", host, testOrgName, testRepoName),
			ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, testRepoName, toTag),
			ToVersion:          "2,0,0",
			ToReleaseName:      "2,0,0",
			FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, testRepoName, fromTag),
			FromReleaseName:    fromVersion,
			FromVersion:        fromVersion,
		},
		Paths: []v1.DependencyUpdatePath{
			{
				{
					Host:               host,
					Owner:              testOrgName,
					Repo:               viaRepo,
					URL:                fmt.Sprintf("https://%s/%s/%s.git", host, testOrgName, viaRepo),
					ToReleaseHTMLURL:   fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, viaRepo, toTag),
					ToVersion:          "2,0,0",
					ToReleaseName:      "2,0,0",
					FromReleaseHTMLURL: fmt.Sprintf("https://%s/%s/%s/releases/%s", host, testOrgName, viaRepo, fromTag),
					FromReleaseName:    fromVersion,
					FromVersion:        fromVersion,
				},
			},
		},
	}

	dependencyUpdate, err := operations.AddDependencyMatrixUpdatePaths(asset, update)
	assert.NoError(t, err)
	assert.Len(t, dependencyUpdate[0].Paths[0], 2)
}

// Helper method, not supposed to be a test by itself
func getFakeClientsAndNs(t *testing.T) (versioned.Interface, tektonclient.Interface, kubernetes.Interface, opts.CommonOptions, string) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	jxClient, ns, err := options.JXClientAndDevNamespace()
	assert.NoError(t, err, "There shouldn't be any error getting the fake JXClient and DevEnv")

	tektonClient, _, err := options.TektonClient()
	assert.NoError(t, err, "There shouldn't be any error getting the fake Tekton Client")

	kubeClient, err := options.KubeClient()
	assert.NoError(t, err, "There shouldn't be any error getting the fake Kube Client")

	return jxClient, tektonClient, kubeClient, commonOpts, ns
}

func TestCreatePullRequestBuildersFn(t *testing.T) {
	fn := operations.CreatePullRequestBuildersFn("1.0.1")
	dir, err := ioutil.TempDir("", "")
	defer func() {
		err := os.RemoveAll(dir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	err = util.CopyDir(filepath.Join("testdata", "CreatePullRequestBuildersFn"), dir, true)
	assert.NoError(t, err)
	var gitInfo *gits.GitRepository
	result, err := fn(dir, gitInfo)
	assert.NoError(t, err)
	tests.AssertFileContains(t, filepath.Join(dir, "docker", "gcr.io", "jenkinsxio", "builder-cf.yml"), "version: 1.0.1")
	tests.AssertFileContains(t, filepath.Join(dir, "docker", "gcr.io", "jenkinsxio", "builder-dlang.yml"), "version: 1.0.1")
	tests.AssertFileContains(t, filepath.Join(dir, "docker", "gcr.io", "jenkinsxio", "builder-base.yml"), "version: 0.0.1")
	assert.Contains(t, result, "0.0.1")
	assert.Contains(t, result, "0.0.2")
	assert.Len(t, result, 2)
}

func TestCreatePullRequestGitReleasesFn(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)
		commonOpts := &opts.CommonOptions{}
		gitter := gits.NewGitCLI()
		roadRunnerOrg, err := gits.NewFakeRepository("acme", "roadrunner", func(dir string) error {
			return ioutil.WriteFile(filepath.Join(dir, "README"), []byte("TODO"), 0655)
		}, gitter)
		assert.NoError(t, err)
		gitProvider := gits.NewFakeProvider(roadRunnerOrg)
		roadRunnerOrg.Releases["v1.2.3"] = &gits.GitRelease{
			Name: "1.2.3",
		}
		helmer := helm_test.NewMockHelmer()

		testhelpers.ConfigureTestOptionsWithResources(commonOpts,
			[]runtime.Object{},
			[]runtime.Object{
				kube.NewPermanentEnvironment("EnvWhereApplicationIsDeployed"),
			},
			gitter,
			gitProvider,
			helmer,
			resources_test.NewMockInstaller(),
		)

		pro := operations.PullRequestOperation{
			CommonOptions: commonOpts,
			SrcGitURL:     "",
			Version:       "",
		}

		fn := pro.CreatePullRequestGitReleasesFn("fake.git/acme/roadrunner")
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata", "CreatePullRequestGitReleasesFn"), dir, true)
		assert.NoError(t, err)
		oldVersions, err := fn(dir, nil)
		assert.NoError(t, err)
		assert.Len(t, oldVersions, 1)
		assert.Equal(t, "1.2.2", oldVersions[0])
		tests.AssertFileContains(t, filepath.Join(dir, "git", "fake.git", "acme", "roadrunner.yml"), "version: 1.2.3")
	})
	t.Run("not-found", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)
		commonOpts := &opts.CommonOptions{}
		gitter := gits.NewGitCLI()
		roadRunnerOrg, err := gits.NewFakeRepository("acme", "roadrunner", func(dir string) error {
			return ioutil.WriteFile(filepath.Join(dir, "README"), []byte("TODO"), 0655)
		}, gitter)
		assert.NoError(t, err)
		gitProvider := gits.NewFakeProvider(roadRunnerOrg)
		helmer := helm_test.NewMockHelmer()

		testhelpers.ConfigureTestOptionsWithResources(commonOpts,
			[]runtime.Object{},
			[]runtime.Object{
				kube.NewPermanentEnvironment("EnvWhereApplicationIsDeployed"),
			},
			gitter,
			gitProvider,
			helmer,
			resources_test.NewMockInstaller(),
		)

		pro := operations.PullRequestOperation{
			CommonOptions: commonOpts,
			SrcGitURL:     "",
			Version:       "",
		}

		fn := pro.CreatePullRequestGitReleasesFn("fake.git/acme/roadrunner")
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata", "CreatePullRequestGitReleasesFn"), dir, true)
		assert.NoError(t, err)
		oldVersions, err := fn(dir, nil)
		assert.Error(t, err)
		assert.Len(t, oldVersions, 0)
		tests.AssertFileContains(t, filepath.Join(dir, "git", "fake.git", "acme", "roadrunner.yml"), "version: 1.2.2")
	})
}

func TestCreatePullRequestRegexFn(t *testing.T) {
	t.Run("capture-groups", func(t *testing.T) {

		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata", "CreatePullRequestRegexFn"), dir, true)
		assert.NoError(t, err)
		fn, err := operations.CreatePullRequestRegexFn("1.0.1", "^version: (.*)$", "builder-dlang.yml")
		assert.NoError(t, err)
		var gitInfo *gits.GitRepository
		result, err := fn(dir, gitInfo)
		assert.NoError(t, err)
		tests.AssertFileContains(t, filepath.Join(dir, "builder-dlang.yml"), "version: 1.0.1")
		assert.Contains(t, result, "0.0.1")
		assert.Len(t, result, 1)
	})
	t.Run("named-capture", func(t *testing.T) {

		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata", "CreatePullRequestRegexFn"), dir, true)
		assert.NoError(t, err)
		fn, err := operations.CreatePullRequestRegexFn("1.0.1", `^version: (?P<version>.*)$`, "builder-dlang.yml")
		assert.NoError(t, err)
		var gitInfo *gits.GitRepository
		result, err := fn(dir, gitInfo)
		assert.NoError(t, err)
		tests.AssertFileContains(t, filepath.Join(dir, "builder-dlang.yml"), "version: 1.0.1")
		assert.Contains(t, result, "0.0.1")
		assert.Len(t, result, 1)
	})
	t.Run("multiple-named-capture", func(t *testing.T) {

		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata", "CreatePullRequestRegexFn"), dir, true)
		assert.NoError(t, err)
		fn, err := operations.CreatePullRequestRegexFn("1.0.1", `(?m)^(\s*)version: (?P<version>.*)$`, "builder-cf.yml")
		assert.NoError(t, err)
		var gitInfo *gits.GitRepository
		result, err := fn(dir, gitInfo)
		assert.NoError(t, err)
		tests.AssertFileContains(t, filepath.Join(dir, "builder-cf.yml"), `abc:
  version: 1.0.1
def:
  version: 1.0.1`)
		assert.Contains(t, result, "0.0.1")
		assert.Contains(t, result, "0.0.2")
		assert.Len(t, result, 2)
	})
}

func TestCreateChartChangeFilesFn(t *testing.T) {
	t.Run("from-chart-sources", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)
		helmer := helm_test.NewMockHelmer()
		helm_test.StubFetchChart("acme/roadrunner", "1.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "roadrunner",
				Version: "1.0.1",
				Sources: []string{
					"https://fake.git/acme/roadrunner",
				},
			},
		}, helmer)
		vaultClient := vault_test.NewMockClient()
		pro := operations.PullRequestOperation{}
		fn := operations.CreateChartChangeFilesFn("acme/roadrunner", "1.0.1", "charts", &pro, helmer, vaultClient, util.IOFileHandles{})
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata/TestCreateChartChangeFilesFn"), dir, true)
		assert.NoError(t, err)
		answer, err := fn(dir, nil)
		assert.NoError(t, err)
		assert.Len(t, answer, 1)
		assert.Contains(t, answer, "1.0.0")
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "roadrunner.yml"), "version: 1.0.1")
		assert.Equal(t, "https://fake.git/acme/roadrunner", pro.SrcGitURL)
		assert.Equal(t, "1.0.1", pro.Version)
	})
	t.Run("from-versions", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)
		helmer := helm_test.NewMockHelmer()
		helm_test.StubFetchChart("acme/wile", "1.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "wile",
				Version: "1.0.1",
			},
		}, helmer)
		vaultClient := vault_test.NewMockClient()
		pro := operations.PullRequestOperation{}
		fn := operations.CreateChartChangeFilesFn("acme/wile", "1.0.1", "charts", &pro, helmer, vaultClient, util.IOFileHandles{})
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata/TestCreateChartChangeFilesFn"), dir, true)
		assert.NoError(t, err)
		answer, err := fn(dir, nil)
		assert.NoError(t, err)
		assert.Len(t, answer, 1)
		assert.Contains(t, answer, "1.0.0")
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "wile.yml"), "version: 1.0.1")
		assert.Equal(t, "https://fake.git/acme/wile", pro.SrcGitURL)
		assert.Equal(t, "1.0.1", pro.Version)
	})
	t.Run("latest", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)
		helmer := helm_test.NewMockHelmer()
		pegomock.When(helmer.SearchCharts(pegomock.EqString("acme/wile"), pegomock.EqBool(true))).ThenReturn(pegomock.ReturnValue([]helm.ChartSummary{
			{
				Name:         "wile",
				ChartVersion: "1.0.1",
				AppVersion:   "1.0.1",
				Description:  "",
			},
			{
				Name:         "wile",
				ChartVersion: "1.0.0",
				AppVersion:   "1.0.0",
				Description:  "",
			},
		}), pegomock.ReturnValue(nil))
		helm_test.StubFetchChart("acme/wile", "1.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "wile",
				Version: "1.0.1",
			},
		}, helmer)
		pegomock.When(helmer.IsRepoMissing("https://acme.com/charts")).ThenReturn(pegomock.ReturnValue(false), pegomock.ReturnValue("acme"), pegomock.ReturnValue(nil))
		vaultClient := vault_test.NewMockClient()
		pro := operations.PullRequestOperation{}
		fn := operations.CreateChartChangeFilesFn("acme/wile", "", "charts", &pro, helmer, vaultClient, util.IOFileHandles{})
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata/TestCreateChartChangeFilesFn"), dir, true)
		assert.NoError(t, err)
		answer, err := fn(dir, nil)
		assert.NoError(t, err)
		assert.Len(t, answer, 1)
		assert.Contains(t, answer, "1.0.0")
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "wile.yml"), "version: 1.0.1")
		assert.Equal(t, "https://fake.git/acme/wile", pro.SrcGitURL)
		assert.Equal(t, "1.0.1", pro.Version)
	})

}

func TestPullRequestOperation_WrapChangeFilesWithCommitFn(t *testing.T) {

	commonOpts := &opts.CommonOptions{}
	gitter := gits.NewGitCLI()
	roadRunnerOrg, err := gits.NewFakeRepository("acme", "roadrunner", func(dir string) error {
		return ioutil.WriteFile(filepath.Join(dir, "README"), []byte("TODO"), 0655)
	}, gitter)
	assert.NoError(t, err)
	wileOrg, err := gits.NewFakeRepository("acme", "wile", func(dir string) error {
		return ioutil.WriteFile(filepath.Join(dir, "README"), []byte("TODO"), 0655)
	}, gitter)
	assert.NoError(t, err)
	gitProvider := gits.NewFakeProvider(roadRunnerOrg, wileOrg)
	helmer := helm_test.NewMockHelmer()

	testhelpers.ConfigureTestOptionsWithResources(commonOpts,
		[]runtime.Object{},
		[]runtime.Object{
			kube.NewPermanentEnvironment("EnvWhereApplicationIsDeployed"),
		},
		gitter,
		gitProvider,
		helmer,
		resources_test.NewMockInstaller(),
	)

	wrapped := func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		return []string{"1.2.2"}, ioutil.WriteFile(filepath.Join(dir, "test.yml"), []byte("version: 1.2.3"), 0655)
	}
	pro := operations.PullRequestOperation{
		CommonOptions: commonOpts,
		SrcGitURL:     "https://fake.git/acme/wile.git",
		Version:       "1.2.3",
	}

	dir, err := ioutil.TempDir("", "")
	defer func() {
		err := os.RemoveAll(dir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	err = gitter.Init(dir)
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "test.yml"), []byte("1.2.2"), 0655)
	assert.NoError(t, err)
	err = gitter.Add(dir, "*")
	assert.NoError(t, err)
	err = gitter.CommitDir(dir, "Initial commit")
	assert.NoError(t, err)

	gitInfo, err := gits.ParseGitURL("https://fake.git/acme/roadrunner.git")
	assert.NoError(t, err)

	fn := pro.WrapChangeFilesWithCommitFn("charts", wrapped)
	result, err := fn(dir, gitInfo)
	assert.NoError(t, err)
	assert.Len(t, result, 0)
	tests.AssertFileContains(t, filepath.Join(dir, "test.yml"), "1.2.3")
	msg, err := gitter.GetLatestCommitMessage(dir)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(msg, "chore(deps): bump acme/wile from 1.2.2 to 1.2.3"))
	// Without AuthorName and AuthorEmail, there shouldn't be a Signed-off-by message.
	assert.False(t, strings.Contains(msg, "Signed-off-by:"))

	// Wrap another commit, but this time with AuthorName and AuthorEmail set.
	wrappedWithAuthor := func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		return []string{"1.2.3"}, ioutil.WriteFile(filepath.Join(dir, "test.yml"), []byte("version: 1.2.4"), 0655)
	}
	pro.AuthorEmail = "someone@example.com"
	pro.AuthorName = "Some Author"
	pro.Version = "1.2.4"
	fnWithAuthor := pro.WrapChangeFilesWithCommitFn("charts", wrappedWithAuthor)
	resultWithAuthor, err := fnWithAuthor(dir, gitInfo)
	assert.NoError(t, err)
	assert.Len(t, resultWithAuthor, 0)
	tests.AssertFileContains(t, filepath.Join(dir, "test.yml"), "1.2.4")
	msgWithAuthor, err := gitter.GetLatestCommitMessage(dir)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(msgWithAuthor, "chore(deps): bump acme/wile from 1.2.3 to 1.2.4"))
	assert.True(t, strings.HasSuffix(msgWithAuthor, "Signed-off-by: Some Author <someone@example.com>"))
}
