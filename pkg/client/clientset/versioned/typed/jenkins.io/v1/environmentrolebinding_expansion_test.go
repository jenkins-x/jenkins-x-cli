// +build unit

package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testEnvironmentRoleBinding = &v1.EnvironmentRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.EnvironmentRoleBindingSpec{},
	}
)

func TestPatchUpdateEnvironmentRoleBindingNoModification(t *testing.T) {
	json, err := json.Marshal(testEnvironmentRoleBinding)
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

	environmentRoleBindings := environmentRoleBindings{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := environmentRoleBindings.PatchUpdate(testEnvironmentRoleBinding)
	assert.NoError(t, err)
	assert.Equal(t, testEnvironmentRoleBinding, updated)
}

func TestPatchUpdateEnvironmentRoleBindingWithChange(t *testing.T) {
	subject := "snafu"
	clonedEnvironmentRoleBinding := testEnvironmentRoleBinding.DeepCopy()
	clonedEnvironmentRoleBinding.Spec.Subjects = []rbacv1.Subject{
		{
			Name: subject,
		},
	}

	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testEnvironmentRoleBinding)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(clonedEnvironmentRoleBinding)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	fakeClient := newClientForTest(get, patch)

	environmentRoleBindings := environmentRoleBindings{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := environmentRoleBindings.PatchUpdate(clonedEnvironmentRoleBinding)
	assert.NoError(t, err)
	assert.NotEqual(t, testEnvironmentRoleBinding, updated)
	assert.Equal(t, subject, updated.Spec.Subjects[0].Name)
}

func TestPatchUpdateEnvironmentRoleBindingWithErrorInGet(t *testing.T) {
	errorMessage := "error during GET"
	get := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, nil)

	environmentRoleBindings := environmentRoleBindings{
		client: fakeClient,
		ns:     "default",
	}

	updated, err := environmentRoleBindings.PatchUpdate(testEnvironmentRoleBinding)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}

func TestPatchUpdateEnvironmentRoleBindingWithErrorInPatch(t *testing.T) {
	errorMessage := "error during PATCH"
	get := func(*http.Request) (*http.Response, error) {
		json, err := json.Marshal(testEnvironmentRoleBinding)
		if err != nil {
			assert.Failf(t, "unable to marshal test instance: %s", err.Error())
		}
		return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(json)}, nil
	}

	patch := func(*http.Request) (*http.Response, error) {
		return nil, errors.New(errorMessage)
	}

	fakeClient := newClientForTest(get, patch)

	environmentRoleBindings := environmentRoleBindings{
		client: fakeClient,
		ns:     "default",
	}
	subject := "snafu"
	clonedEnvironmentRoleBinding := testEnvironmentRoleBinding.DeepCopy()
	clonedEnvironmentRoleBinding.Spec.Subjects = []rbacv1.Subject{
		{
			Name: subject,
		},
	}
	updated, err := environmentRoleBindings.PatchUpdate(clonedEnvironmentRoleBinding)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errorMessage)
	assert.Nil(t, updated)
}
