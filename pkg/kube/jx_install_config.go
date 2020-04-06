package kube

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// JXInstallConfig is the struct used to create the jx-install-config configmap
type JXInstallConfig struct {
	Server       string `structs:"server" yaml:"server" json:"server"`
	CA           []byte `structs:"ca.crt" yaml:"ca.crt" json:"ca.crt"`
	KubeProvider string `structs:"kubeProvider" yaml:"kubeProvider" json:"kubeProvider"`
}

// RememberRegion remembers cloud providers region in Kubernetes Config Map (jx-install-config), so jx can access this
// information later. Usually executed when provisioning new Kubernetes cluster.
//
// If jx-install-config config map doesn't exist, it will be created. If region value is already saved, it will be
// overridden by this function call.
func RememberRegion(kubeClient kubernetes.Interface, namespace string, region string) error {
	_, err := DefaultModifyConfigMap(kubeClient, namespace, ConfigMapNameJXInstallConfig, func(configMap *v1.ConfigMap) error {
		configMap.Data[Region] = region
		return nil
	}, nil)
	if err != nil {
		return errors.Wrapf(err, "saving cloud region in ConfigMap %s", ConfigMapNameJXInstallConfig)
	}
	return nil
}

// RememberInstallValues remembers any non-blank installation values in the Kubernetes Config Map (jx-install-config), so jx can access this
// information later. Usually executed when provisioning new Kubernetes cluster.
//
// If jx-install-config config map doesn't exist, it will be created. If region value is already saved, it will be
// overridden by this function call.
func RememberInstallValues(kubeClient kubernetes.Interface, namespace string, values map[string]string) error {
	_, err := DefaultModifyConfigMap(kubeClient, namespace, ConfigMapNameJXInstallConfig, func(configMap *v1.ConfigMap) error {
		if configMap.Data == nil {
			configMap.Data = values
			return nil
		}
		for k, v := range values {
			if v != "" {
				configMap.Data[k] = v
			}
		}
		return nil
	}, nil)
	if err != nil {
		return errors.Wrapf(err, "saving install values in in ConfigMap %s", ConfigMapNameJXInstallConfig)
	}
	return nil
}

// ReadInstallValues reads the installed configuration values from the installation namespace
//
// Empty map is returned if:
// - jx-install-config config map doesn't exist
// - kube client returns error on Config Map read operation
//
// Error is returned if:
// - jx-install-config config map doesn't exist
// - kube client returns error on Config Map read operation
func ReadInstallValues(kubeClient kubernetes.Interface, namespace string) (map[string]string, error) {
	data, err := GetConfigMapData(kubeClient, ConfigMapNameJXInstallConfig, namespace)
	if data == nil {
		data = map[string]string{}
	}
	if err != nil {
		return data, err
	}
	return data, nil
}

// ReadRegion allows to read cloud region from Config Map (jx-install-config). Region value is usually written using
// RememberRegion function.
//
// Empty string is returned if:
// - region value doesn't exist
// - has empty value
// - jx-install-config config map doesn't exist
// - kube client returns error on Config Map read operation
//
// Error is returned if:
// - jx-install-config config map doesn't exist
// - kube client returns error on Config Map read operation
func ReadRegion(kubeClient kubernetes.Interface, namespace string) (string, error) {
	data, err := ReadInstallValues(kubeClient, namespace)
	if err != nil {
		return "", err
	}
	return data[Region], nil
}
