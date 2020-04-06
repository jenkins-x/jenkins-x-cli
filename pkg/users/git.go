package users

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/pkg/errors"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/src-d/go-git.v4/plumbing/object"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/log"

	jenkninsv1client "github.com/jenkins-x/jx/pkg/client/clientset/versioned"

	"github.com/jenkins-x/jx/pkg/gits"
)

// GitUserResolver allows git users to be converted to Jenkins X users
type GitUserResolver struct {
	GitProvider gits.GitProvider
	JXClient    jenkninsv1client.Interface
	Namespace   string
}

// GitSignatureAsUser resolves the signature to a Jenkins X User
func (r *GitUserResolver) GitSignatureAsUser(signature *object.Signature) (*jenkinsv1.User, error) {
	// We can't resolve no info so shortcircuit
	if signature.Name == "" && signature.Email == "" {
		return nil, errors.Errorf("both name and email are empty")
	}
	gitUser := &gits.GitUser{
		Email: signature.Email,
		Name:  signature.Name,
	}
	return r.Resolve(gitUser)
}

// GitUserSliceAsUserDetailsSlice resolves a slice of git users to a slice of Jenkins X User Details
func (r *GitUserResolver) GitUserSliceAsUserDetailsSlice(users []gits.GitUser) ([]jenkinsv1.UserDetails, error) {
	answer := []jenkinsv1.UserDetails{}
	for _, user := range users {
		u, err := r.Resolve(&user)
		if err != nil {
			return nil, err
		}
		if u != nil {
			answer = append(answer, u.Spec)
		}
	}
	return answer, nil
}

// Resolve will convert the GitUser to a Jenkins X user and attempt to complete the user info by:
// * checking the user custom resources to see if the user is present there
// * making a call to the gitProvider
// as often user info is not complete in a git response
func (r *GitUserResolver) Resolve(user *gits.GitUser) (*jenkinsv1.User, error) {
	if r == nil || user == nil {
		return nil, nil
	}
	selectUsers := func(id string, users []jenkinsv1.User) (string, []jenkinsv1.User,
		*jenkinsv1.User, error) {
		var gitUser *gits.GitUser
		if user.Login != "" {
			gitUser = r.GitProvider.UserInfo(user.Login)
		}
		if gitUser == nil {
			gitUser = user
		}

		possibles := make([]jenkinsv1.User, 0)
		if gitUser == nil {
			// annoyingly UserInfo swallows the error, so we recreate it!
			log.Logger().Warnf("unable to find user with login %s from %s", user.Login, r.GitProvider.Kind())
		} else if user.Email != "" {
			// Don't do this if email is empty as otherwise we risk matching any users who have empty emails!
			for _, u := range users {
				if u.Spec.Email == gitUser.Email {
					possibles = append(possibles, u)
				}
			}
		}
		new := r.GitUserToUser(gitUser)
		id = naming.ToValidName(gitUser.Login)
		// Check if the user id is available, if not append "-<n>" where <n> is some integer
		for i := 0; true; i++ {
			_, err := r.JXClient.JenkinsV1().Users(r.Namespace).Get(id, v1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				break
			}
			id = fmt.Sprintf("%s-%d", gitUser.Login, i)
		}
		new.Name = naming.ToValidName(id)
		return id, possibles, new, nil
	}
	user.Login = naming.ToValidValue(user.Login)
	return Resolve(user.Login, r.GitProviderKey(), r.JXClient, r.Namespace, selectUsers)
}

// UpdateUserFromPRAuthor will attempt to use the
func (r *GitUserResolver) UpdateUserFromPRAuthor(author *jenkinsv1.User, pullRequest *gits.GitPullRequest,
	commits []*gits.GitCommit) (*jenkinsv1.User, error) {

	if pullRequest != nil {
		updated := false
		if author != nil {
			gitLogin := r.GitUserLogin(author)
			if gitLogin == "" {
				gitLogin = author.Spec.Login
			}
			for _, commit := range commits {
				if commit.Author != nil && gitLogin == commit.Author.Login {
					log.Logger().Info("Found commit author match for: " + author.
						Spec.Login + " with email address: " + commit.Author.Email + "\n")
					author.Spec.Email = commit.Author.Email
					updated = true
					break
				}
			}
		}
		if updated {
			return r.JXClient.JenkinsV1().Users(r.Namespace).PatchUpdate(author)
		}
	}
	return author, nil
}

// UserToGitUser performs type conversion from a Jenkins X User to a Git User
func (r *GitUserResolver) UserToGitUser(id string, user *jenkinsv1.User) *gits.GitUser {
	return &gits.GitUser{
		Login:     id,
		Email:     user.Spec.Email,
		Name:      user.Spec.Name,
		URL:       user.Spec.URL,
		AvatarURL: user.Spec.AvatarURL,
	}
}

// GitUserToUser performs type conversion from a GitUser to a Jenkins X user,
// attaching the Git Provider account to Accounts
func (r *GitUserResolver) GitUserToUser(gitUser *gits.GitUser) *jenkinsv1.User {
	user := CreateUser(r.Namespace, gitUser.Login, gitUser.Name, gitUser.Email)
	return AddAccountReference(user, r.GitProviderKey(), gitUser.Login)
}

// GitUserLogin returns the login for the git provider, or an empty string if not found
func (r *GitUserResolver) GitUserLogin(user *jenkinsv1.User) string {
	for _, a := range user.Spec.Accounts {
		if a.Provider == r.GitProviderKey() {
			return a.ID
		}
	}
	return ""
}

// GitProviderKey returns the provider key for this GitUserResolver
func (r *GitUserResolver) GitProviderKey() string {
	if r == nil || r.GitProvider == nil {
		return ""
	}
	return fmt.Sprintf("jenkins.io/git-%s-userid", r.GitProvider.Kind())
}

// mergeGitUsers merges user1 into user2, replacing any that do not have empty values on user2 with those from user1
