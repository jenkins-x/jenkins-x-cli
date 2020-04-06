// +build unit

package buildnum

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jenkins-x/jx/pkg/buildnum/mocks/matchers"
	"github.com/jenkins-x/jx/pkg/kube"
	. "github.com/petergtz/pegomock"

	build_num_test "github.com/jenkins-x/jx/pkg/buildnum/mocks"

	"github.com/stretchr/testify/assert"
)

func TestVendGET(t *testing.T) {
	mockIssuer := build_num_test.NewMockBuildNumberIssuer()
	pID := kube.NewPipelineIDFromString("owner1/repo1/branch1")
	expectedBuildNum := "3"
	When(mockIssuer.NextBuildNumber(matchers.EqKubePipelineID(pID))).ThenReturn(expectedBuildNum, nil)

	respRecord := makeVendRequest(t, http.MethodGet, "/vend/owner1/repo1/branch1", mockIssuer)
	assert.Equal(t, http.StatusOK, respRecord.Code,
		"Expected OK status code for valid /vend GET request.")
	body := respRecord.Body.String()
	assert.Equal(t, expectedBuildNum, body)
}

func TestVendGETMissingPipeline(t *testing.T) {
	mockIssuer := build_num_test.NewMockBuildNumberIssuer()
	pID := kube.NewPipelineIDFromString("")
	expectedBuildNum := "543"
	When(mockIssuer.NextBuildNumber(matchers.EqKubePipelineID(pID))).ThenReturn(expectedBuildNum, nil)

	respRecord := makeVendRequest(t, http.MethodGet, "/vend/", mockIssuer)
	assert.Equal(t, http.StatusBadRequest, respRecord.Code,
		"Expected Bad Request for /vend GET request with missing pipeline.")
}

func TestVendUnsupportedMethod(t *testing.T) {
	mockIssuer := build_num_test.NewMockBuildNumberIssuer()

	respRecord := makeVendRequest(t, http.MethodDelete, "/vend/a/b/c", mockIssuer)
	assert.Equal(t, http.StatusMethodNotAllowed, respRecord.Code,
		"Expected Method Not Allowed status code for valid /vend DELETE request.")
}

func TestVendError(t *testing.T) {
	mockIssuer := build_num_test.NewMockBuildNumberIssuer()
	err := errors.New("something bad getting a build number")
	When(mockIssuer.NextBuildNumber(matchers.AnyKubePipelineID())).ThenReturn("", err)

	respRecord := makeVendRequest(t, http.MethodGet, "/vend/owner1/repo1/branch1", mockIssuer)
	assert.Equal(t, http.StatusInternalServerError, respRecord.Code,
		"Expected Internal Server Error status code for /vend GET request when BuildNumberIssuer fails.")
}

func makeVendRequest(t *testing.T, method string, path string, mockIssuer BuildNumberIssuer) *httptest.ResponseRecorder {
	server := NewHTTPBuildNumberServer("", 1234, mockIssuer)

	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		t.Fatal("Unexpected error setting up fake HTTP request.", err)
	}

	rr := httptest.NewRecorder()

	server.vend(rr, req)

	return rr
}
