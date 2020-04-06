// +build unit

package verify_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/verify"
	"github.com/stretchr/testify/assert"
)

func newTestServer(endpoint string, fn func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(endpoint, fn)
	server := httptest.NewServer(mux)
	return server
}

func TestStepVerifyURLSuccess(t *testing.T) {
	t.Parallel()

	endpoint := "/test"
	server := newTestServer(endpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	url := server.URL + endpoint
	options := verify.StepVerifyURLOptions{
		Endpoint: url,
		Code:     http.StatusOK,
		Timeout:  10 * time.Second,
	}

	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	commonOpts.Out = os.Stdout
	commonOpts.Err = os.Stderr
	options.CommonOptions = &commonOpts

	err := options.Run()
	assert.NoError(t, err, "should verify an endpoint which returns the expected code without error")
}

func TestStepVerifyURLFailure(t *testing.T) {
	t.Parallel()

	endpoint := "/test"
	server := newTestServer(endpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	url := server.URL + endpoint
	options := verify.StepVerifyURLOptions{
		Endpoint: url,
		Code:     http.StatusOK,
		Timeout:  5 * time.Second,
	}

	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	commonOpts.Out = os.Stdout
	commonOpts.Err = os.Stderr
	options.CommonOptions = &commonOpts

	err := options.Run()
	assert.Error(t, err, "should verify an endpoint which does not return the expected code with an error")
}
