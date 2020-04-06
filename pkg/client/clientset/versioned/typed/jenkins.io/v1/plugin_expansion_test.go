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
	testPlugin = &v1.Plugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.PluginSpec{},
	}
)

func TestPatchUpdatePluginNoModification(t *testing.T) {
	json, err := json.Marshal(testPlugin)
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

	plugins := plugins{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := plugins.PatchUpdate(testPlugin)
	assert.NoError(t, err)
	assert.Equal(t, testPlugin, updated)
}

func TestPatchUpdatePluginWithChange(t *testing.T) {
	name := "susfu"
	clonedPlugin := testPlugin.DeepCopy()
	clonedPlugin.Spec.Name = name

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testPlugin)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedPlugin)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	plugins := plugins{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := plugins.PatchUpdate(clonedPlugin)
	assert.NoError(t, err)
	assert.NotEqual(t, testPlugin, updated)
	assert.Equal(t, name, updated.Spec.Name)
}

func TestPatchUpdatePluginWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	plugins := plugins{
		client: fakeClient,
		ns:     "default",
	}
	updated, err := plugins.PatchUpdate(testPlugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdatePluginWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testPlugin)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	plugins := plugins{
		client: fakeClient,
		ns:     "default",
	}
	name := "susfu"
	clonedPlugin := testPlugin.DeepCopy()
	clonedPlugin.Spec.Name = name
	updated, err := plugins.PatchUpdate(clonedPlugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
