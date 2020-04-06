// +build unit

package helm_test

import (
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/secreturl/localvault"
	"github.com/stretchr/testify/assert"
)

var expectedTemplatedValuesTree = `JenkinsXGitHub:
  password: myPipelineUserToken
  username: james
prow:
  hmacToken: abc
tekton:
  auth:
    git:
      password: myPipelineUserToken
      username: james
`

func TestValuesTreeTemplates(t *testing.T) {
	t.Parallel()

	testData := path.Join("test_data", "tree_of_values_yaml_templates")

	localVaultDir := path.Join(testData, "local_vault_files")
	secretURLClient := localvault.NewFileSystemClient(localVaultDir)

	result, _, err := helm.GenerateValues(config.NewRequirementsConfig(), nil, testData, nil, true, secretURLClient)
	assert.NoError(t, err)
	assert.Equal(t, expectedTemplatedValuesTree, string(result))
}
