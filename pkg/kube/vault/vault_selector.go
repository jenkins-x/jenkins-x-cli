package vault

import (
	"errors"
	"fmt"

	"github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/client-go/kubernetes"
)

// Selector is an interface for selecting a vault from the installed ones on the platform
// It should pick the most logical one, or give the user a way of picking a vault if there are multiple installed
type Selector interface {
	GetVault(name string, namespace string, useIngressURL bool) (*Vault, error)
}

type vaultSelector struct {
	vaultOperatorClient versioned.Interface
	kubeClient          kubernetes.Interface
	Handles             util.IOFileHandles
}

// NewVaultSelector creates a new vault selector
func NewVaultSelector(o OptionsInterface) (Selector, error) {
	operator, err := o.VaultOperatorClient()
	if err != nil {
		return nil, err
	}
	kubeclient, _, err := o.KubeClientAndNamespace()
	if err != nil {
		return nil, err
	}

	v := &vaultSelector{
		vaultOperatorClient: operator,
		kubeClient:          kubeclient,
		Handles:             o.GetIOFileHandles(),
	}
	return v, nil
}

// GetVault retrieve the given vault by name
func (v *vaultSelector) GetVault(name string, namespace string, useIngressURL bool) (*Vault, error) {
	vaults, err := GetVaults(v.kubeClient, v.vaultOperatorClient, namespace, useIngressURL)
	if err != nil {
		return nil, err
	}

	if name != "" {
		// Return the vault that the user wanted (or an error if it doesn't exist)
		for _, v := range vaults {
			if v.Name == name {
				return v, nil
			}
		}
		return nil, errors.New(fmt.Sprintf("vault '%s' not found in namespace '%s'", name, namespace))
	}

	if len(vaults) == 0 {
		return nil, errors.New(fmt.Sprintf("no vaults found in namespace '%s'", namespace))
	}
	if len(vaults) > 1 { // Get the user to select the vault from the list
		return v.selectVault(vaults)
	}
	// If there is only one vault, return that one
	return vaults[0], nil
}

func (v *vaultSelector) selectVault(vaults []*Vault) (*Vault, error) {
	vaultMap, vaultNames := make(map[string]*Vault, len(vaults)), make([]string, len(vaults))
	for i, vault := range vaults {
		vaultMap[vault.Name] = vault
		vaultNames[i] = vault.Name
	}

	vaultName, err := util.PickName(vaultNames, "Select Vault:", "", v.Handles)
	if err != nil {
		return nil, err
	}
	return vaultMap[vaultName], nil
}
