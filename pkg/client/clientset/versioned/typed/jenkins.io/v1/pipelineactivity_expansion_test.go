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
	testPipelineActivity = &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.PipelineActivitySpec{},
	}
)

func TestPatchUpdatePipelineActivityNoModification(t *testing.T) {
	json, err := json.Marshal(testPipelineActivity)
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

	pipelineActivities := pipelineActivities{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := pipelineActivities.PatchUpdate(testPipelineActivity)
	assert.NoError(t, err)
	assert.Equal(t, testPipelineActivity, updated)
}

func TestPatchUpdatePipelineActivityWithChange(t *testing.T) {
	name := "test"
	clonedPipelineActivity := testPipelineActivity.DeepCopy()
	clonedPipelineActivity.Spec.Pipeline = name

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testPipelineActivity)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedPipelineActivity)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	pipelineActivities := pipelineActivities{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := pipelineActivities.PatchUpdate(clonedPipelineActivity)
	assert.NoError(t, err)
	assert.NotEqual(t, testPipelineActivity, updated)
	assert.Equal(t, name, updated.Spec.Pipeline)
}

func TestPatchUpdatePipelineActivityWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	pipelineActivities := pipelineActivities{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := pipelineActivities.PatchUpdate(testPipelineActivity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdatePipelineActivityWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testPipelineActivity)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	pipelineActivities := pipelineActivities{
		client: fakeClient,
		ns:     "default",
	}
	name := "test"
	clonedPipelineActivity := testPipelineActivity.DeepCopy()
	clonedPipelineActivity.Spec.Pipeline = name
	updated, err := pipelineActivities.PatchUpdate(clonedPipelineActivity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
