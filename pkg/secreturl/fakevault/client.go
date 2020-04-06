package fakevault

import (
	"regexp"

	"github.com/jenkins-x/jx/pkg/secreturl"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

var fakeURIRegex = regexp.MustCompile(`:[\s"]*vault:[-_.\w\/:]*`)

// FakeClient a local file system based client loading/saving content from the given URL
type FakeClient struct {
	data map[string]map[string]interface{}
}

// NewFakeClient create a new fake client
func NewFakeClient() secreturl.Client {
	return &FakeClient{
		data: map[string]map[string]interface{}{},
	}
}

// Read reads a named secret from the vault
func (c *FakeClient) Read(secretName string) (map[string]interface{}, error) {
	return c.data[secretName], nil
}

// ReadObject reads a generic named object from vault.
// The secret _must_ be serializable to JSON.
func (c *FakeClient) ReadObject(secretName string, secret interface{}) error {
	m, err := c.Read(secretName)
	if err != nil {
		return errors.Wrapf(err, "reading the secret %q from vault", secretName)
	}
	err = util.ToStructFromMapStringInterface(m, &secret)
	if err != nil {
		return errors.Wrapf(err, "deserializing the secret %q from vault", secretName)
	}
	return nil
}

// Write writes a named secret to the vault with the data provided. Data can be a generic map of stuff, but at all points
// in the map, keys _must_ be strings (not bool, int or even interface{}) otherwise you'll get an error
func (c *FakeClient) Write(secretName string, data map[string]interface{}) (map[string]interface{}, error) {
	c.data[secretName] = data
	return c.Read(secretName)
}

// WriteObject writes a generic named object to the vault.
// The secret _must_ be serializable to JSON.
func (c *FakeClient) WriteObject(secretName string, secret interface{}) (map[string]interface{}, error) {
	// TODO
	return c.Read(secretName)
}

// ReplaceURIs will replace any local: URIs in a string
func (c *FakeClient) ReplaceURIs(s string) (string, error) {
	return secreturl.ReplaceURIs(s, c, fakeURIRegex, "vault:")
}
