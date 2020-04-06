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
	testRelease = &v1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.ReleaseSpec{},
	}
)

func TestPatchUpdateReleaseNoModification(t *testing.T) {
	json, err := json.Marshal(testRelease)
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

	releases := releases{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := releases.PatchUpdate(testRelease)
	assert.NoError(t, err)
	assert.Equal(t, testRelease, updated)
}

func TestPatchUpdateReleaseWithChange(t *testing.T) {
	name := "susfu"
	clonedRelease := testRelease.DeepCopy()
	clonedRelease.Spec.Name = name

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testRelease)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedRelease)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	releases := releases{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := releases.PatchUpdate(clonedRelease)
	assert.NoError(t, err)
	assert.NotEqual(t, testRelease, updated)
	assert.Equal(t, name, updated.Spec.Name)
}

func TestPatchUpdateReleaseWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	releases := releases{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := releases.PatchUpdate(testRelease)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateReleaseWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testRelease)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	releases := releases{
		client: fakeClient,
		ns:     "default",
	}
	name := "susfu"
	clonedRelease := testRelease.DeepCopy()
	clonedRelease.Spec.Name = name
	updated, err := releases.PatchUpdate(clonedRelease)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
