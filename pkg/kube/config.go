package kube

import (
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	// DefaultNamespace the standard namespace for Jenkins X
	DefaultNamespace = "jx"

	// PodNamespaceFile the file path and name for pod namespace
	PodNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// KubeConfig implements kube interactions
type KubeConfig struct{}

// NewKubeConfig creates a new KubeConfig struct to be used to interact with the underlying kube system
func NewKubeConfig() Kuber {
	return &KubeConfig{}
}

// CurrentContextName returns the current context name
func CurrentContextName(config *api.Config) string {
	if config != nil {
		return config.CurrentContext
	}
	return ""
}

// CurrentContext returns the current context
func CurrentContext(config *api.Config) *api.Context {
	if config != nil {
		name := config.CurrentContext
		if name != "" && config.Contexts != nil {
			return config.Contexts[name]
		}
	}
	return nil
}

// CurrentCluster returns the current cluster
func CurrentCluster(config *api.Config) (string, *api.Cluster) {
	if config != nil {
		context := CurrentContext(config)
		if context != nil && config.Clusters != nil {
			return context.Cluster, config.Clusters[context.Cluster]
		}
	}
	return "", nil
}

// Cluster returns the cluster of the given config
func Cluster(config *api.Config) string {
	if config != nil {
		context := CurrentContext(config)
		if context != nil && config.Clusters != nil {
			return context.Cluster
		}
	}
	return ""
}

// CurrentServer returns the current context's server
func CurrentServer(config *api.Config) string {
	context := CurrentContext(config)
	return Server(config, context)
}

// Server returns the server of the given context
func Server(config *api.Config, context *api.Context) string {
	if context != nil && config != nil && config.Clusters != nil {
		cluster := config.Clusters[context.Cluster]
		if cluster != nil {
			return cluster.Server
		}
	}
	return ""
}

// CertificateAuthorityData returns the certificate authority data for the given context
func CertificateAuthorityData(config *api.Config, context *api.Context) []byte {
	if context != nil && config != nil && config.Clusters != nil {
		cluster := config.Clusters[context.Cluster]
		if cluster != nil {
			return cluster.CertificateAuthorityData
		}
	}
	return []byte{}
}

// UpdateConfig defines new config entries for jx
func (k *KubeConfig) UpdateConfig(namespace string, server string, caData string, user string, token string) error {
	config, po, err := k.LoadConfig()
	if err != nil {
		return errors.Wrap(err, "loading existing config")
	}

	clusterName := "jx-cluster"
	cluster := &api.Cluster{
		Server:                   server,
		CertificateAuthorityData: []byte(caData),
	}

	authInfo := &api.AuthInfo{
		Token: token,
	}

	ctxName := fmt.Sprintf("jx-cluster-%s-ctx", user)
	ctx := &api.Context{
		Cluster:   clusterName,
		AuthInfo:  user,
		Namespace: namespace,
	}

	config.Clusters[clusterName] = cluster
	config.AuthInfos[user] = authInfo
	config.Contexts[ctxName] = ctx
	config.CurrentContext = ctxName

	return clientcmd.ModifyConfig(po, *config, false)
}

// AddUserToConfig adds the given user to the config
func AddUserToConfig(user string, token string, config *api.Config) (*api.Config, error) {
	currentClusterName, currentCluster := CurrentCluster(config)
	if currentCluster == nil || currentClusterName == "" {
		return config, errors.New("no cluster found in config")
	}
	currentCtx := CurrentContext(config)
	currentNamespace := DefaultNamespace
	if currentCtx != nil {
		currentNamespace = currentCtx.Namespace
	}

	ctx := &api.Context{
		Cluster:   currentClusterName,
		AuthInfo:  user,
		Namespace: currentNamespace,
	}

	authInfo := &api.AuthInfo{
		Token: token,
	}

	config.AuthInfos[user] = authInfo
	ctxName := fmt.Sprintf("jx-%s-%s-ctx", currentClusterName, user)
	config.Contexts[ctxName] = ctx
	config.CurrentContext = ctxName

	return config, nil
}

// LoadConfig loads the Kubernetes configuration
func (k *KubeConfig) LoadConfig() (*api.Config, *clientcmd.PathOptions, error) {
	po := clientcmd.NewDefaultPathOptions()
	if po == nil {
		return nil, po, fmt.Errorf("Could not find any default path options for the kubeconfig file usually found at ~/.kube/config")
	}
	config, err := po.GetStartingConfig()
	if err != nil {
		return nil, po, fmt.Errorf("Could not load the kube config file %s due to %s", po.GetDefaultFilename(), err)
	}
	return config, po, err
}

// CurrentNamespace returns the current namespace in the context
func CurrentNamespace(config *api.Config) string {
	ctx := CurrentContext(config)
	if ctx != nil {
		n := ctx.Namespace
		if n != "" {
			return n
		}
	}
	// if we are in a pod lets try load the pod namespace file
	data, err := ioutil.ReadFile(PodNamespaceFile)
	if err == nil {
		n := string(data)
		if n != "" {
			return n
		}
	}
	return "default"
}
