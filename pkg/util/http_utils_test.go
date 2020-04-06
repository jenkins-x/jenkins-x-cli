// +build unit

package util

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// transport for testing, using easy to spot values
var testTransport http.RoundTripper = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   7 * time.Second,
		KeepAlive: 7 * time.Second,
		DualStack: false,
	}).DialContext,
	MaxIdleConns:          7,
	IdleConnTimeout:       7 * time.Second,
	TLSHandshakeTimeout:   7 * time.Second,
	ExpectContinueTimeout: 7 * time.Second,
}

func TestGetClient(t *testing.T) {
	t.Parallel()

	// verify that default client timeout is correct
	myClient := GetClient()
	assert.Equal(t, time.Duration(getIntFromEnv("DEFAULT_HTTP_REQUEST_TIMEOUT", 30))*time.Second, myClient.Timeout)

	// verify that it times out properly
	timeoutClient := GetClientWithTimeout(3 * time.Second)
	assert.Equal(t, 3*time.Second, timeoutClient.Timeout)

	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	backend := httptest.NewServer(handlerFunc)

	response, err := timeoutClient.Get(backend.URL)
	if err == nil {
		defer response.Body.Close()
		t.Error("Request should have timed out")
	} else if err, ok := err.(net.Error); ok && !err.Timeout() {
		t.Errorf("Error making request to test server: %s", err.Error())
	}

	// verify using custom Transport
	customClient := GetCustomClient(testTransport, 7)
	assert.Equal(t, 7*time.Second, customClient.Timeout)
	//assert.Equal(t, customClient.Transport.(http.Transport).MaxIdleConns, 7)
	response, err = customClient.Get(backend.URL)
	if err == nil {
		defer response.Body.Close()
		t.Error("Request should have timed out")
	} else if err, ok := err.(net.Error); ok && !err.Timeout() {
		t.Errorf("Error making request to test server: %s", err.Error())
	}

	// sanity check
	assert.NotEqual(t, http.Client{}, myClient)
	assert.NotEqual(t, http.Client{}.Timeout, myClient.Timeout)
	assert.NotEqual(t, http.Client{}.Timeout, timeoutClient.Timeout)
	assert.NotEqual(t, http.Client{}.Timeout, customClient.Timeout)
	assert.NotEqual(t, myClient.Timeout, timeoutClient.Timeout)
	assert.NotEqual(t, myClient.Timeout, customClient.Timeout)
	assert.NotEqual(t, customClient.Timeout, timeoutClient.Timeout)

	myClient2 := GetClient()
	assert.Equal(t, myClient, myClient2)
}
