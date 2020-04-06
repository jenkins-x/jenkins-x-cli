package secrets

import (
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// SecretsLocationKey key in the config map which stored the location where the secrets are stored
	SecretsLocationKey = "secretsLocation"
)

// SecretsLocationKind type for secrets location kind
type SecretsLocationKind string

const (
	// FileSystemLocationKind indicates that secrets location is the file system
	FileSystemLocationKind SecretsLocationKind = "local"
	// VaultLocationKind indicates that secrets location is vault
	VaultLocationKind SecretsLocationKind = "vault"
	// KubeLocationKind inidcates that secrets location is in Kuberntes
	KubeLocationKind SecretsLocationKind = "kube"
	// AutoLocationKind indicates that secrets location needs to be dynamically determine
	AutoLocationKind SecretsLocationKind = "auto"
)

// SecretLocation interfaces to identify where is the secrets location
type SecretLocation interface {
	// Location returns the location where the secrets are stored
	Location() SecretsLocationKind
	// SecretLocation configure the secrets location. It will save the
	// value in a config map if persist flag is set.
	SetLocation(location SecretsLocationKind, persist bool) error
}

type secretLocation struct {
	kubeClient kubernetes.Interface
	namespace  string
	location   SecretsLocationKind
}

// NewSecretLocation creates a SecretLocation
func NewSecretLocation(kubeClient kubernetes.Interface, namespace string) SecretLocation {
	return &secretLocation{
		kubeClient: kubeClient,
		namespace:  namespace,
		location:   FileSystemLocationKind,
	}
}

// Location returns the location of the secrets. It fetches the secrets location first
// for the config map, if not value is persisted there, it will just use the default location.
func (s *secretLocation) Location() SecretsLocationKind {
	configMap, err := getInstallConfigMapData(s.kubeClient, s.namespace)
	if err != nil {
		return s.location
	}
	value, ok := configMap[SecretsLocationKey]
	if ok && value == string(VaultLocationKind) {
		return VaultLocationKind
	}
	return s.location
}

// SetLocation configures the secrets location. It will store the value in a config map
// if the persist flag is set
func (s *secretLocation) SetLocation(location SecretsLocationKind, persist bool) error {
	s.location = location
	if persist {
		_, err := kube.DefaultModifyConfigMap(s.kubeClient, s.namespace, kube.ConfigMapNameJXInstallConfig,
			func(configMap *v1.ConfigMap) error {
				configMap.Data[SecretsLocationKey] = string(s.location)
				return nil
			}, nil)
		if err != nil {
			return errors.Wrapf(err, "saving secrets location in configmap %s", kube.ConfigMapNameJXInstallConfig)
		}
	}
	return nil
}

func getInstallConfigMapData(kubeClient kubernetes.Interface, namespace string) (map[string]string, error) {
	configMap, err := kube.GetConfigMapData(kubeClient, kube.ConfigMapNameJXInstallConfig, namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "getting configmap %s", kube.ConfigMapNameJXInstallConfig)
	}
	return configMap, nil
}

// ToSecretsLocation converts a string to a SecretsLocationKind
func ToSecretsLocation(location string) SecretsLocationKind {
	switch location {
	case "local":
		return FileSystemLocationKind
	case "vault":
		return VaultLocationKind
	case "kube":
		return KubeLocationKind
	default:
		return AutoLocationKind
	}
}
