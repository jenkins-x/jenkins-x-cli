package vault

import (
	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

const (
	usernameKey = "Username"
	passwordKey = "Password"
)

// WriteYAMLFiles stores the given YAML files in vault. The final secret path is
// a concatenation of the 'path' with the file name.
func WriteYamlFiles(client Client, path string, files ...string) error {
	secrets := map[string]string{}
	for _, file := range files {
		exists, err := util.FileExists(file)
		if exists && err == nil {
			empty, err := util.FileIsEmpty(file)
			if !empty && err == nil {
				content, err := ioutil.ReadFile(file)
				if err != nil {
					return errors.Wrapf(err, "reading secrets file '%s'", file)
				}
				key := filepath.Base(file)
				secrets[key] = string(content)
			}
		}
	}
	for secretName, secret := range secrets {
		secretPath := path + secretName
		_, err := client.WriteYaml(secretPath, secret)
		if err != nil {
			return errors.Wrapf(err, "storing the YAML file '%s' into vault", secretName)
		}
	}
	return nil
}

// WriteBasicAuth stores the basic authentication credentials in vault at the given path.
func WriteBasicAuth(client Client, path string, auth config.BasicAuth) error {
	return WriteMap(client, path, map[string]interface{}{
		usernameKey: auth.Username,
		passwordKey: auth.Password,
	})
}

// WriteMap stores the map in vault at the given path.
func WriteMap(client Client, path string, secret map[string]interface{}) error {
	_, err := client.Write(path, secret)
	if err != nil {
		return errors.Wrapf(err, "storing basic auth credentials into vault at path '%s'", path)
	}
	return nil
}
