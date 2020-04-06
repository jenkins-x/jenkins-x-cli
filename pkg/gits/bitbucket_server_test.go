// +build unit

package gits_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	bitbucket "github.com/gfleury/go-bitbucket-v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/suite"
)

const (
	orgname = "test-org"
)

type BitbucketServerProviderTestSuite struct {
	suite.Suite
	mux      *http.ServeMux
	server   *httptest.Server
	provider *gits.BitbucketServerProvider
}

var bitbucketServerRouter = util.Router{
	"/rest/api/1.0/projects": util.MethodMap{
		"GET": "projects.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos": util.MethodMap{
		"GET":  "repos.json",
		"POST": "repos.test-repo123.json",
	},
	"/rest/api/1.0/users/test-user/repos/test-repo": util.MethodMap{
		"GET": "repos.test-repo.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo": util.MethodMap{
		"GET":    "repos.test-repo.json",
		"POST":   "repos.test-repo.json",
		"PUT":    "repos.test-repo-renamed.json",
		"DELETE": "repos.test-repo.nil.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo/pull-requests": util.MethodMap{
		"POST": "pr.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo/pull-requests/1": util.MethodMap{
		"GET": "pr.json",
		"PUT": "pr.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo/pull-requests/1/commits": util.MethodMap{
		"GET": "pr-commits.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo/pull-requests/1/merge": util.MethodMap{
		"POST": "pr-merge-success.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo/pull-requests/1/comments": util.MethodMap{
		"POST": "pr-comment.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo/webhooks": util.MethodMap{
		"POST": "webhook.json",
		"GET":  "webhooks.json",
	},
	"/rest/api/1.0/projects/TEST-ORG/repos/test-repo/webhooks/123": util.MethodMap{
		"PUT": "webhook.json",
	},
	"/rest/api/1.0/users/test-user": util.MethodMap{
		"GET": "user.json",
	},
	"/rest/build-status/1.0/commits/d6f24ee03d76a2caf0a4e1975fb43e8f61759b9c": util.MethodMap{
		"GET": "build-statuses.json",
	},
}

func (suite *BitbucketServerProviderTestSuite) SetupSuite() {
	suite.mux = http.NewServeMux()

	for path, methodMap := range bitbucketServerRouter {
		suite.mux.HandleFunc(path, util.GetMockAPIResponseFromFile("test_data/bitbucket_server", methodMap))
	}

	as := auth.AuthServer{
		URL:         "http://auth.example.com",
		Name:        "Test Auth Server",
		Kind:        "Oauth2",
		CurrentUser: "test-user",
	}
	ua := auth.UserAuth{
		Username: "test-user",
		ApiToken: "0123456789abdef",
	}

	git := gits.NewGitCLI()
	bp, err := gits.NewBitbucketServerProvider(&as, &ua, git)

	suite.Require().NotNil(bp)
	suite.Require().Nil(err)

	var ok bool
	suite.provider, ok = bp.(*gits.BitbucketServerProvider)
	suite.Require().True(ok)
	suite.Require().NotNil(suite.provider)

	suite.server = httptest.NewServer(suite.mux)
	suite.Require().NotNil(suite.server)

	cfg := bitbucket.NewConfiguration(suite.server.URL + "/rest")
	ctx := context.Background()

	apiKeyAuthContext := context.WithValue(ctx, bitbucket.ContextAccessToken, ua.ApiToken)
	suite.provider.Client = bitbucket.NewAPIClient(apiKeyAuthContext, cfg)
}

func (suite *BitbucketServerProviderTestSuite) TestGetRepository() {
	repo, err := suite.provider.GetRepository("TEST-ORG", "test-repo")
	suite.Require().Nil(err)
	suite.Require().NotNil(repo)
}

func (suite *BitbucketServerProviderTestSuite) TestListOrganizations() {
	orgs, err := suite.provider.ListOrganisations()
	suite.Require().Nil(err)
	suite.Require().NotEmpty(orgs)
}

// https://github.com/jenkins-x/jx/issues/2682
// Issue #2682
func (suite *BitbucketServerProviderTestSuite) TestListOrganizationsFailing() {
	as := auth.AuthServer{}
	ua := auth.UserAuth{}

	git := gits.NewGitCLI()
	bp, err := gits.NewBitbucketServerProvider(&as, &ua, git)

	suite.Require().NotNil(bp)
	suite.Require().Nil(err)

	var ok bool
	provider, ok := bp.(*gits.BitbucketServerProvider)
	suite.Require().True(ok)
	suite.Require().NotNil(suite.provider)

	// Broken URL
	cfg := bitbucket.NewConfiguration("http://127.0.0.1:4444/rest")
	ctx := context.Background()

	apiKeyAuthContext := context.WithValue(ctx, bitbucket.ContextAccessToken, ua.ApiToken)
	provider.Client = bitbucket.NewAPIClient(apiKeyAuthContext, cfg)

	orgs, err := provider.ListOrganisations()
	suite.Require().EqualError(err, "Get http://127.0.0.1:4444/rest/api/1.0/projects?limit=25&start=0: dial tcp 127.0.0.1:4444: connect: connection refused")
	suite.Require().Empty(orgs)

}
func (suite *BitbucketServerProviderTestSuite) TestListRepositories() {
	repos, err := suite.provider.ListRepositories("TEST-ORG")

	suite.Require().Nil(err)
	suite.Require().NotNil(repos)
	suite.Require().Equal(len(repos), 2)

	for _, repo := range repos {
		suite.Require().NotNil(repo)
	}
}

func (suite *BitbucketServerProviderTestSuite) TestCreateRepository() {
	repo, err := suite.provider.CreateRepository("TEST-ORG", "test-repo123", false)

	suite.Require().Nil(err)
	suite.Require().NotNil(repo)
}

func (suite *BitbucketServerProviderTestSuite) TestDeleteRepository() {
	err := suite.provider.DeleteRepository("TEST-ORG", "test-repo")
	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestRenameRepository() {

	repo, err := suite.provider.RenameRepository("TEST-ORG", "test-repo", "test-repo-renamed")

	suite.Require().Nil(err)
	suite.Require().NotNil(repo)

	suite.Require().Equal(repo.Name, "test-repo-renamed")
}

func (suite *BitbucketServerProviderTestSuite) TestValidateRepositoryName() {
	err := suite.provider.ValidateRepositoryName("TEST-ORG", "test-repo")
	suite.Require().NotNil(err)

	err = suite.provider.ValidateRepositoryName("TEST-ORG", "foo-repo")
	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestForkRepository() {

	fork, err := suite.provider.ForkRepository(
		"TEST-ORG",
		"test-repo",
		"",
	)

	suite.Require().NotNil(fork)
	suite.Require().Nil(err)

	suite.Require().Equal(fork.Name, "test-repo")
}

func (suite *BitbucketServerProviderTestSuite) TestCreatePullRequest() {
	args := gits.GitPullRequestArguments{
		GitRepository: &gits.GitRepository{
			Name:    "test-repo",
			Project: "TEST-ORG",
		},
		Head:  "refs/heads/feat/world",
		Base:  "refs/heads/master",
		Title: "Test Pull Request",
		Body:  "Test Pull request description",
	}

	pr, err := suite.provider.CreatePullRequest(&args)

	suite.Require().NotNil(pr)
	suite.Require().Nil(err)
	suite.Require().Equal(*pr.State, "OPEN")
}

func (suite *BitbucketServerProviderTestSuite) TestUpdatePullRequestStatus() {
	number := 1
	state := "CLOSED"

	pr := &gits.GitPullRequest{
		URL:    "https://auth.example.com/projects/TEST-ORG/repos/test-repo",
		Repo:   "test-repo",
		Number: &number,
		State:  &state,
	}

	err := suite.provider.UpdatePullRequestStatus(pr)

	suite.Require().Nil(err)
	suite.Require().Equal("OPEN", *pr.State)
	suite.Require().Equal("d6f24ee03d76a2caf0a4e1975fb43e8f61759b9c", pr.LastCommitSha)
}

func (suite *BitbucketServerProviderTestSuite) TestGetPullRequest() {

	pr, err := suite.provider.GetPullRequest(
		"test-user",
		&gits.GitRepository{Name: "test-repo", Project: "TEST-ORG"},
		1,
	)

	suite.Require().Nil(err)
	suite.Require().Equal(*pr.Number, 1)
}

func (suite *BitbucketServerProviderTestSuite) TestPullRequestCommits() {
	commits, err := suite.provider.GetPullRequestCommits("test-user", &gits.GitRepository{
		URL:     "https://auth.example.com/projects/TEST-ORG/repos/test-repo",
		Name:    "test-repo",
		Project: "TEST-ORG",
	}, 1)

	suite.Require().Nil(err)
	suite.Require().NotEmpty(commits)
	suite.Require().Equal(len(commits), 2)
	suite.Require().Equal("Test User", commits[0].Author.Name)
}

func (suite *BitbucketServerProviderTestSuite) TestPullRequestLastCommitStatus() {
	prNumber := 1
	pr := &gits.GitPullRequest{
		URL:    "https://auth.example.com/projects/TEST-ORG/repos/test-repo/pull-requests/7/overview",
		Repo:   "test-repo",
		Number: &prNumber,
	}
	lastCommitStatus, err := suite.provider.PullRequestLastCommitStatus(pr)

	suite.Require().Nil(err)
	suite.Require().NotEmpty(lastCommitStatus)
	suite.Require().Equal(lastCommitStatus, "in-progress")
}

func (suite *BitbucketServerProviderTestSuite) TestListCommitStatuses() {
	buildStatuses, err := suite.provider.ListCommitStatus("TEST-ORG", "test-repo", "d6f24ee03d76a2caf0a4e1975fb43e8f61759b9c")
	suite.Require().Nil(err)
	suite.Require().NotNil(buildStatuses)
	suite.Require().Equal(len(buildStatuses), 2)

	for _, status := range buildStatuses {
		if status.ID == "REPO-MASTER" {
			suite.Require().Equal(status.State, "in-progress")
		} else if status.ID == "Test-Master" {
			suite.Require().Equal(status.State, "success")
		}
		suite.Require().NotEmpty(status.State)
		suite.Require().NotEmpty(status.URL)
	}
}

func (suite *BitbucketServerProviderTestSuite) TestMergePullRequest() {

	id := 1
	pr := &gits.GitPullRequest{
		URL:    "https://auth.example.com/projects/TEST-ORG/repos/test-repo/pull-requests/1",
		Repo:   "test-repo",
		Number: &id,
	}
	err := suite.provider.MergePullRequest(pr, "Merging from unit tests")

	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestJenkinsWebHookPath() {
	p := suite.provider.JenkinsWebHookPath("notUsed", "notUsed")
	suite.Require().Equal("/bitbucket-scmsource-hook/notify?server_url=http%3A%2F%2Fauth.example.com", p)
}

func (suite *BitbucketServerProviderTestSuite) TestCreateWebHook() {

	data := &gits.GitWebHookArguments{
		Repo:   &gits.GitRepository{URL: "https://auth.example.com/projects/TEST-ORG/repos/test-repo"},
		URL:    "https://my-jenkins.example.com/bitbucket-webhook/",
		Secret: "someSecret",
	}
	err := suite.provider.CreateWebHook(data)

	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestListWebHooks() {

	webHooks, err := suite.provider.ListWebHooks("TEST-ORG", "test-repo")

	suite.Require().Nil(err)
	suite.Require().Len(webHooks, 1)

	webHook := webHooks[0]
	suite.Require().Equal(gits.GitWebHookArguments{
		ID:     123,
		Owner:  "TEST-ORG",
		Repo:   nil,
		URL:    "http://jenkins.example.com/bitbucket-scmsource-hook/notify",
		Secret: "abc123",
	}, *webHook)
}

func (suite *BitbucketServerProviderTestSuite) TestUpdateWebHook() {

	data := &gits.GitWebHookArguments{
		Repo:        &gits.GitRepository{URL: "https://auth.example.com/projects/TEST-ORG/repos/test-repo"},
		URL:         "http://jenkins.example.com/bitbucket-scmsource-hook/notify",
		ExistingURL: "http://jenkins.example.com/bitbucket-scmsource-hook/notify",
		Secret:      "someSecret",
	}
	err := suite.provider.UpdateWebHook(data)

	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestUserInfo() {

	userInfo := suite.provider.UserInfo("test-user")

	suite.Require().Equal(gits.GitUser{
		Login: "test-user",
		Name:  "Test User",
		Email: "test-user@example.com",
		URL:   "http://auth.example.com/users/test-user",
	}, *userInfo)
}

func (suite *BitbucketServerProviderTestSuite) TestAddCollaborator() {
	err := suite.provider.AddCollaborator("derek", orgname, "repo")
	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestListInvitations() {
	invites, res, err := suite.provider.ListInvitations()
	suite.Require().NotNil(invites)
	suite.Require().NotNil(res)
	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestAcceptInvitations() {
	res, err := suite.provider.AcceptInvitation(1)
	suite.Require().NotNil(res)
	suite.Require().Nil(err)
}

func (suite *BitbucketServerProviderTestSuite) TestAddPRComment() {

	id := 1
	pr := &gits.GitPullRequest{
		Owner:  "TEST-ORG",
		Repo:   "test-repo",
		Number: &id,
	}
	err := suite.provider.AddPRComment(pr, "This is a new comment.")

	suite.Require().Nil(err)
}

func TestBitbucketServerProviderTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TestBitbucketServerProviderTestSuite in short mode")
	} else {
		suite.Run(t, new(BitbucketServerProviderTestSuite))
	}
}

func (suite *BitbucketServerProviderTestSuite) TearDownSuite() {
	suite.server.Close()
}
