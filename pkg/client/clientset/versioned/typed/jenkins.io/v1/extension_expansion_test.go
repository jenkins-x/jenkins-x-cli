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
	testExtension = &v1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.ExtensionSpec{},
	}
)

func TestPatchUpdateExtensionNoModification(t *testing.T) {
	json, err := json.Marshal(testExtension)
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

	extensions := extensions{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := extensions.PatchUpdate(testExtension)
	assert.NoError(t, err)
	assert.Equal(t, testExtension, updated)
}

func TestPatchUpdateExtensionWithChange(t *testing.T) {
	name := "fubu"
	clonedExtension := testExtension.DeepCopy()
	clonedExtension.Spec.Name = name

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testExtension)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedExtension)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	extensions := extensions{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := extensions.PatchUpdate(clonedExtension)
	assert.NoError(t, err)
	assert.NotEqual(t, testExtension, updated)
	assert.Equal(t, name, updated.Spec.Name)
}

func TestPatchUpdateExtensionWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	extensions := extensions{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := extensions.PatchUpdate(testExtension)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateExtensionWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testExtension)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	extensions := extensions{
		client: fakeClient,
		ns:     "default",
	}
	name := "fubu"
	clonedExtension := testExtension.DeepCopy()
	clonedExtension.Spec.Name = name
	updated, err := extensions.PatchUpdate(clonedExtension)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
