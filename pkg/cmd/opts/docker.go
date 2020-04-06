package opts

import (
	"os"
	"strings"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
)

// GetDockerRegistryOrg parses the docker registry organisation from various places
func (o *CommonOptions) GetDockerRegistryOrg(projectConfig *config.ProjectConfig, repository *gits.GitRepository) string {
	answer := ""
	if projectConfig != nil {
		answer = projectConfig.DockerRegistryOwner
	}
	if answer == "" {
		teamSettings, err := o.TeamSettings()
		if err != nil {
			log.Logger().Warnf("Could not load team settings %s", err.Error())
		} else {
			answer = teamSettings.DockerRegistryOrg
		}
		if answer == "" {
			answer = os.Getenv("DOCKER_REGISTRY_ORG")
		}
		if answer == "" && repository != nil {
			answer = repository.Organisation
		}
	}
	return strings.ToLower(answer)
}

// GetDockerRegistry parses the docker registry from various places
func (o *CommonOptions) GetDockerRegistry(projectConfig *config.ProjectConfig) string {
	dockerRegistry := ""
	if projectConfig != nil {
		dockerRegistry = projectConfig.DockerRegistryHost
	}
	if dockerRegistry == "" {
		dockerRegistry = os.Getenv("DOCKER_REGISTRY")
	}
	if dockerRegistry == "" {
		kubeClient, ns, err := o.KubeClientAndDevNamespace()
		if err != nil {
			log.Logger().Warnf("failed to create kube client: %s", err.Error())
		} else {
			name := kube.ConfigMapJenkinsDockerRegistry
			data, err := kube.GetConfigMapData(kubeClient, name, ns)
			if err != nil {
				log.Logger().Warnf("failed to load ConfigMap %s in namespace %s: %s", name, ns, err.Error())
			} else {
				dockerRegistry = data["docker.registry"]
			}
		}
	}
	return dockerRegistry
}
