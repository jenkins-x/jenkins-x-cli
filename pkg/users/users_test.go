// +build unit

package users_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/jenkins-x/jx/pkg/users"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"k8s.io/apimachinery/pkg/runtime"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/stretchr/testify/assert"
)

func TestResolveUserWithEmptyIdDoesNotCreateEmptyAccountReference(t *testing.T) {
	t.Parallel()
	o := opts.CommonOptions{}
	user := jenkinsv1.User{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "jx",
			Name:      "test-user",
		},
		Spec: jenkinsv1.UserDetails{
			Name:     "Test",
			Email:    "test@test.com",
			Accounts: make([]jenkinsv1.AccountReference, 0),
		},
	}

	testhelpers.ConfigureTestOptionsWithResources(&o,
		[]runtime.Object{},
		[]runtime.Object{&user},
		&gits.GitFake{},
		&gits.FakeProvider{},
		helm_test.NewMockHelmer(),
		resources_test.NewMockInstaller(),
	)

	jxClient, ns, err := o.JXClientAndDevNamespace()
	assert.NoError(t, err)

	// First check resolving with an id
	resolved, err := users.Resolve("", "testProvider", jxClient, ns, func(id string, users []jenkinsv1.User) (string,
		[]jenkinsv1.User, *jenkinsv1.User, error) {
		return "123", []jenkinsv1.User{user}, &jenkinsv1.User{}, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Len(t, resolved.Spec.Accounts, 1)

	// Now with an empty id
	resolved, err = users.Resolve("", "testProvider", jxClient, ns, func(id string, users []jenkinsv1.User) (string,
		[]jenkinsv1.User, *jenkinsv1.User, error) {
		return "", []jenkinsv1.User{user}, &jenkinsv1.User{}, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Len(t, resolved.Spec.Accounts, 0)
}

func TestExistingUserIdButNotFoundBySelectErrors(t *testing.T) {
	t.Parallel()
	o := opts.CommonOptions{}

	//user1 is the user already existing in the cluster
	user1 := jenkinsv1.User{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "jx",
			Name:      "test-user",
		},
		Spec: jenkinsv1.UserDetails{
			Name:     "Test",
			Email:    "test@test.com",
			Accounts: make([]jenkinsv1.AccountReference, 0),
		},
	}

	// user2 is the user we will return as new user when we resolve the users
	user2 := jenkinsv1.User{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "jx",
			Name:      "test-user",
		},
		Spec: jenkinsv1.UserDetails{
			Name:     "Test",
			Email:    "test@acme.com",
			Accounts: make([]jenkinsv1.AccountReference, 0),
		},
	}

	testhelpers.ConfigureTestOptionsWithResources(&o,
		[]runtime.Object{},
		[]runtime.Object{&user1},
		&gits.GitFake{},
		&gits.FakeProvider{},
		helm_test.NewMockHelmer(),
		resources_test.NewMockInstaller(),
	)

	jxClient, ns, err := o.JXClientAndDevNamespace()
	assert.NoError(t, err)

	// Resolve returns the new user as the first one doesn't match because the emails are different
	_, err = users.Resolve("", "testProvider", jxClient, ns, func(id string, users []jenkinsv1.User) (string,
		[]jenkinsv1.User, *jenkinsv1.User, error) {
		// A real selector would actually check the emails are different, here we force it!
		return "test-user", []jenkinsv1.User{}, &user2, nil
	})
	assert.Error(t, err)
}
