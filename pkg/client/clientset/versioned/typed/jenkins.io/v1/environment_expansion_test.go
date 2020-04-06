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
	testEnvironment = &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.EnvironmentSpec{},
	}
)

func TestPatchUpdateEnvironmentNoModification(t *testing.T) {
	json, err := json.Marshal(testEnvironment)
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

	environments := environments{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := environments.PatchUpdate(testEnvironment)
	assert.NoError(t, err)
	assert.Equal(t, testEnvironment, updated)
}

func TestPatchUpdateEnvironmentWithChange(t *testing.T) {
	namespace := "jx"
	clonedEnvironment := testEnvironment.DeepCopy()
	clonedEnvironment.Spec.Namespace = namespace

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testEnvironment)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedEnvironment)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	environments := environments{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := environments.PatchUpdate(clonedEnvironment)
	assert.NoError(t, err)
	assert.NotEqual(t, testEnvironment, updated)
	assert.Equal(t, namespace, updated.Spec.Namespace)
}

func TestPatchUpdateEnvironmentWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	environments := environments{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := environments.PatchUpdate(testEnvironment)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateEnvironmentWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testEnvironment)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	environments := environments{
		client: fakeClient,
		ns:     "default",
	}
	namespace := "jx"
	clonedEnvironment := testEnvironment.DeepCopy()
	clonedEnvironment.Spec.Namespace = namespace
	updated, err := environments.PatchUpdate(clonedEnvironment)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
