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
	testFact = &v1.Fact{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.FactSpec{},
	}
)

func TestPatchUpdateFactNoModification(t *testing.T) {
	json, err := json.Marshal(testFact)
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

	facts := facts{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := facts.PatchUpdate(testFact)
	assert.NoError(t, err)
	assert.Equal(t, testFact, updated)
}

func TestPatchUpdateFactWithChange(t *testing.T) {
	name := "susfu"
	clonedFact := testFact.DeepCopy()
	clonedFact.Spec.Name = name

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testFact)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedFact)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	facts := facts{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := facts.PatchUpdate(clonedFact)
	assert.NoError(t, err)
	assert.NotEqual(t, testFact, updated)
	assert.Equal(t, name, updated.Spec.Name)
}

func TestPatchUpdateFactWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	facts := facts{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := facts.PatchUpdate(testFact)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateFactWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testFact)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	facts := facts{
		client: fakeClient,
		ns:     "default",
	}
	name := "susfu"
	clonedFact := testFact.DeepCopy()
	clonedFact.Spec.Name = name
	updated, err := facts.PatchUpdate(clonedFact)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
