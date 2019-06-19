package helm_test

import (
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/secreturl/localvault"
	"github.com/stretchr/testify/assert"
)

var expectedTemplatedValuesTree = `dummy: cheese
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
	secretUrlClient := localvault.NewFileSystemClient(localVaultDir)

	result, err := helm.GenerateValues(testData, nil, true, secretUrlClient)
	assert.NoError(t, err)
	assert.Equal(t, expectedTemplatedValuesTree, string(result))
}
