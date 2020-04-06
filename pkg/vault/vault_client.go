package vault

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"

	"github.com/hashicorp/vault/api"
	"github.com/jenkins-x/jx/pkg/secreturl"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

const (
	yamlDataKey = "yaml"
)

var vaultURIRegex = regexp.MustCompile(`:[\s"]*vault:[-_.\w\/:]*`)

// Client is an interface for interacting with Vault
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/vault Client -o mocks/vault_client.go
type Client interface {
	// Write writes a named secret to the vault
	Write(secretName string, data map[string]interface{}) (map[string]interface{}, error)

	// WriteObject writes a generic named object to the vault.
	// The secret _must_ be serializable to JSON.
	WriteObject(secretName string, secret interface{}) (map[string]interface{}, error)

	// WriteYaml writes a yaml object to a named secret
	WriteYaml(secretName string, yamlstring string) (map[string]interface{}, error)

	// List lists the secrets under the specified path
	List(path string) ([]string, error)

	// Read reads a named secret from the vault
	Read(secretName string) (map[string]interface{}, error)

	// ReadObject reads a generic named object from vault.
	// The secret _must_ be serializable to JSON.
	ReadObject(secretName string, secret interface{}) error

	// ReadYaml reads a yaml object from a named secret
	ReadYaml(secretName string) (string, error)

	// Config gets the config required for configuring the official Vault CLI
	Config() (vaultURL url.URL, vaultToken string, err error)

	// ReplaceURIs will replace any vault: URIs in a string (or whatever URL scheme the secret URL client supports
	ReplaceURIs(text string) (string, error)
}

// client is a hand wrapper around the official Vault API
type client struct {
	client *api.Client
}

// NewVaultClient creates a new Vault Client wrapping the api.client
func NewVaultClient(apiclient *api.Client) Client {
	return &client{client: apiclient}
}

// Write writes a named secret to the vault with the data provided. Data can be a generic map of stuff, but at all points
// in the map, keys _must_ be strings (not bool, int or even interface{}) otherwise you'll get an error
func (v *client) Write(secretName string, data map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"data": data,
	}
	secret, err := v.client.Logical().Write(secretPath(secretName), payload)
	if secret != nil {
		return secret.Data, err
	}
	return nil, err
}

// Read reads a named secret to the vault
func (v *client) Read(secretName string) (map[string]interface{}, error) {
	secret, err := v.client.Logical().Read(secretPath(secretName))
	if err != nil {
		return nil, errors.Wrapf(err, "reading secret %q from vault", secretName)
	}

	if secret == nil {
		return nil, fmt.Errorf("no secret %q not found in vault", secretName)
	}

	if secret.Data != nil {
		data, ok := secret.Data["data"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid data type for secret %q", secretName)
		}
		return data, nil
	}
	return nil, fmt.Errorf("no data found on secret %q", secretName)
}

// WriteObject writes a generic named object to the vault. The secret _must_ be serializable to JSON
func (v *client) WriteObject(secretName string, secret interface{}) (map[string]interface{}, error) {
	// Convert the secret into a saveable map[string]interface{} format
	m, err := util.ToMapStringInterfaceFromStruct(&secret)
	if err != nil {
		return nil, errors.Wrapf(err, "serializing secret %q object for saving to vault", secretName)
	}
	return v.Write(secretName, m)
}

// ReadObject reads a generic named object from the vault.
func (v *client) ReadObject(secretName string, secret interface{}) error {
	m, err := v.Read(secretName)
	if err != nil {
		return errors.Wrapf(err, "reading the secret %q from vault", secretName)
	}
	err = util.ToStructFromMapStringInterface(m, &secret)
	if err != nil {
		return errors.Wrapf(err, "deserializing the secret %q from vault", secretName)
	}

	return nil
}

// WriteYaml writes a yaml object to a named secret
func (v *client) WriteYaml(secretName string, y string) (map[string]interface{}, error) {
	data := base64.StdEncoding.EncodeToString([]byte(y))
	secretMap := map[string]interface{}{
		yamlDataKey: data,
	}
	return v.Write(secretName, secretMap)
}

// ReadYaml reads a yaml object from a named secret
func (v *client) ReadYaml(secretName string) (string, error) {
	secretMap, err := v.Read(secretName)
	if err != nil {
		return "", errors.Wrapf(err, "reading secret %q from vault", secretName)
	}
	data, ok := secretMap[yamlDataKey]
	if !ok {
		return "", nil
	}
	strData, ok := data.(string)
	if !ok {
		return "", fmt.Errorf("data stored at secret key %s/%s is not a valid string", secretName, yamlDataKey)
	}
	decodedData, err := base64.StdEncoding.DecodeString(strData)
	if err != nil {
		return "", errors.Wrapf(err, "decoding base64 data stored at secret key %s/%s", secretName, yamlDataKey)
	}
	return string(decodedData), nil
}

// List lists the secrets under a given path
func (v *client) List(path string) ([]string, error) {
	secrets, err := v.client.Logical().List(secretMetadataPath(path))
	if err != nil {
		return nil, err
	}

	secretNames := make([]string, 0)
	if secrets == nil {
		return secretNames, nil
	}

	data := secrets.Data
	if data == nil {
		return secretNames, nil
	}

	// Don't do type assertion on nil
	keys := secrets.Data["keys"]
	if keys == nil {
		return secretNames, nil
	}

	for _, s := range keys.([]interface{}) {
		if orig, ok := s.(string); ok {
			secretNames = append(secretNames, orig)
		}
	}

	return secretNames, nil
}

// Config returns the current vault address and api token
func (v *client) Config() (vaultURL url.URL, vaultToken string, err error) {
	parsed, err := url.Parse(v.client.Address())
	return *parsed, v.client.Token(), err
}

// ReplaceURIs will replace any vault: URIs in a string (or whatever URL scheme the secret URL client supports
func (v *client) ReplaceURIs(s string) (string, error) {
	return secreturl.ReplaceURIs(s, v, vaultURIRegex, "vault:")
}
