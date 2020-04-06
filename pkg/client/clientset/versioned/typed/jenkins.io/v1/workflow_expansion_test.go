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
	testWorkflow = &v1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.WorkflowSpec{},
	}
)

func TestPatchUpdateWorkflowNoModification(t *testing.T) {
	json, err := json.Marshal(testWorkflow)
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

	workflows := workflows{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := workflows.PatchUpdate(testWorkflow)
	assert.NoError(t, err)
	assert.Equal(t, testWorkflow, updated)
}

func TestPatchUpdateWorkflowWithChange(t *testing.T) {
	pipelineName := "dummy-pipeline"
	clonedWorkflow := testWorkflow.DeepCopy()
	clonedWorkflow.Spec.PipelineName = pipelineName

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testWorkflow)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedWorkflow)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	workflows := workflows{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := workflows.PatchUpdate(clonedWorkflow)
	assert.NoError(t, err)
	assert.NotEqual(t, testWorkflow, updated)
	assert.Equal(t, pipelineName, updated.Spec.PipelineName)
}

func TestPatchUpdateWorkflowWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	workflows := workflows{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := workflows.PatchUpdate(testWorkflow)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateWorkflowWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testWorkflow)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	workflows := workflows{
		client: fakeClient,
		ns:     "default",
	}
	pipelineName := "dummy-pipeline"
	clonedWorkflow := testWorkflow.DeepCopy()
	clonedWorkflow.Spec.PipelineName = pipelineName
	updated, err := workflows.PatchUpdate(clonedWorkflow)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
