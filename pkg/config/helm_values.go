package config

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

// ExposeDefaultURLTemplate is the default url template format needed by exposecontroller to create ingress rules that work with wiuldcard certs
const ExposeDefaultURLTemplate = "{{.Service}}-{{.Namespace}}.{{.Domain}}"

type ExposeControllerConfig struct {
	Domain        string `json:"domain,omitempty"`
	Exposer       string `json:"exposer,omitempty"`
	HTTP          string `json:"http,omitempty"`
	TLSAcme       string `json:"tlsacme,omitempty"`
	PathMode      string `json:"pathMode,omitempty"`
	URLTemplate   string `json:"urltemplate,omitempty"`
	IngressClass  string `json:"ingressClass,omitempty"`
	TLSSecretName string `json:"tlsSecretName,omitempty"`
}
type ExposeController struct {
	Config      ExposeControllerConfig `json:"config,omitempty"`
	Annotations map[string]string      `json:"Annotations,omitempty"`
	Production  bool                   `json:"production,omitempty"`
}

type JenkinsValuesConfig struct {
	Servers JenkinsServersValuesConfig `json:"Servers,omitempty"`
	Enabled *bool                      `json:"enabled,omitempty"`
}

type ProwValuesConfig struct {
	User       string `json:"user,omitempty"`
	HMACtoken  string `json:"hmacToken,omitempty"`
	OAUTHtoken string `json:"oauthToken,omitempty"`
}

type JenkinsServersValuesConfig struct {
	Gitea  []JenkinsGiteaServersValuesConfig  `json:"Gitea,omitempty"`
	GHE    []JenkinsGithubServersValuesConfig `json:"GHE,omitempty"`
	Global JenkinsServersGlobalConfig         `json:"Global,omitempty"`
}

type JenkinsServersGlobalConfig struct {
	EnvVars map[string]string `json:"EnvVars,omitempty"`
}

type JenkinsGiteaServersValuesConfig struct {
	Name       string `json:"Name,omitempty"`
	Url        string `json:"Url,omitempty"`
	Credential string `json:"Credential,omitempty"`
}

type JenkinsGithubServersValuesConfig struct {
	Name string `json:"Name,omitempty"`
	Url  string `json:"Url,omitempty"`
}

type JenkinsPipelineSecretsValuesConfig struct {
	DockerConfig string `json:"DockerConfig,flow,omitempty"`
}

// EnabledConfig to configure the feature on/off
type EnabledConfig struct {
	Enabled bool `json:"enabled"`
}

type HelmValuesConfig struct {
	ExposeController      *ExposeController                  `json:"expose,omitempty"`
	Jenkins               JenkinsValuesConfig                `json:"jenkins,omitempty"`
	Prow                  ProwValuesConfig                   `json:"prow,omitempty"`
	PipelineSecrets       JenkinsPipelineSecretsValuesConfig `json:"PipelineSecrets,omitempty"`
	ControllerBuild       *EnabledConfig                     `json:"controllerbuild,omitempty"`
	ControllerWorkflow    *EnabledConfig                     `json:"controllerworkflow,omitempty"`
	DockerRegistryEnabled *EnabledConfig                     `json:"docker-registry,omitempty"`
	DockerRegistry        string                             `json:"dockerRegistry,omitempty"`
}

type HelmValuesConfigService struct {
	FileName string
	Config   HelmValuesConfig
}

// GetOrCreateFirstGitea returns the first gitea server creating one if required
func (c *JenkinsServersValuesConfig) GetOrCreateFirstGitea() *JenkinsGiteaServersValuesConfig {
	if len(c.Gitea) == 0 {
		c.Gitea = []JenkinsGiteaServersValuesConfig{
			{
				Name:       "gitea",
				Credential: "jenkins-x-git",
			},
		}
	}
	return &c.Gitea[0]
}

func (c *HelmValuesConfig) AddExposeControllerValues(cmd *cobra.Command, ignoreDomain bool) {
	if !ignoreDomain {
		cmd.Flags().StringVarP(&c.ExposeController.Config.Domain, "domain", "", "", "Domain to expose ingress endpoints.  Example: jenkinsx.io")
	}
	keepJob := false

	cmd.Flags().StringVarP(&c.ExposeController.Config.HTTP, "http", "", "true", "Toggle creating http or https ingress rules")
	cmd.Flags().StringVarP(&c.ExposeController.Config.Exposer, "exposer", "", "Ingress", "Used to describe which strategy exposecontroller should use to access applications")
	cmd.Flags().StringVarP(&c.ExposeController.Config.TLSAcme, "tls-acme", "", "", "Used to enable automatic TLS for ingress")
	cmd.Flags().StringVarP(&c.ExposeController.Config.URLTemplate, "urltemplate", "", "", "For ingress; exposers can set the urltemplate to expose")
	cmd.Flags().StringVarP(&c.ExposeController.Config.IngressClass, "ingress-class", "", "", "Used to set the ingress.class annotation in exposecontroller created ingress")
	cmd.Flags().BoolVarP(&keepJob, "keep-exposecontroller-job", "", false, "Prevents Helm deleting the exposecontroller Job and Pod after running.  Useful for debugging exposecontroller logs but you will need to manually delete the job if you update an environment")

	cmd.Flags().MarkDeprecated("http", "please use `jx upgrade ingress` after install instead")
	cmd.Flags().MarkDeprecated("tls-acme", "please use `jx upgrade ingress` after install instead")

	annotations := make(map[string]string)
	annotations["helm.sh/hook"] = "post-install,post-upgrade"
	if !keepJob {
		annotations["helm.sh/hook-delete-policy"] = "hook-succeeded"
	}
	c.ExposeController.Annotations = annotations
}

func (c HelmValuesConfig) String() (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshall helm values %v", err)
	}
	return string(b), nil
}
