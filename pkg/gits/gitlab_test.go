// +build unit

package gits_test

import (
	"testing"

	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/suite"
	"github.com/xanzy/go-gitlab"
)

const (
	gitlabUserName       = "testperson"
	gitlabOrgName        = "testorg"
	gitlabProjectName    = "test-project"
	gitlabProjectID      = "5690870"
	gitlabMergeRequestID = 12
)

type GitlabProviderSuite struct {
	suite.Suite
	mux      *http.ServeMux
	server   *httptest.Server
	provider *gits.GitlabProvider
}

func (suite *GitlabProviderSuite) SetupSuite() {
	mux, server, provider := setup(suite)
	suite.mux = mux
	suite.server = server
	suite.provider = provider
}

func (suite *GitlabProviderSuite) TearDownSuite() {
	suite.server.Close()
}

// setup sets up a test HTTP server along with a gitlab.Client that is
// configured to talk to that test server.  Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup(suite *GitlabProviderSuite) (*http.ServeMux, *httptest.Server, *gits.GitlabProvider) {
	// mux is the HTTP request multiplexer used with the test server.
	mux := http.NewServeMux()
	configureGitlabMock(suite, mux)

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(mux)

	// Gitlab client
	client := gitlab.NewClient(nil, "")
	client.SetBaseURL(server.URL)

	userAuth := &auth.UserAuth{
		Username: gitlabUserName,
		ApiToken: "test",
	}

	authServer := &auth.AuthServer{
		URL:   server.URL,
		Users: []*auth.UserAuth{userAuth},
	}
	// Gitlab provider that we want to test
	git := gits.NewGitCLI()
	provider, _ := gits.WithGitlabClient(authServer, userAuth, client, git)

	return mux, server, provider.(*gits.GitlabProvider)
}

func configureGitlabMock(suite *GitlabProviderSuite, mux *http.ServeMux) {
	mux.HandleFunc("/api/v4/groups", func(w http.ResponseWriter, r *http.Request) {
		src, err := ioutil.ReadFile("test_data/gitlab/groups.json")

		suite.Require().Nil(err)
		w.Write(src)
	})

	mux.HandleFunc(fmt.Sprintf("/api/v4/groups/%s/projects", gitlabOrgName), func(w http.ResponseWriter, r *http.Request) {
		src, err := ioutil.ReadFile("test_data/gitlab/group-projects.json")

		suite.Require().Nil(err)
		w.Write(src)
	})

	mux.HandleFunc(fmt.Sprintf("/api/v4/users/%s/projects", gitlabUserName), func(w http.ResponseWriter, r *http.Request) {
		src, err := ioutil.ReadFile("test_data/gitlab/user-projects.json")

		suite.Require().Nil(err)
		w.Write(src)
	})

	gitlabRouter := util.Router{
		fmt.Sprintf("/api/v4/projects/%s", gitlabProjectID): util.MethodMap{
			"GET": "project.json",
		},
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d", gitlabProjectID, gitlabMergeRequestID): util.MethodMap{
			"GET": "merge-request.json",
			"PUT": "update-merge-request.json",
		},
		fmt.Sprintf("/api/v4/projects/%s/merge_requests", gitlabProjectID): util.MethodMap{
			"POST": "create-merge-request.json",
		},
	}
	for path, methodMap := range gitlabRouter {
		mux.HandleFunc(path, util.GetMockAPIResponseFromFile("test_data/gitlab", methodMap))
	}
}

func (suite *GitlabProviderSuite) TestListOrganizations() {
	orgs, err := suite.provider.ListOrganisations()

	suite.Require().Nil(err)
	suite.Require().Len(orgs, 1)
	suite.Require().Equal(gitlabOrgName, orgs[0].Login)
}

func (suite *GitlabProviderSuite) TestListRepositories() {
	require := suite.Require()
	scenarios := []struct {
		testDescription  string
		org              string
		expectedRepoName string
		expectedSSHURL   string
		expectedHTTPSURL string
		expectedHTMLURL  string
	}{
		{"List repositories for organization",
			gitlabOrgName,
			"orgproject",
			"git@gitlab.com:testorg/orgproject.git",
			"https://gitlab.com/testorg/orgproject.git",
			"https://gitlab.com/testorg/orgproject"},
		{"List repositories without organization",
			"", "userproject",
			"git@gitlab.com:testperson/userproject.git",
			"https://gitlab.com/testperson/userproject.git",
			"https://gitlab.com/testperson/userproject"},
	}

	for _, s := range scenarios {
		repositories, err := suite.provider.ListRepositories(s.org)
		require.Nil(err)
		require.Len(repositories, 2)
		require.Equal(s.expectedRepoName, repositories[0].Name)
		require.Equal(s.expectedSSHURL, repositories[0].SSHURL)
		require.Equal(s.expectedHTTPSURL, repositories[0].CloneURL)
		require.Equal(s.expectedHTMLURL, repositories[0].HTMLURL)
	}
}

func (suite *GitlabProviderSuite) TestGetRepository() {
	repo, err := suite.provider.GetRepository(gitlabUserName, gitlabProjectName)

	suite.Require().Nil(err)
	suite.Require().NotNil(repo)

	suite.Require().Equal(gitlabProjectName, repo.Name)
}

func (suite *GitlabProviderSuite) TestAddCollaborator() {
	err := suite.provider.AddCollaborator("derek", gitlabOrgName, "repo")
	suite.Require().Nil(err)
}

func (suite *GitlabProviderSuite) TestListInvitations() {
	invites, res, err := suite.provider.ListInvitations()
	suite.Require().NotNil(invites)
	suite.Require().NotNil(res)
	suite.Require().Nil(err)
}

func (suite *GitlabProviderSuite) TestAcceptInvitations() {
	res, err := suite.provider.AcceptInvitation(1)
	suite.Require().NotNil(res)
	suite.Require().Nil(err)
}

func (suite *GitlabProviderSuite) TestGetPullRequest() {
	pr, err := suite.provider.GetPullRequest(
		gitlabUserName,
		&gits.GitRepository{Name: gitlabProjectName},
		gitlabMergeRequestID,
	)

	suite.Require().Nil(err)
	suite.Require().Equal(*pr.Number, gitlabMergeRequestID)
}

func (suite *GitlabProviderSuite) TestCreatePullRequest() {

	args := gits.GitPullRequestArguments{
		GitRepository: &gits.GitRepository{Name: gitlabProjectName, Organisation: gitlabUserName},
		Head:          "source_branch",
		Base:          "target_branch",
		Title:         "Update Test Pull Request",
	}
	pr, err := suite.provider.CreatePullRequest(&args)

	//suite.Require().NotNil(pr)
	suite.Require().Nil(err)
	suite.Require().Equal(*pr.State, "merged")
	suite.Require().Equal(*pr.Number, 3)
	suite.Require().Equal(pr.Owner, gitlabUserName)
	suite.Require().Equal(pr.Repo, gitlabProjectName)
	suite.Require().Equal(pr.Author.Login, gitlabUserName)
}

func (suite *GitlabProviderSuite) TestUpdatePullRequest() {
	args := gits.GitPullRequestArguments{
		GitRepository: &gits.GitRepository{Name: gitlabProjectName, Organisation: gitlabUserName},
		Head:          "source_branch",
		Base:          "target_branch",
		Title:         "Update Test Pull Request",
	}
	pr, err := suite.provider.UpdatePullRequest(
		&args,
		gitlabMergeRequestID,
	)

	suite.Require().Nil(err)
	suite.Require().Equal(*pr.Number, gitlabMergeRequestID)
	suite.Require().Equal(pr.Owner, gitlabUserName)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestGitlabProviderSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TestGitlabProviderSuite in short mode")
	} else {
		suite.Run(t, new(GitlabProviderSuite))
	}
}
