package config

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"io/ioutil"
	"path/filepath"
	"reflect"

	"github.com/jenkins-x/jx/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	// ProjectConfigFileName is the name of the project configuration file
	ProjectConfigFileName = "jenkins-x.yml"
)

// +exported

// ProjectConfig defines Jenkins X Pipelines usually stored inside the `jenkins-x.yml` file in projects
type ProjectConfig struct {
	// List of global environment variables to add to each branch build and each step
	Env []corev1.EnvVar `json:"env,omitempty"`

	PreviewEnvironments *PreviewEnvironmentConfig   `json:"previewEnvironments,omitempty"`
	IssueTracker        *IssueTrackerConfig         `json:"issueTracker,omitempty"`
	Chat                *ChatConfig                 `json:"chat,omitempty"`
	Wiki                *WikiConfig                 `json:"wiki,omitempty"`
	Addons              []*AddonConfig              `json:"addons,omitempty"`
	BuildPack           string                      `json:"buildPack,omitempty"`
	BuildPackGitURL     string                      `json:"buildPackGitURL,omitempty"`
	BuildPackGitURef    string                      `json:"buildPackGitRef,omitempty"`
	PipelineConfig      *jenkinsfile.PipelineConfig `json:"pipelineConfig,omitempty"`
	NoReleasePrepare    bool                        `json:"noReleasePrepare,omitempty"`
	DockerRegistryHost  string                      `json:"dockerRegistryHost,omitempty"`
	DockerRegistryOwner string                      `json:"dockerRegistryOwner,omitempty"`
}

type PreviewEnvironmentConfig struct {
	Disabled         bool `json:"disabled,omitempty"`
	MaximumInstances int  `json:"maximumInstances,omitempty"`
}

type IssueTrackerConfig struct {
	Kind    string `json:"kind,omitempty"`
	URL     string `json:"url,omitempty"`
	Project string `json:"project,omitempty"`
}

type WikiConfig struct {
	Kind  string `json:"kind,omitempty"`
	URL   string `json:"url,omitempty"`
	Space string `json:"space,omitempty"`
}

type ChatConfig struct {
	Kind             string `json:"kind,omitempty"`
	URL              string `json:"url,omitempty"`
	DeveloperChannel string `json:"developerChannel,omitempty"`
	UserChannel      string `json:"userChannel,omitempty"`
}

type AddonConfig struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// LoadProjectConfig loads the project configuration if there is a project configuration file
func LoadProjectConfig(projectDir string) (*ProjectConfig, string, error) {
	fileName := ProjectConfigFileName
	if projectDir != "" {
		fileName = filepath.Join(projectDir, fileName)
	}
	config, err := LoadProjectConfigFile(fileName)
	return config, fileName, err
}

// LoadProjectConfigFile loads a specific project YAML configuration file
func LoadProjectConfigFile(fileName string) (*ProjectConfig, error) {
	config := ProjectConfig{}
	exists, err := util.FileExists(fileName)
	if err != nil || !exists {
		return &config, err
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return &config, fmt.Errorf("Failed to load file %s due to %s", fileName, err)
	}
	validationErrors, err := util.ValidateYaml(&config, data)
	if err != nil {
		return &config, fmt.Errorf("failed to validate YAML file %s due to %s", fileName, err)
	}
	if len(validationErrors) > 0 {
		return &config, fmt.Errorf("Validation failures in YAML file %s:\n%s", fileName, strings.Join(validationErrors, "\n"))
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return &config, fmt.Errorf("Failed to unmarshal YAML file %s due to %s", fileName, err)
	}
	return &config, nil
}

// IsEmpty returns true if this configuration is empty
func (c *ProjectConfig) IsEmpty() bool {
	empty := &ProjectConfig{}
	return reflect.DeepEqual(empty, c)
}

// SaveConfig saves the configuration file to the given project directory
func (c *ProjectConfig) SaveConfig(fileName string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", fileName)
	}
	return nil
}

// GetOrCreatePipelineConfig lazily creates a PipelineConfig if required
func (c *ProjectConfig) GetOrCreatePipelineConfig() *jenkinsfile.PipelineConfig {
	if c.PipelineConfig == nil {
		c.PipelineConfig = &jenkinsfile.PipelineConfig{}
	}
	return c.PipelineConfig
}

// GetPipeline retrieves the parsed pipeline for the specified type
func (c *ProjectConfig) GetPipeline(kind string) (*syntax.ParsedPipeline, error) {
	var parsed *syntax.ParsedPipeline

	if c.PipelineConfig == nil {
		return nil, nil
	}

	switch kind {
	case jenkinsfile.PipelineKindRelease:
		if c.PipelineConfig.Pipelines.Release == nil {
			parsed = nil
		} else {
			parsed = c.PipelineConfig.Pipelines.Release.Pipeline
		}
	case jenkinsfile.PipelineKindPullRequest:
		if c.PipelineConfig.Pipelines.PullRequest == nil {
			parsed = nil
		} else {
			parsed = c.PipelineConfig.Pipelines.PullRequest.Pipeline
		}
	case jenkinsfile.PipelineKindFeature:
		if c.PipelineConfig.Pipelines.Feature == nil {
			parsed = nil
		} else {
			parsed = c.PipelineConfig.Pipelines.Feature.Pipeline
		}
	default:
		return nil, errors.Errorf("unknown pipeline kind %s", kind)
	}

	if parsed == nil {
		return nil, errors.Errorf("no pipeline defined for kind %s", kind)
	}
	return parsed, nil
}
