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
	testCommitStatus = &v1.CommitStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.CommitStatusSpec{},
	}
)

func TestPatchUpdateCommitStatusNoModification(t *testing.T) {
	json, err := json.Marshal(testCommitStatus)
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

	commitStatuses := commitStatuses{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := commitStatuses.PatchUpdate(testCommitStatus)
	assert.NoError(t, err)
	assert.Equal(t, testCommitStatus, updated)
}

func TestPatchUpdateCommitStatusWithChange(t *testing.T) {
	context := "foo"
	clonedCommitStatus := testCommitStatus.DeepCopy()
	clonedCommitStatus.Spec.Items = []v1.CommitStatusDetails{
		{
			Context: context,
		},
	}

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testCommitStatus)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedCommitStatus)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	commitStatuses := commitStatuses{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := commitStatuses.PatchUpdate(clonedCommitStatus)
	assert.NoError(t, err)
	assert.NotEqual(t, testCommitStatus, updated)
	assert.Equal(t, context, updated.Spec.Items[0].Context)
}

func TestPatchUpdateCommitStatusWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	commitStatuses := commitStatuses{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := commitStatuses.PatchUpdate(testCommitStatus)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateCommitStatusWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testCommitStatus)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	commitStatuses := commitStatuses{
		client: fakeClient,
		ns:     "default",
	}
	context := "foo"
	clonedCommitStatus := testCommitStatus.DeepCopy()
	clonedCommitStatus.Spec.Items = []v1.CommitStatusDetails{
		{
			Context: context,
		},
	}
	updated, err := commitStatuses.PatchUpdate(clonedCommitStatus)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
