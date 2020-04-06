// +build unit

package config_test

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
)

func TestAdminSecrets(t *testing.T) {
	t.Parallel()

	testFile, err := ioutil.ReadFile("admin_secrets_test.yaml")
	assert.NoError(t, err)
	secretsFromFile := config.AdminSecretsConfig{}
	err = yaml.Unmarshal(testFile, &secretsFromFile)
	assert.NoError(t, err)

	service := config.AdminSecretsService{}
	service.Flags.DefaultAdminPassword = "mysecret"
	service.Flags.KanikoSecret = "kanikosecret"

	err = service.NewAdminSecretsConfig()
	assert.NoError(t, err)

	secretsFromService := service.Secrets
	tests.Debugf("%v", secretsFromService)

	assert.Equal(t, secretsFromFile, secretsFromService, "expected admin secret values do not match")
}
