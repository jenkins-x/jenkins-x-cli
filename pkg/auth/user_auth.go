package auth

import (
	"os"
	"strings"
)

const (
	usernameSuffix    = "_USERNAME"
	apiTokenSuffix    = "_API_TOKEN"
	bearerTokenSuffix = "_BEARER_TOKEN"
	DefaultUsername   = "dummy"
)

// UsernameEnv builds the username environment variable name
func UsernameEnv(prefix string) string {
	prefix = strings.ToUpper(prefix)
	return prefix + usernameSuffix
}

// ApiTokenEnv builds the api token environment variable name
func ApiTokenEnv(prefix string) string {
	prefix = strings.ToUpper(prefix)
	return prefix + apiTokenSuffix
}

// BearerTokenEnv builds the bearer token environment variable name
func BearerTokenEnv(prefix string) string {
	prefix = strings.ToUpper(prefix)
	return prefix + bearerTokenSuffix
}

// CreateAuthUserFromEnvironment creates a user auth from environment variables
func CreateAuthUserFromEnvironment(prefix string) UserAuth {
	user := UserAuth{}
	username, set := os.LookupEnv(UsernameEnv(prefix))
	if set {
		user.Username = username
	}
	apiToken, set := os.LookupEnv(ApiTokenEnv(prefix))
	if set {
		user.ApiToken = apiToken
	}
	bearerToken, set := os.LookupEnv(BearerTokenEnv(prefix))
	if set {
		user.BearerToken = bearerToken
	}

	if user.ApiToken != "" || user.Password != "" {
		if user.Username == "" {
			user.Username = DefaultUsername
		}
	}

	return user
}

// IsInvalid returns true if the user auth has a valid token
func (a *UserAuth) IsInvalid() bool {
	return a.BearerToken == "" && (a.ApiToken == "" || a.Username == "")
}

// Valid returns true when the user authentication is valid, otherwise false
func (a *UserAuth) IsValid() bool {
	if a.Username == "" {
		return false
	}
	if a.ApiToken == "" {
		return false
	}
	return true
}
