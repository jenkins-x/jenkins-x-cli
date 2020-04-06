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
	testTeam = &v1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.TeamSpec{},
	}
)

func TestPatchUpdateTeamNoModification(t *testing.T) {
	json, err := json.Marshal(testTeam)
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

	teams := teams{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := teams.PatchUpdate(testTeam)
	assert.NoError(t, err)
	assert.Equal(t, testTeam, updated)
}

func TestPatchUpdateTeamWithChange(t *testing.T) {
	label := "Black Team"
	clonedTeam := testTeam.DeepCopy()
	clonedTeam.Spec.Label = label

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testTeam)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedTeam)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	teams := teams{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := teams.PatchUpdate(clonedTeam)
	assert.NoError(t, err)
	assert.NotEqual(t, testTeam, updated)
	assert.Equal(t, label, updated.Spec.Label)
}

func TestPatchUpdateTeamWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	teams := teams{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := teams.PatchUpdate(testTeam)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateTeamWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testTeam)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	teams := teams{
		client: fakeClient,
		ns:     "default",
	}
	label := "Black Team"
	clonedTeam := testTeam.DeepCopy()
	clonedTeam.Spec.Label = label
	updated, err := teams.PatchUpdate(clonedTeam)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
