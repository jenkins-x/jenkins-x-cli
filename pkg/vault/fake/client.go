package fake

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"

	"github.com/jenkins-x/jx/pkg/secreturl"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

const (
	yamlDataKey = "yaml"
)

var vaultURIRegex = regexp.MustCompile(`vault:[-_.\w\/:]*`)

// FakeVaultClient is an in memory implementation of vault, useful for testing
type FakeVaultClient struct {
	Data map[string]map[string]interface{}
}

//NewFakeVaultClient creates a new FakeVaultClient
func NewFakeVaultClient() FakeVaultClient {
	return FakeVaultClient{
		Data: make(map[string]map[string]interface{}),
	}
}

// Write a secret to vault
func (f FakeVaultClient) Write(secretName string, data map[string]interface{}) (map[string]interface{}, error) {
	fmt.Printf("fakeClient: storing key at %s data: %#v\n", secretName, data)
	f.Data[secretName] = data
	return data, nil
}

// WriteObject a secret to vault
func (f FakeVaultClient) WriteObject(secretName string, secret interface{}) (map[string]interface{}, error) {
	payload, err := util.ToMapStringInterfaceFromStruct(secret)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return f.Write(secretName, payload)
}

// WriteYaml a secret to vault
func (f FakeVaultClient) WriteYaml(secretName string, y string) (map[string]interface{}, error) {
	data := base64.StdEncoding.EncodeToString([]byte(y))
	secretMap := map[string]interface{}{
		yamlDataKey: data,
	}
	return f.Write(secretName, secretMap)
}

// List the secrets in vault
func (f FakeVaultClient) List(path string) ([]string, error) {
	secretNames := make([]string, 0)
	for _, s := range f.Data[path] {
		if orig, ok := s.(string); ok {
			secretNames = append(secretNames, orig)
		}
	}
	return secretNames, nil
}

// Read a secret from vault
func (f FakeVaultClient) Read(secretName string) (map[string]interface{}, error) {
	if answer, ok := f.Data[secretName]; !ok {
		return nil, errors.Errorf("secret does not exist at key %s", secretName)
	} else {
		return answer, nil
	}
}

// ReadObject a secret from vault
func (f FakeVaultClient) ReadObject(secretName string, secret interface{}) error {
	m, err := f.Read(secretName)
	if err != nil {
		return errors.Wrapf(err, "reading the secret %q from vault", secretName)
	}
	err = util.ToStructFromMapStringInterface(m, &secret)
	if err != nil {
		return errors.Wrapf(err, "deserializing the secret %q from vault", secretName)
	}

	return nil
}

// ReadYaml a secret from vault
func (f FakeVaultClient) ReadYaml(secretName string) (string, error) {
	secretMap, err := f.Read(secretName)
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

// Config shows the vault config
func (f FakeVaultClient) Config() (vaultURL url.URL, vaultToken string, err error) {
	u, err := url.Parse("https://fake.vault")
	if err != nil {
		return *u, "", errors.WithStack(err)
	}
	return *u, "fakevault", nil
}

// ReplaceURIs corrects the URIs
func (f FakeVaultClient) ReplaceURIs(text string) (string, error) {
	return secreturl.ReplaceURIs(text, f, vaultURIRegex, "vault:")
}
