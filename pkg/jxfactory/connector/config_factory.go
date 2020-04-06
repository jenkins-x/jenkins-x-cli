package connector

import (
	"github.com/heptio/sonobuoy/pkg/dynamic"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	// this is so that we load the auth plugins so we can connect to, say, GCP

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// ConfigClientFactory uses the given config to create clients
type ConfigClientFactory struct {
	name   string
	config *rest.Config
}

// NewConfigClientFactory creates a client factory for a given name and config
func NewConfigClientFactory(name string, config *rest.Config) *ConfigClientFactory {
	return &ConfigClientFactory{name, config}
}

// CreateApiExtensionsClient creates an API extensions client
func (f *ConfigClientFactory) CreateApiExtensionsClient() (apiextensionsclientset.Interface, error) {
	client, err := apiextensionsclientset.NewForConfig(f.config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create ApiExtensionsClient for remote cluster %s", f.name)
	}
	log.Logger().Infof("creating ApiExtensionsClient for cluster %s", f.name)
	return client, nil

}

// CreateKubeClient creates a new Kubernetes client
func (f *ConfigClientFactory) CreateKubeClient() (kubernetes.Interface, error) {
	client, err := kubernetes.NewForConfig(f.config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create KubeClient for remote cluster %s", f.name)
	}
	log.Logger().Infof("creating KubeClient for cluster %s", f.name)
	return client, nil
}

// CreateJXClient creates a new Kubernetes client for Jenkins X CRDs
func (f *ConfigClientFactory) CreateJXClient() (versioned.Interface, error) {
	client, err := versioned.NewForConfig(f.config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create JXClient for remote cluster %s", f.name)
	}
	log.Logger().Infof("creating JXClient for cluster %s", f.name)
	return client, nil
}

// CreateTektonClient create a new Kubernetes client for Tekton resources
func (f *ConfigClientFactory) CreateTektonClient() (tektonclient.Interface, error) {
	client, err := tektonclient.NewForConfig(f.config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create TektonClient for remote cluster %s", f.name)
	}
	log.Logger().Infof("creating TektonClient for cluster %s", f.name)
	return client, nil
}

// CreateDynamicClient create a new dynamic client
func (f *ConfigClientFactory) CreateDynamicClient() (*dynamic.APIHelper, error) {
	client, err := dynamic.NewAPIHelperFromRESTConfig(f.config)
	if err != nil {
		return nil, err
	}
	log.Logger().Infof("creating DynamicClient for cluster %s", f.name)
	return client, nil
}
