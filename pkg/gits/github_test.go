// +build unit

package gits

import (
	"testing"

	"github.com/google/go-github/github"

	"github.com/stretchr/testify/assert"
)

func TestIsOwnerGitHubUser_isOwner(t *testing.T) {
	t.Parallel()
	isOwnerGitHubUser := IsOwnerGitHubUser("owner", "owner")
	assert.True(t, isOwnerGitHubUser, "The owner should be the same as the GitHubUser")
}

func TestIsOwnerGitHubUser_isNotOwner(t *testing.T) {
	t.Parallel()
	isOwnerGitHubUser := IsOwnerGitHubUser("owner", "notowner")
	assert.False(t, isOwnerGitHubUser, "The owner must not be the same as the GitHubUser")
}

func TestExtractRepositoryCommitAuthor(t *testing.T) {
	tests := []struct {
		testName         string
		commitAuthor     *github.CommitAuthor
		repositoryAuthor *github.User
		want             *GitUser
	}{
		{
			"no repository author",
			&github.CommitAuthor{
				Name:  github.String("John Doe"),
				Email: github.String("jdoe@example.com"),
			},
			nil,
			&GitUser{
				URL:       "",
				Login:     "",
				Name:      "John Doe",
				Email:     "jdoe@example.com",
				AvatarURL: "",
			},
		},
		{
			"repository and commit author match",
			&github.CommitAuthor{
				Name:  github.String("John Doe"),
				Email: github.String("jdoe@example.com"),
			},
			&github.User{
				Login:     github.String("jdoe"),
				Email:     github.String("jdoe@example.com"),
				Name:      github.String("Foo Bar"),
				URL:       github.String("https://github.com/foobar"),
				AvatarURL: github.String("https://avatars.github.com/foobar"),
			},
			&GitUser{
				URL:       "https://github.com/foobar",
				Login:     "jdoe",
				Name:      "John Doe",
				Email:     "jdoe@example.com",
				AvatarURL: "https://avatars.github.com/foobar",
			},
		},
	}

	for _, test := range tests {
		rc := github.RepositoryCommit{
			Commit: &github.Commit{
				Author: test.commitAuthor,
			},
			Author: test.repositoryAuthor,
		}

		got := extractRepositoryCommitAuthor(&rc)
		assert.Equal(t, test.want, got, test.testName)
	}
}
