// +build unit

package secreturl_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/jenkins-x/jx/pkg/secreturl"
	"github.com/jenkins-x/jx/pkg/secreturl/fakevault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var uriRegexp = regexp.MustCompile(`:[\s"]*vault:[-_.\w\/:]*`)

const schemaPrefix = "vault:"

func TestReplaceURIs(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testValue := "test"
	testKey := "vault:cluster/admin:password"
	_, err := secretClient.Write("cluster/admin", map[string]interface{}{"password": testValue})
	require.NoError(t, err)

	testString := `
user: test
password: %s
`
	result, err := secreturl.ReplaceURIs(fmt.Sprintf(testString, testKey), secretClient, uriRegexp, schemaPrefix)
	assert.NoError(t, err, "should replace the URIs without error")
	assert.EqualValues(t, fmt.Sprintf(testString, testValue), result, "should replace the URIs")
}

func TestReplaceURIsWithQuotation(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testValue := "test"
	testKey := "vault:cluster/admin:password"
	_, err := secretClient.Write("cluster/admin", map[string]interface{}{"password": testValue})
	require.NoError(t, err)

	testString := `
user: test
password: "%s"
`
	result, err := secreturl.ReplaceURIs(fmt.Sprintf(testString, testKey), secretClient, uriRegexp, schemaPrefix)
	assert.NoError(t, err, "should replace the URIs without error")
	assert.EqualValues(t, fmt.Sprintf(testString, testValue), result, "should replace the URIs")
}
func TestReplaceURIsWithoutReplacements(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testValue := "test"
	testString := `
user: test
password: %s
`
	result, err := secreturl.ReplaceURIs(fmt.Sprintf(testString, testValue), secretClient, uriRegexp, schemaPrefix)
	assert.NoError(t, err, "should replace the URIs without error")
	assert.EqualValues(t, fmt.Sprintf(testString, testValue), result, "should replace the URIs")
}

func TestReplaceURIsWithoutKey(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testValue := "test"
	testKey := "vault:cluster/admin:"
	_, err := secretClient.Write("cluster/admin", map[string]interface{}{"password": testValue})
	require.NoError(t, err)

	testString := `
user: test
password: %s
`
	_, err = secreturl.ReplaceURIs(fmt.Sprintf(testString, testKey), secretClient, uriRegexp, schemaPrefix)
	assert.Error(t, err, "should fail when no URIs key is found")
}

func TestReplaceURIsNoValueFoundInVault(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testKey := "vault:cluster/admin:password"

	testString := `
user: test
password: %s
`
	_, err := secreturl.ReplaceURIs(fmt.Sprintf(testString, testKey), secretClient, uriRegexp, schemaPrefix)
	assert.Error(t, err, "should fail when no value is found in vault")
}

func TestReplaceURIsWhenNoSecretFoundInVault(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testValue := "test"
	testKey := "vault:cluster/admin:password"
	_, err := secretClient.Write("cluster/admin", map[string]interface{}{"token": testValue})
	require.NoError(t, err)

	testString := `
user: test
password: %s
`
	_, err = secreturl.ReplaceURIs(fmt.Sprintf(testString, testKey), secretClient, uriRegexp, schemaPrefix)
	assert.Error(t, err, "should replace the URIs without error")
}

func TestReplaceURIsSchemaIsYamlKeyWithoutValue(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testString := `
user: test
vault: 
  enabled: true
`
	result, err := secreturl.ReplaceURIs(testString, secretClient, uriRegexp, schemaPrefix)
	assert.NoError(t, err, "should replace the URIs without error")
	assert.EqualValues(t, testString, result, "should replace the URIs")
}

func TestReplaceURIsSchemaIsYamlKeyWithValue(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testString := `
user: test
vault: test 
`
	result, err := secreturl.ReplaceURIs(testString, secretClient, uriRegexp, schemaPrefix)
	assert.NoError(t, err, "should replace the URIs without error")
	assert.EqualValues(t, testString, result, "should replace the URIs")
}

func TestReplaceURIsWithClusterDotsInName(t *testing.T) {
	secretClient := fakevault.NewFakeClient()

	testValue := "test"
	testKey := "vault:cluster.kops.k8s.local/admin:password"
	_, err := secretClient.Write("cluster.kops.k8s.local/admin", map[string]interface{}{"password": testValue})
	require.NoError(t, err)

	testString := `
user: test
password: %s
`
	result, err := secreturl.ReplaceURIs(fmt.Sprintf(testString, testKey), secretClient, uriRegexp, schemaPrefix)
	assert.NoError(t, err, "should replace the URIs without error")
	assert.EqualValues(t, fmt.Sprintf(testString, testValue), result, "should replace the URIs")
}
