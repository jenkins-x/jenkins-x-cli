// +build unit

package gits_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/suite"
	bitbucket "github.com/wbrefvem/go-bitbucket"
)

type UserProfile struct {
	name     string
	url      string
	username string
}

type BitbucketCloudProviderTestSuite struct {
	suite.Suite
	mux       *http.ServeMux
	server    *httptest.Server
	provider  gits.BitbucketCloudProvider
	providers map[string]gits.BitbucketCloudProvider
}

const (
	orgName = "test-org"
)

var profiles = []UserProfile{
	{
		url:      "https://auth.example.com",
		name:     "Test Auth Server",
		username: "test-user",
	},
	{
		url:      "https://auth.example.com",
		name:     "Test Auth Server with Underscore user",
		username: "test_user",
	},
}

var bitbucketRouter = util.Router{
	"/repositories/test-user": util.MethodMap{
		"GET": "repos.json",
	},
	"/repositories/test_user": util.MethodMap{
		"GET": "repos.json",
	},
	"/repositories/test-user/test-repo": util.MethodMap{
		"GET":    "repos.test-repo.json",
		"DELETE": "repos.test-repo.nil.json",
		"PUT":    "repos.test-repo-renamed.json",
	},
	"/repositories/test-user/test-repo/forks": util.MethodMap{
		"POST": "repos.test-fork.json",
	},
	"/repositories/test-user/test-repo/pullrequests": util.MethodMap{
		"POST": "pullrequests.test-repo.json",
	},
	"/repositories/test-org/test-repo/pullrequests": util.MethodMap{
		"POST": "pullrequests.test-org.test-repo.json",
	},
	"/repositories/test-user/test-repo/pullrequests/3/": util.MethodMap{
		"GET": "pullrequests.test-repo-closed.json",
	},
	"/repositories/test-org/test-repo/pullrequests/4/": util.MethodMap{
		"GET": "pullrequests.test-org.test-repo-closed.json",
	},
	"/repositories/test-user/test-repo/pullrequests/1/commits": util.MethodMap{
		"GET": "pullrequests.test-user.test-repo.1.json",
	},
	"/repositories/test-user/test-repo/pullrequests/3/commits": util.MethodMap{
		"GET": "pullrequests.test-repo.commits.json",
	},
	"/repositories/test-org/test-repo/pullrequests/4/commits": util.MethodMap{
		"GET": "pullrequests.test-org.test-repo.commits.json",
	},
	"/repositories/test-user/test-repo/commit/5c8afc5/statuses": util.MethodMap{
		"GET": "repos.test-repo.statuses.json",
	},
	"/repositories/test-user/test-repo/commit/7793466f879b83f1bdd8f3fc3f761bc3cb61bc41": util.MethodMap{
		"GET": "repos.test-user.test-repo.commits.7793466f879b83f1bdd8f3fc3f761bc3cb61bc41.json",
	},
	"/repositories/test-user/test-repo/commit/bbc7b863a56144647a806646b73e3b43749decad": util.MethodMap{
		"GET": "repos.test-user.test-repo.commits.bbc7b863a56144647a806646b73e3b43749decad.json",
	},
	"/repositories/test-user/test-repo/pullrequests/1/merge": util.MethodMap{
		"POST": "pullrequests.test-repo.merged.json",
	},
	"/repositories/test-user/test-repo/hooks": util.MethodMap{
		"POST": "webhooks.example.json",
	},
	"/repositories/test-user/test-repo/issues": util.MethodMap{
		"POST": "issues.test-repo.issue-1.json",
		"GET":  "issues.test-repo.json",
	},
	"/repositories/test-user/test-repo/issues/1": util.MethodMap{
		"GET": "issues.test-repo.issue-1.json",
	},
	"/users/test-user": util.MethodMap{
		"GET": "users.test-user.json",
	},
	"/repositories/test-user/test-repo/pullrequests/1/comments": util.MethodMap{
		"POST": "pullrequests.test-comment.json",
	},
	"/repositories/test-user/test-repo/issues/1/comments": util.MethodMap{
		"POST": "issue-comments.issue-1.json",
	},
}

func setupGitProvider(url, name, user string) (gits.GitProvider, error) {
	as := auth.AuthServer{
		URL:         url,
		Name:        "Test Auth Server",
		Kind:        "Oauth2",
		CurrentUser: user,
	}
	ua := auth.UserAuth{
		Username: user,
		ApiToken: "0123456789abdef",
	}

	git := gits.NewGitCLI()
	bp, err := gits.NewBitbucketCloudProvider(&as, &ua, git)

	return bp, err
}

func (suite *BitbucketCloudProviderTestSuite) SetupSuite() {
	suite.mux = http.NewServeMux()

	for path, methodMap := range bitbucketRouter {
		suite.mux.HandleFunc(path, util.GetMockAPIResponseFromFile("test_data/bitbucket_cloud", methodMap))
	}

	suite.server = httptest.NewServer(suite.mux)
	suite.Require().NotNil(suite.server)

	cfg := bitbucket.NewConfiguration()
	cfg.BasePath = suite.server.URL

	clientSingleton := bitbucket.NewAPIClient(cfg)
	suite.providers = map[string]gits.BitbucketCloudProvider{}

	for _, profile := range profiles {
		gp, err := setupGitProvider(profile.url, profile.name, profile.username)

		suite.Require().NotNil(gp)
		suite.Require().Nil(err)

		var ok bool
		bp, ok := gp.(*gits.BitbucketCloudProvider)

		suite.Require().NotNil(bp)
		suite.Require().True(ok)
		bp.Client = clientSingleton

		suite.providers[profile.username] = *bp
	}

	suite.provider = suite.providers["test-user"]

}

func (suite *BitbucketCloudProviderTestSuite) TestListRepositories() {

	for username, provider := range suite.providers {
		repos, err := provider.ListRepositories(username)
		suite.Require().Nil(err)
		suite.Require().NotNil(repos)

		suite.Require().Equal(len(repos), 2)

		for _, repo := range repos {
			suite.Require().NotNil(repo)
		}
	}
}

func (suite *BitbucketCloudProviderTestSuite) TestGetRepository() {

	repo, err := suite.provider.GetRepository(
		suite.provider.Username,
		"test-repo",
	)

	suite.Require().NotNil(repo)
	suite.Require().Nil(err)

	suite.Require().Equal(repo.Name, "test-repo")
}

func (suite *BitbucketCloudProviderTestSuite) TestDeleteRepository() {

	err := suite.provider.DeleteRepository(
		suite.provider.Username,
		"test-repo",
	)

	suite.Require().Nil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestForkRepository() {

	fork, err := suite.provider.ForkRepository(
		suite.provider.Username,
		"test-repo",
		"",
	)

	suite.Require().NotNil(fork)
	suite.Require().Nil(err)

	suite.Require().Equal(fork.Name, "test-fork")
}

func (suite *BitbucketCloudProviderTestSuite) TestValidateRepositoryName() {

	err := suite.provider.ValidateRepositoryName(suite.provider.Username, "test-repo")

	suite.Require().NotNil(err)

	err = suite.provider.ValidateRepositoryName(suite.provider.Username, "foo-repo")

	suite.Require().Nil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestRenameRepository() {

	repo, err := suite.provider.RenameRepository(suite.provider.Username, "test-repo", "test-repo-renamed")

	suite.Require().Nil(err)
	suite.Require().NotNil(repo)

	suite.Require().Equal(repo.Name, "test-repo-renamed")
}

func (suite *BitbucketCloudProviderTestSuite) TestCreatePullRequest() {
	args := gits.GitPullRequestArguments{
		GitRepository: &gits.GitRepository{Name: "test-repo", Organisation: "test-user"},
		Head:          "83777f6",
		Base:          "77d0a923f297",
		Title:         "Test Pull Request",
	}

	pr, err := suite.provider.CreatePullRequest(&args)

	suite.Require().NotNil(pr)
	suite.Require().Nil(err)
	suite.Require().Equal(*pr.State, "OPEN")
	suite.Require().Equal(*pr.Number, 3)
	suite.Require().Equal(pr.Owner, "test-user")
	suite.Require().Equal(pr.Repo, "test-repo")
	suite.Require().Equal(pr.Author.Login, "test-user")
}

func (suite *BitbucketCloudProviderTestSuite) TestCreateOrgPullRequest() {
	args := gits.GitPullRequestArguments{
		GitRepository: &gits.GitRepository{Name: "test-repo", Organisation: "test-org"},
		Head:          "83777f6",
		Base:          "77d0a923f297",
		Title:         "Test Pull Request",
	}

	pr, err := suite.provider.CreatePullRequest(&args)

	suite.Require().NotNil(pr)
	suite.Require().Nil(err)
	suite.Require().Equal(*pr.State, "OPEN")
	suite.Require().Equal(*pr.Number, 4)
	suite.Require().Equal(pr.Owner, "test-org")
	suite.Require().Equal(pr.Repo, "test-repo")
	suite.Require().Equal(pr.Author.Login, "test-user")
}

func (suite *BitbucketCloudProviderTestSuite) TestUpdatePullRequestStatus() {
	number := 3
	state := "OPEN"

	pr := &gits.GitPullRequest{
		Owner:  "test-user",
		Repo:   "test-repo",
		Number: &number,
		State:  &state,
	}

	err := suite.provider.UpdatePullRequestStatus(pr)

	suite.Require().NotNil(pr)
	suite.Require().Nil(err)
	suite.Require().Equal(*pr.State, "DECLINED")
	suite.Require().Equal(*pr.Number, 3)
	suite.Require().Equal(pr.Owner, "test-user")
	suite.Require().Equal(pr.Repo, "test-repo")
	suite.Require().Equal(pr.Author.Login, "test-user")
}

func (suite *BitbucketCloudProviderTestSuite) TestUpdateOrgPullRequestStatus() {
	number := 4
	state := "OPEN"

	pr := &gits.GitPullRequest{
		Owner:  "test-org",
		Repo:   "test-repo",
		Number: &number,
		State:  &state,
	}

	err := suite.provider.UpdatePullRequestStatus(pr)

	suite.Require().NotNil(pr)
	suite.Require().Nil(err)
	suite.Require().Equal(*pr.State, "DECLINED")
	suite.Require().Equal(*pr.Number, 4)
	suite.Require().Equal(pr.Owner, "test-org")
	suite.Require().Equal(pr.Repo, "test-repo")
	suite.Require().Equal(pr.Author.Login, "test-user")
}

func (suite *BitbucketCloudProviderTestSuite) TestGetPullRequest() {

	pr, err := suite.provider.GetPullRequest(
		"test-user",
		&gits.GitRepository{Name: "test-repo"},
		3,
	)

	suite.Require().Nil(err)
	suite.Require().Equal(*pr.Number, 3)
}

func (suite *BitbucketCloudProviderTestSuite) TestPullRequestCommits() {
	commits, err := suite.provider.GetPullRequestCommits("test-user", &gits.GitRepository{Name: "test-repo"}, 1)

	suite.Require().Nil(err)
	suite.Require().Equal(len(commits), 2)
	suite.Require().Equal(commits[0].Author.Email, "test-user@gmail.com")
}

func (suite *BitbucketCloudProviderTestSuite) TestPullRequestLastCommitStatus() {

	pr := &gits.GitPullRequest{
		Owner:         "test-user",
		Repo:          "test-repo",
		LastCommitSha: "5c8afc5",
	}
	lastCommitStatus, err := suite.provider.PullRequestLastCommitStatus(pr)

	suite.Require().Nil(err)
	suite.Require().NotEmpty(lastCommitStatus)
	suite.Require().Equal(lastCommitStatus, "in-progress")
}

func (suite *BitbucketCloudProviderTestSuite) testStatuses(statuses []*gits.GitRepoStatus, err error) {
	suite.Require().Nil(err)
	suite.Require().NotNil(statuses)
	suite.Require().Equal(len(statuses), 2)

	for _, status := range statuses {
		if status.ID == "ffffffffbf8d2a62" {
			suite.Require().Equal(status.State, "success")
		} else if status.ID == "626bb1b3" {
			suite.Require().Equal(status.State, "in-progress")
		}
		suite.Require().NotEmpty(status.State)
		suite.Require().NotEmpty(status.URL)
	}
}

func (suite *BitbucketCloudProviderTestSuite) TestListCommitStatus() {
	statuses, err := suite.provider.ListCommitStatus("test-user", "test-repo", "5c8afc5")
	suite.testStatuses(statuses, err)

	statuses, err = suite.provider.ListCommitStatus("test-user", "test-repo", "5c8afc5")
	suite.testStatuses(statuses, err)
}

func (suite *BitbucketCloudProviderTestSuite) TestMergePullRequest() {

	id := 1
	pr := &gits.GitPullRequest{
		Owner:  "test-user",
		Repo:   "test-repo",
		Number: &id,
	}
	err := suite.provider.MergePullRequest(pr, "Merging from unit tests")

	suite.Require().Nil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestCreateWebHook() {

	data := &gits.GitWebHookArguments{
		Repo: &gits.GitRepository{Name: "test-repo", Organisation: "test-user"},
		URL:  "https://my-jenkins.example.com/bitbucket-webhook/",
	}
	err := suite.provider.CreateWebHook(data)

	suite.Require().Nil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestSearchIssues() {
	issues, err := suite.provider.SearchIssues("test-user", "test-repo", "")

	suite.Require().Nil(err)
	suite.Require().NotNil(issues)

	for _, issue := range issues {
		suite.Require().NotNil(issue)
	}
}

func (suite *BitbucketCloudProviderTestSuite) TestGetIssue() {
	issue, err := suite.provider.GetIssue("test-user", "test-repo", 1)

	suite.Require().Nil(err)
	suite.Require().NotNil(issue)
	suite.Require().Equal(*issue.Number, 1)
}

func (suite *BitbucketCloudProviderTestSuite) TestCreateIssue() {

	issueToCreate := &gits.GitIssue{
		Title: "This is a test issue",
	}

	issue, err := suite.provider.CreateIssue("test-user", "test-repo", issueToCreate)

	suite.Require().Nil(err)
	suite.Require().NotNil(issue)
}

func (suite *BitbucketCloudProviderTestSuite) TestAddPRComment() {
	comment := "This is my comment. There are many like it but this one is mine."
	prNumber := 1

	pr := &gits.GitPullRequest{
		Number: &prNumber,
		Owner:  "test-user",
		Repo:   "test-repo",
	}
	err := suite.provider.AddPRComment(pr, comment)
	suite.Require().Nil(err)

	pr = &gits.GitPullRequest{
		Number: &prNumber,
		Owner:  "test-user",
	}
	err = suite.provider.AddPRComment(pr, comment)
	suite.Require().NotNil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestCreateIssueComment() {
	comment := "This is my comment. There are many like it but this one is mine."

	err := suite.provider.CreateIssueComment(
		"test-user",
		"test-repo",
		1,
		comment,
	)
	suite.Require().Nil(err)

	err = suite.provider.CreateIssueComment(
		"test-user",
		"test-repo",
		0,
		comment,
	)
	suite.Require().NotNil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestUpdateRelease() {
	err := suite.provider.UpdateRelease("", "", "", nil)
	suite.Require().Nil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestUserInfo() {
	user := suite.provider.UserInfo("test-user")
	suite.Require().NotNil(user)
	suite.Require().Equal("test-user", user.Login)
}

func (suite *BitbucketCloudProviderTestSuite) TestAddCollaborator() {
	err := suite.provider.AddCollaborator("derek", orgName, "repo")
	suite.Require().Nil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestListInvitations() {
	invites, res, err := suite.provider.ListInvitations()
	suite.Require().NotNil(invites)
	suite.Require().NotNil(res)
	suite.Require().Nil(err)
}

func (suite *BitbucketCloudProviderTestSuite) TestAcceptInvitations() {
	res, err := suite.provider.AcceptInvitation(1)
	suite.Require().NotNil(res)
	suite.Require().Nil(err)
}

func TestBitbucketCloudProviderTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping BitbucketCloudProviderTestSuite in short mode")
	} else {
		suite.Run(t, new(BitbucketCloudProviderTestSuite))
	}
}

func (suite *BitbucketCloudProviderTestSuite) TearDownSuite() {
	suite.server.Close()
}
