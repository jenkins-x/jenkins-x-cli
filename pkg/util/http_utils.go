package util

import (
	"bytes"

	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
)

// defaults mirror the default http.Transport values
var jxDefaultTransport http.RoundTripper = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   time.Duration(getIntFromEnv("HTTP_DIALER_TIMEOUT", 30)) * time.Second,
		KeepAlive: time.Duration(getIntFromEnv("HTTP_DIALER_KEEP_ALIVE", 30)) * time.Second,
		DualStack: getBoolFromEnv("HTTP_USE_DUAL_STACK", true),
	}).DialContext,
	MaxIdleConns:          getIntFromEnv("HTTP_MAX_IDLE_CONNS", 100),
	IdleConnTimeout:       time.Duration(getIntFromEnv("HTTP_IDLE_CONN_TIMEOUT", 90)) * time.Second,
	TLSHandshakeTimeout:   time.Duration(getIntFromEnv("HTTP_TLS_HANDSHAKE_TIMEOUT", 10)) * time.Second,
	ExpectContinueTimeout: time.Duration(getIntFromEnv("HTTP_EXPECT_CONTINUE_TIMEOUT", 1)) * time.Second,
	Proxy:                 http.ProxyFromEnvironment,
}

var defaultClient = http.Client{Transport: jxDefaultTransport, Timeout: time.Duration(getIntFromEnv("DEFAULT_HTTP_REQUEST_TIMEOUT", 30)) * time.Second}

// GetClient returns a Client reference with our default configuration
func GetClient() *http.Client {
	return &defaultClient
}

// GetClientWithTimeout returns a client with JX default transport and user specified timeout
func GetClientWithTimeout(duration time.Duration) *http.Client {
	client := http.Client{}
	client.Transport = jxDefaultTransport
	client.Timeout = duration
	return &client
}

// GetCustomClient returns a client with user specified transport and timeout (in seconds)
func GetCustomClient(transport http.RoundTripper, timeout int) *http.Client {
	return &(http.Client{Transport: transport, Timeout: time.Duration(timeout) * time.Second})
}

func getIntFromEnv(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		intValue, err := strconv.Atoi(value)
		if err == nil {
			return intValue
		}
		log.Logger().Warnf("Unable to convert env var %s with value %s to integer, using default value of %d instead", key, value, fallback)
	}
	return fallback
}

func getBoolFromEnv(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		boolValue, err := strconv.ParseBool(value)
		if err == nil {
			return boolValue
		}
		log.Logger().Warnf("Unable to convert env var %s with value %s to boolean, using default value of %t instead", key, value, fallback)
	}
	return fallback
}

// CallWithExponentialBackOff make a http call with exponential backoff retry
func CallWithExponentialBackOff(url string, auth string, httpMethod string, reqBody []byte, reqParams url.Values) ([]byte, error) {
	log.Logger().Debugf("%sing %s to %s", httpMethod, reqBody, url)
	resp, respBody := &http.Response{}, []byte{}
	if url != "" && httpMethod != "" {
		f := func() error {
			req, err := http.NewRequest(httpMethod, url, bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			if !strings.Contains(url, "localhost") || !strings.Contains(url, "127.0.0.1") {
				if strings.Count(auth, ":") == 1 {
					jxBasicAuthUser, jxBasicAuthPass := GetBasicAuthUserAndPassword(auth)
					req.SetBasicAuth(jxBasicAuthUser, jxBasicAuthPass)
				}
			}

			if reqParams != nil {
				req.URL.RawQuery = reqParams.Encode()
			}

			resp, err = defaultClient.Do(req)
			if err != nil {
				return backoff.Permanent(err)
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return errors.Errorf("%s not available, error was %d %s", url, resp.StatusCode, resp.Status)
			}
			respBody, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return backoff.Permanent(errors.Wrap(err, "parsing response body"))
			}
			resp.Body.Close()
			return nil
		}
		exponentialBackOff := backoff.NewExponentialBackOff()
		timeout := 1 * time.Minute
		exponentialBackOff.MaxElapsedTime = timeout
		exponentialBackOff.Reset()
		err := backoff.Retry(f, exponentialBackOff)
		if err != nil {
			return []byte{}, errors.Wrapf(err, "error performing request via %s", url)
		}
	}
	return respBody, nil
}

func GetBasicAuthUserAndPassword(auth string) (string, string) {
	if auth != "" {
		creds := strings.Fields(strings.Replace(auth, ":", " ", -1))
		return creds[0], creds[1]
	}
	return "", ""
}
