package auth

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// ServerKind indicates the server kind used to load the auth config from file
type ServerKind string

// GitServerKind indicate the server kind for git
const GitServerKind ServerKind = "git"

// NewFileAuthConfigService creates a new file config service
func NewFileAuthConfigService(filename string, serverKind string) (ConfigService, error) {
	handler, err := newFileAuthConfigHandler(filename, serverKind)
	return NewAuthConfigService(handler), err
}

// newFileAuthConfigHandler creates a new FileBasedAuthConfigService that stores its data under the given filename
// If the fileName is an absolute path, it will be used. If it is a simple filename, it will be stored in the default
// Config directory
func newFileAuthConfigHandler(fileName string, serverKind string) (ConfigHandler, error) {
	svc := &FileAuthConfigHandler{
		serverKind: serverKind,
	}
	// If the fileName is an absolute path, use that. Otherwise treat it as a config filename to be used in
	if fileName == filepath.Base(fileName) {
		dir, err := util.ConfigDir()
		if err != nil {
			return svc, err
		}
		svc.fileName = filepath.Join(dir, fileName)
	} else {
		svc.fileName = fileName
	}
	return svc, nil
}

// loadFileAuth loads the auth config from given file
func (s *FileAuthConfigHandler) loadFileAuth(fileName string) (*AuthConfig, error) {
	if fileName == "" {
		return nil, fmt.Errorf("empty file name for auth config")
	}
	exists, err := util.FileExists(fileName)
	if err != nil {
		return nil, fmt.Errorf("checking if the auth config file exists %s due to %s", fileName, err)
	}
	if !exists {
		return nil, fmt.Errorf("auth config file %q does not exist", fileName)
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "loading the auth config from file %q", fileName)
	}
	config := &AuthConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling the auth config YAML from file %q", fileName)
	}
	return config, nil
}

// LoadConfig loads the configuration from the users JX config directory
func (s *FileAuthConfigHandler) LoadConfig() (*AuthConfig, error) {
	config, err := s.loadFileAuth(s.fileName)
	if err != nil {
		// Try to load the auth config from git credentials file
		if s.serverKind == string(GitServerKind) {
			gitConfig, err := loadGitCredentialsAuth()
			if err != nil {
				return nil, errors.Wrap(err, "loading the auth config from git credentials file")
			}
			return gitConfig, nil
		}
		return nil, errors.Wrapf(err, "loading the auth config from file %q", s.fileName)
	}
	return config, nil
}

// SaveConfig saves the configuration to disk
func (s *FileAuthConfigHandler) SaveConfig(config *AuthConfig) error {
	fileName := s.fileName
	if fileName == "" {
		return fmt.Errorf("no filename defined")
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
}
