// +build unit

package verify_test

import (
	"os"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/verify"
	"github.com/jenkins-x/jx/pkg/secreturl/fakevault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createStepVerifyValuesOptions(test string) *verify.StepVerifyValuesOptions {
	dir := path.Join("test_data", "verify_values", test)
	options := verify.StepVerifyValuesOptions{
		SchemaFile:      path.Join(dir, "schema.json"),
		RequirementsDir: dir,
		ValuesFile:      path.Join(dir, "values.yaml"),
		SecretClient:    fakevault.NewFakeClient(),
	}
	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	commonOpts.Out = os.Stdout
	commonOpts.Err = os.Stderr
	options.CommonOptions = &commonOpts
	return &options
}

func TestStepVerifyValuesWithValidValues(t *testing.T) {
	t.Parallel()

	options := createStepVerifyValuesOptions("valid_values")
	err := options.Run()

	assert.NoError(t, err, "Command failed: %v", options)
}

func TestStepVerifyValuesWithErrorValues(t *testing.T) {
	t.Parallel()

	options := createStepVerifyValuesOptions("error_values")
	err := options.Run()

	assert.Error(t, err, "Command didn't fail: %v", options)
}

func TestStepVerifyValuesWithValidSecretValues(t *testing.T) {
	t.Parallel()

	options := createStepVerifyValuesOptions("secret_values")

	_, err := options.SecretClient.Write("cluster/adminUser", map[string]interface{}{"password": "test"})
	require.NoError(t, err)
	_, err = options.SecretClient.Write("cluster/pipelineUser", map[string]interface{}{"token": "aaaaa12345bbbbb12345ccccc12345ddddd12345"})
	require.NoError(t, err)

	err = options.Run()

	assert.NoError(t, err, "Command failed: %v", options)
}

func TestStepVerifyValuesWithMissingSecretValues(t *testing.T) {
	t.Parallel()

	options := createStepVerifyValuesOptions("secret_values")

	_, err := options.SecretClient.Write("cluster/adminUser", map[string]interface{}{"password": "test"})
	require.NoError(t, err)

	err = options.Run()

	assert.Error(t, err, "Command didn't fail: %v", options)
}

func TestStepVerifyValuesWithErrorSecretValues(t *testing.T) {
	t.Parallel()

	options := createStepVerifyValuesOptions("secret_values")

	_, err := options.SecretClient.Write("cluster/adminUser", map[string]interface{}{"password": "test"})
	require.NoError(t, err)
	_, err = options.SecretClient.Write("cluster/pipelineUser", map[string]interface{}{"token": "123"})
	require.NoError(t, err)

	err = options.Run()

	assert.Error(t, err, "Command didn't fail: %v", options)
}
