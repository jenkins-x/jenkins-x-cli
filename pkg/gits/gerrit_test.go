// +build unit

package gits_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/stretchr/testify/suite"
)

type GerritProviderTestSuite struct {
	suite.Suite
	mux      *http.ServeMux
	server   *httptest.Server
	provider *gits.GerritProvider
}

var gerritRouter = util.Router{
	"/a/projects/": util.MethodMap{
		"GET": "list-projects.json",
	},
	"/a/projects/test-org%2Ftest-user/": util.MethodMap{
		"PUT": "create-project.json",
	},
}

func (suite *GerritProviderTestSuite) SetupSuite() {
	suite.mux = http.NewServeMux()
	suite.server = httptest.NewServer(suite.mux)

	suite.Require().NotNil(suite.server)
	for path, methodMap := range gerritRouter {
		suite.mux.HandleFunc(path, util.GetMockAPIResponseFromFile("test_data/gerrit", methodMap))
	}

	as := auth.AuthServer{
		URL:         suite.server.URL,
		Name:        "Test Server",
		Kind:        "Oauth2",
		CurrentUser: "test-user",
	}
	ua := auth.UserAuth{
		Username: "test-user",
		ApiToken: "0123456789abdef",
	}

	gitter := gits.NewGitCLI()
	provider, err := gits.NewGerritProvider(&as, &ua, gitter)

	suite.Require().NotNil(provider)
	suite.Require().Nil(err)

	var ok bool
	suite.provider, ok = provider.(*gits.GerritProvider)
	suite.Require().True(ok)
	suite.Require().NotNil(suite.provider)
	suite.Require().NotNil(suite.provider.Client)
}

func (suite *GerritProviderTestSuite) TestListRepositories() {
	repos, err := suite.provider.ListRepositories("")

	suite.Require().NotNil(repos)
	suite.Require().Nil(err)
	suite.Require().Equal(4, len(repos))

	var repoNames []string
	for _, repo := range repos {
		repoNames = append(repoNames, repo.Name)
	}
	sort.Strings(repoNames)

	suite.Require().Equal("All-Projects", repoNames[0])
	suite.Require().Equal("All-Users", repoNames[1])
	suite.Require().Equal("RecipeBook", repoNames[2])
	suite.Require().Equal("testing", repoNames[3])

}

func (suite *GerritProviderTestSuite) TestCreateRepository() {
	repo, err := suite.provider.CreateRepository("test-org", "test-user", false)
	suite.T().Log(err)
	suite.Require().NotNil(repo)
	suite.Require().Nil(err)
	suite.Require().Equal("test-org/test-repo", repo.Name)
	suite.Require().Equal(fmt.Sprintf("%s/test-org/test-repo", suite.server.URL), repo.CloneURL)
	suite.Require().Equal(fmt.Sprintf("%s:test-org/test-repo", suite.server.URL), repo.SSHURL)
}

func TestGerritProviderTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping GerritProviderTestSuite in short mode")
	} else {
		suite.Run(t, new(GerritProviderTestSuite))
	}
}

func (suite *GerritProviderTestSuite) TearDownSuite() {
	suite.server.Close()
}
