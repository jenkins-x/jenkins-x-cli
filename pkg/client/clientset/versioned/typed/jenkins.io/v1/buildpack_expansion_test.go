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
	testBuildPack = &v1.BuildPack{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.BuildPackSpec{},
	}
)

func TestPatchUpdateBuildPackNoModification(t *testing.T) {
	json, err := json.Marshal(testBuildPack)
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

	buildPacks := buildPacks{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := buildPacks.PatchUpdate(testBuildPack)
	assert.NoError(t, err)
	assert.Equal(t, testBuildPack, updated)
}

func TestPatchUpdateBuildPackWithChange(t *testing.T) {
	url := "git@github.com:jenkins-x/jx.git"
	clonedBuildPack := testBuildPack.DeepCopy()
	clonedBuildPack.Spec.GitURL = url

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testBuildPack)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedBuildPack)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	buildPacks := buildPacks{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := buildPacks.PatchUpdate(clonedBuildPack)
	assert.NoError(t, err)
	assert.NotEqual(t, testBuildPack, updated)
	assert.Equal(t, url, updated.Spec.GitURL)
}

func TestPatchUpdateBuildPackWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	buildPacks := buildPacks{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := buildPacks.PatchUpdate(testBuildPack)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateBuildPackWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testBuildPack)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	buildPacks := buildPacks{
		client: fakeClient,
		ns:     "default",
	}
	url := "git@github.com:jenkins-x/jx.git"
	clonedBuildPack := testBuildPack.DeepCopy()
	clonedBuildPack.Spec.GitURL = url
	updated, err := buildPacks.PatchUpdate(clonedBuildPack)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
