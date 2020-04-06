package auth

import (
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/vault"
	"github.com/pkg/errors"

	"sigs.k8s.io/yaml"
)

// LoadConfig loads the config from the vault
func (v *VaultAuthConfigHandler) LoadConfig() (*AuthConfig, error) {
	data, err := v.vaultClient.ReadYaml(vault.AuthSecretPath(v.secretName))
	if err != nil {
		return nil, errors.Wrapf(err, "loading the auth config %q from vault", v.secretName)
	}

	var config AuthConfig
	if data == "" {
		return &config, nil
	}

	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return nil, errors.Wrapf(err, "unmarshalling auth config %q", v.secretName)
	}
	return &config, nil
}

// SaveConfig saves the config to the vault
func (v *VaultAuthConfigHandler) SaveConfig(config *AuthConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "marshaling auth config")
	}
	if _, err := v.vaultClient.WriteYaml(vault.AuthSecretPath(v.secretName), string(data)); err != nil {
		return errors.Wrapf(err, "saving auth config %q in vault", v.secretName)
	}
	return nil
}

// NewVaultAuthConfigService creates a new config service  that loads/saves the auth config form/into vault
func NewVaultAuthConfigService(secretName string, vaultClient vault.Client) ConfigService {
	// Only use the base file name as a vault key in case there is a full file path
	secretName = filepath.Base(secretName)
	handler := newVaultAuthConfigHandler(secretName, vaultClient)
	return NewAuthConfigService(&handler)
}

// newVaultAuthConfigHandler creates a config handler  that loads/saves the auth config  under a specified secret name in a vault
func newVaultAuthConfigHandler(secretName string, vaultClient vault.Client) VaultAuthConfigHandler {
	return VaultAuthConfigHandler{
		secretName:  secretName,
		vaultClient: vaultClient,
	}
}
