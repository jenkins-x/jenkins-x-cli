package vault_test

import (
	"github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/jenkins-x/jx/pkg/vault"
	"github.com/jenkins-x/jx/pkg/vault/test_utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetConfigData(t *testing.T) {
	vaultOperatorClient, factory, err, kubeClient := test_utils.SetupMocks(t, nil)

	vaultName, namespace := "myVault", "myVaultNamespace"
	test_utils.CreateMockedVault(vaultName, namespace, "foo.bar", "myJWT", vaultOperatorClient, kubeClient)

	// Invoke the function under test
	config, jwt, saName, err := factory.GetConfigData(vaultName, namespace)

	assert.Equal(t, "http://foo.bar", config.Address)
	assert.Equal(t, "myJWT", jwt)
	assert.Equal(t, "myVault-auth-sa", saName)
	assert.NoError(t, err)
}

func TestGetConfigData_DefaultNamespacesUsed(t *testing.T) {
	vaultOperatorClient, factory, err, kubeClient := test_utils.SetupMocks(t, nil)

	vaultName, namespace := "myVault", "jx" // "jx" is the default namespace used by the kubeClient
	test_utils.CreateMockedVault(vaultName, namespace, "foo.bar", "myJWT", vaultOperatorClient, kubeClient)

	// Invoke the function under test
	config, jwt, saName, err := factory.GetConfigData("", "")

	assert.Equal(t, "http://foo.bar", config.Address)
	assert.Equal(t, "myJWT", jwt)
	assert.Equal(t, "myVault-auth-sa", saName)
	assert.NoError(t, err)
}

func TestGetConfigData_ErrorsWhenNoVaultsInNamespace(t *testing.T) {
	vaultOperatorClient, factory, err, kubeClient := test_utils.SetupMocks(t, nil)

	vaultName, namespace := "myVault", "myVaultNamespace"
	test_utils.CreateMockedVault(vaultName, namespace, "foo.bar", "myJWT", vaultOperatorClient, kubeClient)

	// Invoke the function under test
	config, jwt, saName, err := factory.GetConfigData("", "Nothing In This Namespace")

	assert.Nil(t, config)
	assert.Empty(t, jwt)
	assert.Empty(t, saName)
	assert.EqualError(t, err, "no vaults found in namespace 'Nothing In This Namespace'")
}

func TestGetConfigData_ConfigUsedFromVaultSelector(t *testing.T) {
	// Two vaults are configured in the same namespace, the user specifies one with the -m flag
	vaultOperatorClient, factory, err, kubeClient := test_utils.SetupMocks(t, nil)

	namespace := "myVaultNamespace"
	_ = test_utils.CreateMockedVault("vault1", namespace, "one.ah.ah.ah", "count", vaultOperatorClient, kubeClient)
	vault2 := test_utils.CreateMockedVault("vault2", namespace, "two.ah.ah.ah", "von-count", vaultOperatorClient, kubeClient)

	// Create a mock Selector that just returns the second vault
	factory.Selector = PredefinedVaultSelector{vaultToReturn: vault2, url: "http://two.ah.ah.ah"}

	// Invoke the function under test
	config, jwt, saName, err := factory.GetConfigData("", namespace)

	assert.Equal(t, "http://two.ah.ah.ah", config.Address)
	assert.Equal(t, "von-count", jwt)
	assert.Equal(t, "vault2-auth-sa", saName)
	assert.NoError(t, err)
}

// PredefinedVaultSelector is a dummy Selector that returns a pre-defined vault
type PredefinedVaultSelector struct {
	vaultToReturn v1alpha1.Vault
	url           string
}

func (p PredefinedVaultSelector) GetVault(name string, namespaces string) (*vault.Vault, error) {
	return &vault.Vault{
		Name:                   p.vaultToReturn.Name,
		Namespace:              p.vaultToReturn.Namespace,
		AuthServiceAccountName: p.vaultToReturn.Name + "-auth-sa",
		URL:                    p.url,
	}, nil
}
