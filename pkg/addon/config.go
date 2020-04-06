package addon

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/util"
	"sigs.k8s.io/yaml"
)

//AddonConfig Addon Configuration
type AddonConfig struct {
	Name    string
	Enabled bool
}

//AddonsConfig Addons Configuration
type AddonsConfig struct {
	Addons []*AddonConfig
}

// LoadAddonsConfig loads the addons configuration from the `~/.jx/addon.yml` file if it exists
func LoadAddonsConfig() (*AddonsConfig, error) {
	var config *AddonsConfig
	fileName, err := addonConfigFileName()
	if err != nil {
		return config, err
	}
	exists, err := util.FileExists(fileName)
	if err != nil {
		return config, err
	}
	config = &AddonsConfig{}
	if !exists {
		return config, nil
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return config, fmt.Errorf("Failed to load file %s due to %s", fileName, err)
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("Failed to unmarshal YAML file %s due to %s", fileName, err)
	}
	return config, nil
}

// IsAddonEnabled returns true if the given addon is enabled
func IsAddonEnabled(name string) bool {
	configs, err := LoadAddonsConfig()
	if err != nil {
		return false
	}
	return configs.GetOrCreate(name).Enabled
}

// Save saves the addons configuration to the `~/.jx/addon.yml` file
func (c *AddonsConfig) Save() error {
	fileName, err := addonConfigFileName()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
}

// GetOrCreate returns the addon configuration for the given name or creates a new config object
func (c *AddonsConfig) GetOrCreate(name string) *AddonConfig {
	for _, addon := range c.Addons {
		if addon.Name == name {
			return addon
		}
	}
	answer := &AddonConfig{
		Name: name,
	}
	c.Addons = append(c.Addons, answer)
	return answer
}

func addonConfigFileName() (string, error) {
	dir, err := util.ConfigDir()
	if err != nil {
		return "", err
	}
	fileName := filepath.Join(dir, "addons.yml")
	return fileName, nil

}
