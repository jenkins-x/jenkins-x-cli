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
	testPipelineStructure = &v1.PipelineStructure{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
)

func TestPatchUpdatePipelineStructureNoModification(t *testing.T) {
	json, err := json.Marshal(testPipelineStructure)
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

	pipelineStructures := pipelineStructures{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := pipelineStructures.PatchUpdate(testPipelineStructure)
	assert.NoError(t, err)
	assert.Equal(t, testPipelineStructure, updated)
}

func TestPatchUpdatePipelineStructureWithChange(t *testing.T) {
	ref := "susfu"
	clonedPipelineStructure := testPipelineStructure.DeepCopy()
	clonedPipelineStructure.PipelineRef = &ref

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testPipelineStructure)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedPipelineStructure)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	pipelineStructures := pipelineStructures{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := pipelineStructures.PatchUpdate(clonedPipelineStructure)
	assert.NoError(t, err)
	assert.NotEqual(t, testPipelineStructure, updated)
	assert.Equal(t, &ref, updated.PipelineRef)
}

func TestPatchUpdatePipelineStructureWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	pipelineStructures := pipelineStructures{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := pipelineStructures.PatchUpdate(testPipelineStructure)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdatePipelineStructureWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testPipelineStructure)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	pipelineStructures := pipelineStructures{
		client: fakeClient,
		ns:     "default",
	}
	ref := "susfu"
	clonedPipelineStructure := testPipelineStructure.DeepCopy()
	clonedPipelineStructure.PipelineRef = &ref
	updated, err := pipelineStructures.PatchUpdate(clonedPipelineStructure)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
