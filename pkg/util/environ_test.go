// +build unit

package util_test

import (
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestGetAndCleanEnviron(t *testing.T) {
	tests := map[string]struct {
		setEnvs   map[string]string
		unsetEnvs []string
		want      map[string]string
		wantErr   bool
	}{

		"get and clean no env variables": {
			setEnvs: map[string]string{},
			want:    map[string]string{},
			wantErr: false,
		},
		"get and clean one env variables": {
			setEnvs: map[string]string{
				"TEST1": "value1",
			},
			want: map[string]string{
				"TEST1": "value1",
			},
			wantErr: false,
		},
		"get and clean multiple env variables": {
			setEnvs: map[string]string{
				"TEST1": "value1",
				"TEST2": "value2",
			},
			want: map[string]string{
				"TEST1": "value1",
				"TEST2": "value2",
			},
			wantErr: false,
		},
		"get and clean only set env variables": {
			setEnvs: map[string]string{
				"TEST1": "value1",
			},
			unsetEnvs: []string{
				"TEST2",
			},
			want: map[string]string{
				"TEST1": "value1",
			},
			wantErr: false,
		},
	}

	for description, tc := range tests {
		t.Run(description, func(t *testing.T) {
			for key, value := range tc.setEnvs {
				err := os.Setenv(key, value)
				assert.NoErrorf(t, err, "should set env variable '%s'='%s'", key, value)
			}
			envNames := append(tc.unsetEnvs, util.MapKeys(tc.setEnvs)...)
			envs, err := util.GetAndCleanEnviron(envNames)
			if tc.wantErr {
				assert.Error(t, err, "should return an error")
			} else {
				assert.NoError(t, err, "should not return an error")
			}
			assert.Equal(t, tc.want, envs, "should contained the expected cleaned keys")
			for key := range envs {
				_, set := os.LookupEnv(key)
				assert.Falsef(t, set, "env variable '%s' should not be set", key)
			}
		})
	}
}

func TestRetoreEnviron(t *testing.T) {
	tests := map[string]struct {
		environ map[string]string
		want    map[string]string
		wantErr bool
	}{
		"restore nonenv variable": {
			environ: map[string]string{},
			want:    map[string]string{},
			wantErr: false,
		},
		"restore one env variable": {
			environ: map[string]string{
				"TEST1": "value1",
			},
			want: map[string]string{
				"TEST1": "value1",
			},
			wantErr: false,
		},
		"restore multiple env variables": {
			environ: map[string]string{
				"TEST1": "value1",
				"TEST2": "value2",
			},
			want: map[string]string{
				"TEST1": "value1",
				"TEST2": "value2",
			},
			wantErr: false,
		},
	}

	for description, tc := range tests {
		t.Run(description, func(t *testing.T) {
			err := util.RestoreEnviron(tc.environ)
			if tc.wantErr {
				assert.Error(t, err, "should retun and error")
			} else {
				assert.NoError(t, err, "should not reutnr an error")
			}
			for wantKey, wantValue := range tc.want {
				value, set := os.LookupEnv(wantKey)
				assert.Truef(t, set, "should set env variable '%s'", wantKey)
				assert.Equalf(t, wantValue, value, "should set env variable '%s'='%s'", wantKey, wantValue)
				err := os.Unsetenv(wantKey)
				assert.NoError(t, err, "should unset the envariable '%s", wantKey)
			}
		})
	}
}
