// +build unit

package cve_test

import (
	"os"
	"testing"

	"net/http"

	"net/http/httptest"

	"fmt"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cve"
	"github.com/jenkins-x/jx/pkg/table"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/suite"
)

type AnchoreProviderTestSuite struct {
	suite.Suite
	mux      *http.ServeMux
	server   *httptest.Server
	provider *cve.AnchoreProvider
}

var router = util.Router{
	"/images/by_id/07b67913cd8c1ffc961c402b58c4e539ee6aaeae0b08969fc653267f4b975503/vuln/os": util.MethodMap{
		"GET": "vulnerabilities.json",
	},
	"/images/sha256:b9f03c3c4b196d46639bee0ec9cd0f6dbea8cc39d32767c8312f04317c3b18f4": util.MethodMap{
		"GET": "image.json",
	},
	"/images": util.MethodMap{
		"GET": "images.json",
	},
}

func TestAnchoreProviderTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TestAnchoreProviderTestSuite in short mode")
	} else {
		suite.Run(t, new(AnchoreProviderTestSuite))
	}
}

func (suite *AnchoreProviderTestSuite) SetupSuite() {
	suite.mux = http.NewServeMux()

	for path, methodMap := range router {
		suite.mux.HandleFunc(path, util.GetMockAPIResponseFromFile("test_data/anchore", methodMap))
	}

	suite.server = httptest.NewServer(suite.mux)
	suite.Require().NotNil(suite.server)

	as := auth.AuthServer{
		URL:         suite.server.URL,
		Name:        "Test Anchore Server",
		Kind:        "anchore-anchore-engine-core",
		CurrentUser: "admin",
	}
	ua := auth.UserAuth{
		Username: "admin",
		ApiToken: "admin",
	}

	a, err := cve.NewAnchoreProvider(&as, &ua)
	suite.Require().NotNil(a)
	suite.Require().Nil(err)

	var ok bool
	suite.provider, ok = a.(*cve.AnchoreProvider)
	suite.Require().True(ok)
	suite.Require().NotNil(suite.provider)

}

func (suite *AnchoreProviderTestSuite) TestUnmarshallImages() {

	var images []cve.Image

	subPath := fmt.Sprintf(cve.GetImages)

	err := suite.provider.AnchoreGet(subPath, &images)
	suite.Require().NoError(err)

	suite.Require().NotEmpty(images)
	suite.EqualValues(5, len(images))

	for _, i := range images {
		suite.EqualValues("analyzed", i.AnalysisStatus)
	}

	suite.EqualValues("jenkinsxio/nexus", images[0].ImageDetails[0].Repo)
	suite.EqualValues("0.0.5", images[0].ImageDetails[0].Tag)
	suite.EqualValues("docker.io", images[0].ImageDetails[0].Registry)

	suite.EqualValues("rawlingsj/jr1-rust", images[4].ImageDetails[0].Repo)
	suite.EqualValues("0.0.11", images[4].ImageDetails[0].Tag)
	suite.EqualValues("docker.io", images[4].ImageDetails[0].Registry)
}

func (suite *AnchoreProviderTestSuite) TestGetImageVulnerabilityTable() {

	vTable := table.CreateTable(os.Stdout)

	query := cve.CVEQuery{
		ImageID: "07b67913cd8c1ffc961c402b58c4e539ee6aaeae0b08969fc653267f4b975503",
	}

	err := suite.provider.GetImageVulnerabilityTable(nil, nil, &vTable, query)
	suite.Require().NoError(err)

	vTable.Render()

}
