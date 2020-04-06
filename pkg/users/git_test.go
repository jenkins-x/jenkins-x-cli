// +build unit

package users_test

import (
	"fmt"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/jenkins-x/jx/pkg/users"

	uuid "github.com/satori/go.uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestFindUserByLabel(t *testing.T) {
	t.Parallel()
	resolver, _, err := prepare(t)
	assert.NoError(t, err)

	gituserIDUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID := gituserIDUUID.String()

	// Create the user
	labeleduserID, err := createUniqueDummyUser(resolver, true, gituserID)
	defer func() {
		err := removeDummyUser(labeleduserID, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	unLabeleduserID, err := createUniqueDummyUser(resolver, false, gituserID)
	defer func() {
		err := removeDummyUser(unLabeleduserID, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	gitUser := gits.GitUser{
		Login: gituserID,
	}
	user, err := resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	// Validate that we found the labeled one, not the unlabeled one
	assert.Equal(t, labeleduserID, user.Spec.Login)
}

func TestFindUserBySignature(t *testing.T) {
	t.Parallel()
	resolver, _, err := prepare(t)
	assert.NoError(t, err)

	gituserIDUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID := gituserIDUUID.String()

	// Create the user
	userID, err := createUniqueDummyUser(resolver, true, gituserID)
	defer func() {
		err := removeDummyUser(userID, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	signature := object.Signature{
		Email: fmt.Sprintf("%s@test.com", userID),
		Name:  userID,
	}
	user, err := resolver.GitSignatureAsUser(&signature)
	assert.NoError(t, err)
	// Validate that we found the labeled one, not the unlabeled one
	assert.Equal(t, userID, user.Spec.Login)
}

func TestFindUserByAccountReference(t *testing.T) {
	t.Parallel()
	resolver, _, err := prepare(t)
	assert.NoError(t, err)

	gituserID1UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID1 := gituserID1UUID.String()

	gituserID2UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID2 := gituserID2UUID.String()

	// Create the user
	userID1, err := createUniqueDummyUser(resolver, false, gituserID1)
	defer func() {
		err := removeDummyUser(userID1, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	userID2, err := createUniqueDummyUser(resolver, false, gituserID2)
	defer func() {
		err := removeDummyUser(userID2, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)

	gitUser := gits.GitUser{
		Login: gituserID1,
	}
	user, err := resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	// Validate that we found the correct user
	assert.Equal(t, userID1, user.Spec.Login)
	// Validate that the label was added
	val, ok := user.Labels[resolver.GitProviderKey()]
	assert.True(t, ok)
	assert.Equal(t, gituserID1, val)
}

func TestFindUserByFromGitProvider(t *testing.T) {
	t.Parallel()
	resolver, fakeProvider, err := prepare(t)
	assert.NoError(t, err)

	gituserID1UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID1 := gituserID1UUID.String()

	gituserID2UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID2 := gituserID2UUID.String()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	emailUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	email := emailUUID.String()

	// Create the user
	gitUser1 := &gits.GitUser{
		Name:  name,
		Email: fmt.Sprintf("%s@test.com", email),
		Login: gituserID1,
	}
	fakeProvider.Users = []*gits.GitUser{
		gitUser1,
	}
	userID2, err := createUniqueDummyUser(resolver, false, gituserID2)
	defer func() {
		err := removeDummyUser(userID2, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	gitUser := gits.GitUser{
		Login: gituserID1,
	}
	user, err := resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	// Validate that we create a new user from the git provider
	assert.Equal(t, gitUser1.Email, user.Spec.Email)
	assert.Equal(t, gitUser1.Name, user.Spec.Name)
	// Validate that the label was added
	val, ok := user.Labels[resolver.GitProviderKey()]
	assert.True(t, ok)
	assert.Equal(t, gituserID1, val)
	// Validate that the account reference is created
	assert.Len(t, user.Spec.Accounts, 1)
	assert.Equal(t, user.Spec.Accounts[0].Provider, resolver.GitProviderKey())
	assert.Equal(t, user.Spec.Accounts[0].ID, gituserID1)
}

func TestFindUserByFromGitProviderWithNoEmail(t *testing.T) {
	t.Parallel()
	resolver, fakeProvider, err := prepare(t)
	assert.NoError(t, err)

	gituserID1UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID1 := gituserID1UUID.String()

	gituserID2UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	gituserID2 := gituserID2UUID.String()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	userID2UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	userID2 := userID2UUID.String()
	assert.NoError(t, err)
	gitUser1 := &gits.GitUser{
		Name:  name,
		Email: "",
		Login: gituserID1,
	}
	fakeProvider.Users = []*gits.GitUser{
		gitUser1,
	}
	err = createDummyUser(resolver, true, gituserID2, userID2, "", userID2, userID2)
	defer func() {
		err := removeDummyUser(userID2, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	gitUser := gits.GitUser{
		Login: gituserID1,
	}
	user, err := resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	// Validate that we don't attach the two users
	assert.NotEqual(t, user.Name, userID2)
}

func TestFindUserWithDifferentEmailButSameGitLogin(t *testing.T) {
	t.Parallel()
	resolver, fakeProvider, err := prepare(t)
	assert.NoError(t, err)

	userID1UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	userID := userID1UUID.String()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	gitUser1 := &gits.GitUser{
		Name:  name,
		Email: fmt.Sprintf("%s@test.com", name),
		Login: userID,
	}
	fakeProvider.Users = []*gits.GitUser{
		gitUser1,
	}
	err = createDummyUser(resolver, true, "", userID, fmt.Sprintf("%s@acme.com", userID), userID, userID)
	defer func() {
		err := removeDummyUser(userID, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	gitUser := gits.GitUser{
		Login: gitUser1.Login,
		Email: gitUser1.Email,
	}
	_, err = resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	users, err := resolver.JXClient.JenkinsV1().Users(resolver.Namespace).List(metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, users.Items, 2)
	userIds := make([]string, 0)
	for _, u := range users.Items {
		userIds = append(userIds, u.Name)
	}
}

func TestFindUserWithNoEmailButSameGitLogin(t *testing.T) {
	t.Parallel()
	resolver, fakeProvider, err := prepare(t)
	assert.NoError(t, err)

	userID1UUID, err := uuid.NewV4()
	assert.NoError(t, err)
	userID := userID1UUID.String()
	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	gitUser1 := &gits.GitUser{
		Name:  name,
		Email: "",
		Login: userID,
	}
	fakeProvider.Users = []*gits.GitUser{
		gitUser1,
	}
	err = createDummyUser(resolver, true, "", userID, fmt.Sprintf("%s@test.com", name), userID, userID)
	defer func() {
		err := removeDummyUser(userID, resolver)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	gitUser := gits.GitUser{
		Login: gitUser1.Login,
		Email: gitUser1.Email,
	}
	_, err = resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	users, err := resolver.JXClient.JenkinsV1().Users(resolver.Namespace).List(metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, users.Items, 2)
}

func TestFindUserByUpperCaseGitUserName(t *testing.T) {
	t.Parallel()
	resolver, _, err := prepare(t)
	assert.NoError(t, err)
	assert.NoError(t, err)

	gitUser := gits.GitUser{
		Login: "foo",
		Name:  "John",
		Email: "john@acme.com",
	}

	user, err := resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, user.Spec.Login, "foo")
	assert.Contains(t, user.Labels, "jenkins.io/git-fakegit-userid")
	assert.Equal(t, user.Spec.Name, "John")
	assert.Equal(t, user.Spec.Email, "john@acme.com")

	gitUser = gits.GitUser{
		Login: "Foo",
		Name:  "Jane",
		Email: "jane@acme.com",
	}

	user, err = resolver.Resolve(&gitUser)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Contains(t, user.Labels, "jenkins.io/git-fakegit-userid")
	assert.Equal(t, user.Spec.Login, "Foo")
	assert.Equal(t, user.Spec.Name, "Jane")
	assert.Equal(t, user.Spec.Email, "jane@acme.com")
}

func prepare(t *testing.T) (*users.GitUserResolver, *gits.FakeProvider, error) {
	testOrgName := "myorg"
	testRepoName := "my-app"
	fakeRepo, _ := gits.NewFakeRepository(testOrgName, testRepoName, nil, nil)
	fakeProvider := gits.NewFakeProvider(fakeRepo)
	fakeProvider.Type = gits.Fake

	o := opts.CommonOptions{}
	testhelpers.ConfigureTestOptionsWithResources(&o,
		[]runtime.Object{},
		[]runtime.Object{},
		&gits.GitFake{},
		fakeProvider,
		helm_test.NewMockHelmer(),
		resources_test.NewMockInstaller(),
	)

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return nil, nil, err
	}

	return &users.GitUserResolver{
		GitProvider: fakeProvider,
		JXClient:    jxClient,
		Namespace:   ns,
	}, fakeProvider, nil
}

func createDummyUser(resolver *users.GitUserResolver, createLabels bool, gituserID string, name string,
	email string, login string, metaName string) error {

	spec := jenkinsv1.UserDetails{
		Name:  name,
		Email: email,
		Login: login,
	}
	meta := metav1.ObjectMeta{
		Name: metaName,
	}
	if gituserID != "" {
		spec.Accounts = []jenkinsv1.AccountReference{
			{
				ID:       gituserID,
				Provider: resolver.GitProviderKey(),
			},
		}

		if createLabels {
			meta.Labels = map[string]string{
				resolver.GitProviderKey(): gituserID,
			}
		}
	}
	_, err := resolver.JXClient.JenkinsV1().Users(resolver.Namespace).Create(&jenkinsv1.User{
		Spec:       spec,
		ObjectMeta: meta,
	})
	return err
}

func createUniqueDummyUser(resolver *users.GitUserResolver, createLabels bool, gituserID string) (string, error) {
	idUUID, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	id := idUUID.String()
	return id, createDummyUser(resolver, createLabels, gituserID, id, fmt.Sprintf("%s@test.com", id), id, id)
}

func removeDummyUser(id string, resolver *users.GitUserResolver) error {
	return resolver.JXClient.JenkinsV1().Users(resolver.Namespace).Delete(id, &metav1.DeleteOptions{})
}
