// +build unit

package auth_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func setEnvs(t *testing.T, envs map[string]string) {
	err := util.RestoreEnviron(envs)
	assert.NoError(t, err, "should set the environment variables")
}

func cleanEnvs(t *testing.T, envs []string) {
	_, err := util.GetAndCleanEnviron(envs)
	assert.NoError(t, err, "shuold clean the environment variables")
}

func TestCreateAuthUserFromEnvironment(t *testing.T) {
	const prefix = "TEST"
	tests := map[string]struct {
		prefix  string
		setup   func(t *testing.T)
		cleanup func(t *testing.T)
		want    auth.UserAuth
	}{
		"create auth user from environment with api token": {
			prefix: prefix,
			setup: func(t *testing.T) {
				setEnvs(t, map[string]string{
					auth.UsernameEnv(prefix): "test",
					auth.ApiTokenEnv(prefix): "test",
				})
			},
			cleanup: func(t *testing.T) {
				cleanEnvs(t, []string{
					auth.UsernameEnv(prefix),
					auth.ApiTokenEnv(prefix),
				})
			},
			want: auth.UserAuth{
				Username:    "test",
				ApiToken:    "test",
				BearerToken: "",
				Password:    "",
			},
		},
		"create auth user from environment with bearer token": {
			prefix: prefix,
			setup: func(t *testing.T) {
				setEnvs(t, map[string]string{
					auth.UsernameEnv(prefix):    "test",
					auth.BearerTokenEnv(prefix): "test",
				})
			},
			cleanup: func(t *testing.T) {
				cleanEnvs(t, []string{
					auth.UsernameEnv(prefix),
					auth.BearerTokenEnv(prefix),
				})
			},
			want: auth.UserAuth{
				Username:    "test",
				ApiToken:    "",
				BearerToken: "test",
				Password:    "",
			},
		},
		"create auth user from environment with default name": {
			prefix: prefix,
			setup: func(t *testing.T) {
				setEnvs(t, map[string]string{
					auth.ApiTokenEnv(prefix): "test",
				})
			},
			cleanup: func(t *testing.T) {
				cleanEnvs(t, []string{
					auth.ApiTokenEnv(prefix),
				})
			},
			want: auth.UserAuth{
				Username:    auth.DefaultUsername,
				ApiToken:    "test",
				BearerToken: "",
				Password:    "",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}

			user := auth.CreateAuthUserFromEnvironment(prefix)
			assert.Equal(t, tc.want, user)

			if tc.cleanup != nil {
				tc.cleanup(t)
			}
		})
	}
}

func TestIsInvalid(t *testing.T) {
	tests := map[string]struct {
		user *auth.UserAuth
		want bool
	}{
		"invalid user when empty": {
			user: &auth.UserAuth{},
			want: true,
		},
		"invalid user with only a username": {
			user: &auth.UserAuth{
				Username: "test",
			},
			want: true,
		},
		"invalid user with only a api token": {
			user: &auth.UserAuth{
				ApiToken: "test",
			},
			want: true,
		},
		"valid user with only a bearer token": {
			user: &auth.UserAuth{
				BearerToken: "test",
			},
			want: false,
		},
		"valid user with api token": {
			user: &auth.UserAuth{
				Username: "test",
				ApiToken: "test",
			},
			want: false,
		},
		"valid user with bearer token": {
			user: &auth.UserAuth{
				Username:    "test",
				BearerToken: "test",
			},
			want: false,
		},
		"valid user with api token and bearer token": {
			user: &auth.UserAuth{
				Username:    "test",
				ApiToken:    "test",
				BearerToken: "test",
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.user.IsInvalid()
			msg := ""
			if tc.want {
				msg = "user should be invalid"
			} else {
				msg = "user should be valid"
			}
			assert.Equalf(t, tc.want, result, msg)
		})
	}
}
