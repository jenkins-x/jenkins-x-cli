// +build unit

package v1

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	fakerest "k8s.io/client-go/rest/fake"
)

var (
	codecs = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
)

type fakeGetPatchRequest struct {
	get   func(*http.Request) (*http.Response, error)
	patch func(*http.Request) (*http.Response, error)
}

func newClientForTest(get func(*http.Request) (*http.Response, error), patch func(*http.Request) (*http.Response, error)) *fakerest.RESTClient {
	faker := newGetPatchRequest(get, patch)

	fakeClient := &fakerest.RESTClient{
		Client:               fakerest.CreateHTTPClient(faker.GetHandler()),
		NegotiatedSerializer: codecs,
		GroupVersion:         v1.SchemeGroupVersion,
		VersionedAPIPath:     "/not/a/real/path",
	}
	return fakeClient
}

func newGetPatchRequest(get func(*http.Request) (*http.Response, error), patch func(*http.Request) (*http.Response, error)) *fakeGetPatchRequest {
	return &fakeGetPatchRequest{
		get:   get,
		patch: patch,
	}
}

func (f *fakeGetPatchRequest) GetHandler() func(*http.Request) (*http.Response, error) {
	return f.fakeReqHandler
}

func (f *fakeGetPatchRequest) fakeReqHandler(req *http.Request) (*http.Response, error) {
	switch req.Method {
	case "GET":
		if f.get == nil {
			return nil, fmt.Errorf("unexpected request for URL %q with method %q", req.URL.String(), req.Method)
		}
		return f.get(req)
	case "PATCH":
		if f.patch == nil {
			return nil, fmt.Errorf("unexpected request for URL %q with method %q", req.URL.String(), req.Method)
		}
		return f.patch(req)
	default:
		return nil, fmt.Errorf("unexpected request for URL %q with method %q", req.URL.String(), req.Method)
	}
}

func bytesBody(bodyBytes []byte) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(bodyBytes))
}

func defaultHeaders() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}
