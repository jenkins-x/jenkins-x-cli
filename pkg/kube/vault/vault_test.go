// +build unit

package vault_test

import (
	"testing"

	"github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/kube/vault"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateOrUpdateVault_with_no_preexisting_CRD_creates_vault(t *testing.T) {
	namespace := "test-ns"
	vaultName := "foo-vault"

	vaultCRD := &v1alpha1.Vault{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Vault",
			APIVersion: "vault.banzaicloud.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      vaultName,
			Namespace: namespace,
		},
		Spec: v1alpha1.VaultSpec{},
	}

	vaultOperatorClient := fake.NewSimpleClientset()

	_, err := vaultOperatorClient.VaultV1alpha1().Vaults(namespace).Get(vaultName, v1.GetOptions{})
	assert.Error(t, err)
	statusError := err.(*errors.StatusError)
	assert.Equal(t, int32(404), statusError.Status().Code)

	out := log.CaptureOutput(func() {
		err = vault.CreateOrUpdateVault(vaultCRD, vaultOperatorClient, namespace)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Vault 'foo-vault' in namespace 'test-ns' created")

	persistedCRD, err := vaultOperatorClient.VaultV1alpha1().Vaults(namespace).Get(vaultName, v1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, persistedCRD)
	assert.Equal(t, vaultCRD, persistedCRD)
}

func TestCreateOrUpdateVault_with_preexisting_CRD_updates_vault(t *testing.T) {
	namespace := "test-ns"
	vaultName := "foo-vault"

	vaultCRD := &v1alpha1.Vault{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Vault",
			APIVersion: "vault.banzaicloud.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      vaultName,
			Namespace: namespace,
		},
		Spec: v1alpha1.VaultSpec{},
	}

	vaultOperatorClient := fake.NewSimpleClientset()

	vaultCRD, err := vaultOperatorClient.VaultV1alpha1().Vaults(namespace).Create(vaultCRD)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), vaultCRD.Spec.Size)

	// use an update to Size for verification of the update
	vaultCRD.Spec.Size = 1
	out := log.CaptureOutput(func() {
		err = vault.CreateOrUpdateVault(vaultCRD, vaultOperatorClient, namespace)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Vault 'foo-vault' in namespace 'test-ns' updated")

	persistedCRD, err := vaultOperatorClient.VaultV1alpha1().Vaults(namespace).Get(vaultName, v1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, persistedCRD)
	assert.Equal(t, int32(1), vaultCRD.Spec.Size)
}
