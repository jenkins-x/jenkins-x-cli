package config

import (
	"fmt"

	"github.com/ghodss/yaml"
)

type Image struct {
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

type Preview struct {
	Image *Image `json:"image,omitempty"`
}

type PreviewValuesConfig struct {
	ExposeController *ExposeController `json:"expose,omitempty"`
	Preview          *Preview          `json:"preview,omitempty"`
}

func (c PreviewValuesConfig) String() (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshall helm preview values %v", err)
	}
	return string(b), nil
}
