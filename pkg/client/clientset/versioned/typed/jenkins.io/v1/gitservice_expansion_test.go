// +build unit

package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testGitService = &v1.GitService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.GitServiceSpec{},
	}
)

func TestPatchUpdateGitServiceNoModification(t *testing.T) {
	json, err := json.Marshal(testGitService)
	if err != nil {
		assert.Failf(t, "unable to marshal test instance: %s", err.Error())
	}
	get := func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	gitServices := gitServices{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := gitServices.PatchUpdate(testGitService)
	assert.NoError(t, err)
	assert.Equal(t, testGitService, updated)
}

func TestPatchUpdateGitServiceWithChange(t *testing.T) {
	name := "susfu"
	clonedGitService := testGitService.DeepCopy()
	clonedGitService.Spec.Name = name

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testGitService)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedGitService)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	gitServices := gitServices{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := gitServices.PatchUpdate(clonedGitService)
	assert.NoError(t, err)
	assert.NotEqual(t, testGitService, updated)
	assert.Equal(t, name, updated.Spec.Name)
}

func TestPatchUpdateGitServiceWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	gitServices := gitServices{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := gitServices.PatchUpdate(testGitService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateGitServiceWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testGitService)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	gitServices := gitServices{
		client: fakeClient,
		ns:     "default",
	}
	name := "susfu"
	clonedGitService := testGitService.DeepCopy()
	clonedGitService.Spec.Name = name
	updated, err := gitServices.PatchUpdate(clonedGitService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
