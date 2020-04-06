// +build unit

package auth

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGitCredentials(t *testing.T) {
	fileName := filepath.Join("test_data", "git", "credentials")
	config, err := loadGitCredentialsAuthFile(fileName)
	require.NoError(t, err, "should not have failed to load file %s", fileName)
	assert.NotNil(t, config, "should have returned not nil config for file %s", fileName)

	serverURL := "http://cheese.com"
	gitKind := "bitbucketserver"
	username := "user2"
	password := "pwd2"

	assertServerUserPassword(t, config, "https://github.com", "user1", "pwd1")
	assertServerUserPassword(t, config, serverURL, username, password)

	// now lets test merging
	gitAuthConfig := &AuthConfig{
		Servers: []*AuthServer{
			{
				URL:  serverURL,
				Kind: gitKind,
				Users: []*UserAuth{
					{
						Username: username,
						Password: "oldPassword",
					},
				},
			},
		},
	}
	gitAuthConfig.Merge(config)

	assertServerUserPassword(t, gitAuthConfig, "https://github.com", "user1", "pwd1")
	serverAuth := assertServerUserPassword(t, gitAuthConfig, serverURL, username, password)
	require.NotNil(t, serverAuth, "no serverAuth found for URL %s", serverURL)

	// lets verify we still have the same kind
	assert.Equal(t, gitKind, serverAuth.Kind, "server.Kind for server URL after merge %s", serverURL)

}

func assertServerUserPassword(t *testing.T, config *AuthConfig, serverURL string, username string, password string) *AuthServer {
	server := config.GetServer(serverURL)
	require.NotNil(t, server, "no server found for URL %s", serverURL)

	userAuth := server.GetUserAuth(username)
	require.NotNil(t, server, "no user auth found for URL %s user %s", username, serverURL)

	assert.Equal(t, username, userAuth.Username, "userAuth.Username for URL %s", serverURL)
	assert.Equal(t, password, userAuth.Password, "userAuth.Password for URL %s", serverURL)
	assert.Equal(t, password, userAuth.ApiToken, "userAuth.ApiToken for URL %s", serverURL)

	t.Logf("found server %s username %s password %s", server.URL, userAuth.Username, userAuth.Password)
	return server
}

func TestLoadGitCredentialsFileDoesNotExist(t *testing.T) {
	config, err := loadGitCredentialsAuthFile("test_data/does/not/exist")
	require.NoError(t, err, "should not have failed to load non existing git creds file")
	assert.Nil(t, config, "should have returned nil config")
}
