// +build unit

package vault_test

import (
	"fmt"
	"testing"

	vault_const "github.com/jenkins-x/jx/pkg/vault"

	expect "github.com/Netflix/go-expect"
	kubevault "github.com/jenkins-x/jx/pkg/kube/vault"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
)

func Test_GetVault_DoesNotPromptUserIfOnlyOneVaultInNamespace(t *testing.T) {
	vaultOperatorClient, factory, kubeClient, err := setupMocks(t, nil)
	createMockedVault("myVault", "myVaultNamespace", "foo.bar", "myJWT", vaultOperatorClient, kubeClient)

	selector, err := kubevault.NewVaultSelector(factory.Options)

	vault, err := selector.GetVault("", "myVaultNamespace", true)

	assert.Equal(t, "myVault", vault.Name)
	assert.Equal(t, "myVaultNamespace", vault.Namespace)
	assert.Equal(t, "http://foo.bar", vault.URL)
	assert.Equal(t, "myVault-auth-sa", vault.AuthServiceAccountName)
	assert.NoError(t, err)
}

func Test_GetVault_InclusterUsesInternalVaultURL(t *testing.T) {
	vaultOperatorClient, factory, kubeClient, err := setupMocks(t, nil)
	createMockedVault("myVault", "myVaultNamespace", "foo.bar", "myJWT", vaultOperatorClient, kubeClient)

	selector, err := kubevault.NewVaultSelector(factory.Options)

	vault, err := selector.GetVault("", "myVaultNamespace", false)

	assert.Equal(t, "myVault", vault.Name)
	assert.Equal(t, "myVaultNamespace", vault.Namespace)
	assert.Equal(t, fmt.Sprintf("http://myVault:%s", vault_const.DefaultVaultPort), vault.URL)
	assert.Equal(t, "myVault-auth-sa", vault.AuthServiceAccountName)
	assert.NoError(t, err)
}

func Test_GetVault_ErrorsIfNoVaultsInNamespace(t *testing.T) {
	vaultOperatorClient, factory, kubeClient, err := setupMocks(t, nil)
	createMockedVault("myVault", "myVaultNamespace", "foo.bar", "myJWT", vaultOperatorClient, kubeClient)

	selector, err := kubevault.NewVaultSelector(factory.Options)

	vault, err := selector.GetVault("", "Nothing Here Jim", true)

	assert.Nil(t, vault)
	assert.EqualError(t, err, "no vaults found in namespace 'Nothing Here Jim'")
}

func Test_GetVault_ErrorsIfRequestedVaultDoesNotExist(t *testing.T) {
	vaultOperatorClient, factory, kubeClient, err := setupMocks(t, nil)
	createMockedVault("myVault", "myVaultNamespace", "foo.bar", "myJWT", vaultOperatorClient, kubeClient)

	selector, err := kubevault.NewVaultSelector(factory.Options)

	vault, err := selector.GetVault("NoVaultHere", "myVaultNamespace", true)

	assert.Nil(t, vault)
	assert.EqualError(t, err, "vault 'NoVaultHere' not found in namespace 'myVaultNamespace'")
}

func Test_GetVault_GetExplicitVaultSucceedsWhenTwoVaultsAreDefined(t *testing.T) {
	vaultOperatorClient, factory, kubeClient, err := setupMocks(t, nil)
	createMockedVault("vault1", "myVaultNamespace", "one.ah.ah.ah", "Count", vaultOperatorClient, kubeClient)
	createMockedVault("vault2", "myVaultNamespace", "two.ah.ah.ah", "Von-Count", vaultOperatorClient, kubeClient)

	selector, err := kubevault.NewVaultSelector(factory.Options)

	vault, err := selector.GetVault("vault2", "myVaultNamespace", true)

	assert.Equal(t, "vault2", vault.Name)
	assert.Equal(t, "myVaultNamespace", vault.Namespace)
	assert.Equal(t, "http://two.ah.ah.ah", vault.URL)
	assert.Equal(t, "vault2-auth-sa", vault.AuthServiceAccountName)
	assert.NoError(t, err)
}

func Test_GetVault_PromptsUserIfMoreThanOneVaultInNamespace(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")

	// mock terminal
	console := tests.NewTerminal(t, nil)
	defer console.Cleanup()
	vaultOperatorClient, factory, kubeClient, err := setupMocks(t, &console.Stdio)
	createMockedVault("vault1", "myVaultNamespace", "one.ah.ah.ah", "Count", vaultOperatorClient, kubeClient)
	createMockedVault("vault2", "myVaultNamespace", "two.ah.ah.ah", "Von-Count", vaultOperatorClient, kubeClient)

	selector, err := kubevault.NewVaultSelector(factory.Options)

	//Test interactive IO
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		console.ExpectString("Select Vault:")
		console.SendLine("vault2")
		console.ExpectEOF()
	}()

	vault, err := selector.GetVault("", "myVaultNamespace", true)

	console.Close()
	<-donec

	// Dump the terminal's screen.
	t.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

	assert.Equal(t, "vault2", vault.Name)
	assert.Equal(t, "myVaultNamespace", vault.Namespace)
	assert.Equal(t, "http://two.ah.ah.ah", vault.URL)
	assert.Equal(t, "vault2-auth-sa", vault.AuthServiceAccountName)
	assert.NoError(t, err)
}
