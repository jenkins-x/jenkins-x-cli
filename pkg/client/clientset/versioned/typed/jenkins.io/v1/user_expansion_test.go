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
	testUser = &v1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.UserDetails{},
	}
)

func TestPatchUpdateUserNoModification(t *testing.T) {
	json, err := json.Marshal(testUser)
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

	users := users{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := users.PatchUpdate(testUser)
	assert.NoError(t, err)
	assert.Equal(t, testUser, updated)
}

func TestPatchUpdateUserWithChange(t *testing.T) {
	name := "susfu"
	clonedUser := testUser.DeepCopy()
	clonedUser.Spec.Name = name

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testUser)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedUser)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	users := users{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := users.PatchUpdate(clonedUser)
	assert.NoError(t, err)
	assert.NotEqual(t, testUser, updated)
	assert.Equal(t, name, updated.Spec.Name)
}

func TestPatchUpdateUserWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	users := users{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := users.PatchUpdate(testUser)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateUserWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testUser)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	users := users{
		client: fakeClient,
		ns:     "default",
	}
	name := "susfu"
	clonedUser := testUser.DeepCopy()
	clonedUser.Spec.Name = name
	updated, err := users.PatchUpdate(clonedUser)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
