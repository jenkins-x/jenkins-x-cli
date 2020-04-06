// +build unit

package auth_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	url1 = "http://dummy/"
	url2 = "http://another-jenkins/"

	userDoesNotExist = "doesNotExist"
	user1            = "someone"
	user2            = "another"
	token2v2         = "tokenV2"
)

func TestAuthConfig(t *testing.T) {
	t.Parallel()
	configTest := ConfigTest{
		t: t,
	}
	configTest.svc = auth.NewMemoryAuthConfigService()

	config := configTest.Load()

	assert.Equal(t, 0, len(config.Servers), "Should have no servers in config but got %v", config)
	assertNoAuth(t, config, url1, userDoesNotExist)

	auth1 := auth.UserAuth{
		Username: user1,
		ApiToken: "someToken",
	}
	config = configTest.SetUserAuth(url1, auth1)

	assert.Equal(t, 1, len(config.Servers), "Number of servers")
	assert.Equal(t, 1, len(config.Servers[0].Users), "Number of auths")
	assert.Equal(t, &auth1, config.FindUserAuth(url1, user1), "loaded auth for server %s and user %s", url1, user1)
	assert.Equal(t, &auth1, config.FindUserAuth(url1, ""), "loaded auth for server %s and no user", url1)

	auth2 := auth.UserAuth{
		Username: user2,
		ApiToken: "anotherToken",
	}
	config = configTest.SetUserAuth(url1, auth2)

	assert.Equal(t, &auth2, config.FindUserAuth(url1, user2), "Failed to find auth for server %s and user %s", url1, user2)
	assert.Equal(t, &auth1, config.FindUserAuth(url1, user1), "loaded auth for server %s and user %s", url1, user1)
	assert.Equal(t, 1, len(config.Servers), "Number of servers")
	assert.Equal(t, 2, len(config.Servers[0].Users), "Number of auths")
	assertNoAuth(t, config, url1, userDoesNotExist)

	// lets mutate the auth2
	auth2.ApiToken = token2v2
	config = configTest.SetUserAuth(url1, auth2)

	assertNoAuth(t, config, url1, userDoesNotExist)
	assert.Equal(t, 1, len(config.Servers), "Number of servers")
	assert.Equal(t, 2, len(config.Servers[0].Users), "Number of auths")
	assert.Equal(t, &auth1, config.FindUserAuth(url1, user1), "loaded auth for server %s and user %s", url1, user1)
	assert.Equal(t, &auth2, config.FindUserAuth(url1, user2), "loaded auth for server %s and user %s", url1, user2)

	auth3 := auth.UserAuth{
		Username: user1,
		ApiToken: "server2User1Token",
	}
	configTest.SetUserAuth(url2, auth3)

	assertNoAuth(t, config, url1, userDoesNotExist)
	assert.Equal(t, 2, len(config.Servers), "Number of servers")
	assert.Equal(t, 2, len(config.Servers[0].Users), "Number of auths for server 0")
	assert.Equal(t, 1, len(config.Servers[1].Users), "Number of auths for server 1")
	assert.Equal(t, &auth1, config.FindUserAuth(url1, user1), "loaded auth for server %s and user %s", url1, user1)
	assert.Equal(t, &auth2, config.FindUserAuth(url1, user2), "loaded auth for server %s and user %s", url1, user2)
	assert.Equal(t, &auth3, config.FindUserAuth(url2, user1), "loaded auth for server %s and user %s", url2, user1)
}

type ConfigTest struct {
	t   *testing.T
	svc auth.ConfigService
}

func (c *ConfigTest) Load() *auth.AuthConfig {
	config, err := c.svc.LoadConfig()
	require.NoError(c.t, err)
	return config
}

func (c *ConfigTest) SetUserAuth(url string, auth auth.UserAuth) *auth.AuthConfig {
	copy := auth
	c.svc.Config().SetUserAuth(url, &copy)
	c.SaveAndReload()
	return c.svc.Config()
}

func (c *ConfigTest) SaveAndReload() *auth.AuthConfig {
	err := c.svc.SaveConfig()
	require.NoError(c.t, err)
	return c.Load()
}

func assertNoAuth(t *testing.T, config *auth.AuthConfig, url string, user string) {
	found := config.FindUserAuth(url, user)
	if found != nil {
		assert.Fail(t, "Found auth when not expecting it for server %s and user %s", url, user)
	}
}

func TestAuthConfigGetsDefaultName(t *testing.T) {
	t.Parallel()
	c := &auth.AuthConfig{}

	expectedURL := "https://foo.com"
	server := c.GetOrCreateServer(expectedURL)
	assert.NotNil(t, server, "No server found!")
	assert.True(t, server.Name != "", "Should have a server name!")
	assert.Equal(t, expectedURL, server.URL, "Server.URL")
}

func TestDeleteServer(t *testing.T) {
	t.Parallel()
	c := &auth.AuthConfig{}
	url := "https://foo.com"
	server := c.GetOrCreateServer(url)
	assert.NotNil(t, server, "Failed to add the server to the configuration")
	assert.Equal(t, 1, len(c.Servers), "No server found in the configuration")

	c.DeleteServer(url)
	assert.Equal(t, 0, len(c.Servers), "Failed to remove the server from configuration")
	assert.Equal(t, "", c.CurrentServer, "Should be no current server")
}

func TestDeleteServer2(t *testing.T) {
	t.Parallel()
	c := &auth.AuthConfig{}
	url1 := "https://foo1.com"
	server1 := c.GetOrCreateServer(url1)
	assert.NotNil(t, server1, "Failed to add the server to the configuration")
	url2 := "https://foo2.com"
	server2 := c.GetOrCreateServer(url2)
	assert.NotNil(t, server2, "Failed to the server to the configuration!")
	assert.Equal(t, 2, len(c.Servers), "Must have 2 servers in the configuration")
	c.CurrentServer = url2

	c.DeleteServer(url2)
	assert.Equal(t, 1, len(c.Servers), "Failed to remove one server from configuration")
	assert.Equal(t, url1, c.Servers[0].URL, "Failed to remove the right server from the configuration")
	assert.Equal(t, url1, c.CurrentServer, "Server 1 should be current server")
}
