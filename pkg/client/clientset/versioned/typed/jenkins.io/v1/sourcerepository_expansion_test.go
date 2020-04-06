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
	testSourceRepository = &v1.SourceRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.SourceRepositorySpec{},
	}
)

func TestPatchUpdateSourceRepositoryNoModification(t *testing.T) {
	json, err := json.Marshal(testSourceRepository)
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

	sourceRepositories := sourceRepositories{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := sourceRepositories.PatchUpdate(testSourceRepository)
	assert.NoError(t, err)
	assert.Equal(t, testSourceRepository, updated)
}

func TestPatchUpdateSourceRepositoryWithChange(t *testing.T) {
	description := "my repo"
	clonedSourceRepository := testSourceRepository.DeepCopy()
	clonedSourceRepository.Spec.Description = description

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testSourceRepository)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedSourceRepository)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	sourceRepositories := sourceRepositories{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := sourceRepositories.PatchUpdate(clonedSourceRepository)
	assert.NoError(t, err)
	assert.NotEqual(t, testSourceRepository, updated)
	assert.Equal(t, description, updated.Spec.Description)
}

func TestPatchUpdateSourceRepositoryWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	sourceRepositories := sourceRepositories{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := sourceRepositories.PatchUpdate(testSourceRepository)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateSourceRepositoryWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testSourceRepository)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	sourceRepositories := sourceRepositories{
		client: fakeClient,
		ns:     "default",
	}
	description := "my repo"
	clonedSourceRepository := testSourceRepository.DeepCopy()
	clonedSourceRepository.Spec.Description = description
	updated, err := sourceRepositories.PatchUpdate(clonedSourceRepository)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
