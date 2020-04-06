// +build unit

package gits_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	mocks "github.com/jenkins-x/jx/pkg/gits/mocks"
	utiltests "github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGitProviderFromURLForGitHubApp(t *testing.T) {
	// The git provider construct seems to also lookup the user auth into the
	// environment variables, which is causing this test to fail when executed
	// in parallel with other tests which set these environment variables.
	// t.Parallel()
	utiltests.SkipForWindows(t, "go-expect does not work on Windows")

	testData := []struct {
		ghOwner  string
		apiToken string
	}{
		{
			"owner1",
			"tokenOwner1",
		},
		{
			"owner2",
			"tokenOwner2",
		},
		{
			"owner3",
			"tokenOwner3",
		},
	}

	// lets assert that we can find tokens for github apps
	tests := []struct {
		ghOwner       string
		expectedToken string
		wantError     bool
	}{
		{
			"doesNotExist",
			"doesNotExist",
			true,
		},
		{
			"owner1",
			"tokenOwner1",
			false,
		},
		{
			"owner2",
			"tokenOwner2",
			false,
		},
		{
			"owner3",
			"tokenOwner3",
			false,
		},
	}

	gitServiceName := "gh"
	gitServerURL := "https://github.com"
	providerKind := "github"
	inCluster := true
	batchMode := true
	config := &auth.AuthConfig{}
	for _, td := range testData {
		server := &auth.AuthServer{
			URL:  gitServerURL,
			Name: gitServiceName,
			Kind: providerKind,
			Users: []*auth.UserAuth{
				{
					Username:       td.ghOwner,
					ApiToken:       td.apiToken,
					GithubAppOwner: td.ghOwner,
				},
			},
			CurrentUser: td.ghOwner,
		}
		config.AddServer(server)
	}

	authSvc := auth.NewMemoryAuthConfigService()
	authSvc.SetConfig(config)
	handles := util.IOFileHandles{}

	for _, tt := range tests {
		ghOwner := tt.ghOwner
		git := mocks.NewMockGitter()
		result, err := gits.CreateProviderForURL(inCluster, authSvc, providerKind, gitServerURL, ghOwner, git, batchMode, handles)
		if tt.wantError {
			assert.Error(t, err, "should fail to create provider for owner %s", ghOwner)
			assert.Nil(t, result, "created provider should be nil for owner %s", ghOwner)
		} else {
			assert.NoError(t, err, "should create provider without error for owner %s", ghOwner)
			require.NotNil(t, result, "created provider should not be nil for owner %s", ghOwner)
			userAuth := result.UserAuth()
			require.NotNil(t, userAuth, "provider userAuth should not be nil for owner %s", ghOwner)
			assert.Equal(t, tt.expectedToken, userAuth.ApiToken, "provider userAuth.ApiToken for owner %s", ghOwner)
		}
	}
}
