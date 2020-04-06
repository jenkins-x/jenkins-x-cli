package jxfactory

import (
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/kube"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	// this is so that we load the auth plugins so we can connect to, say, GCP
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Factory is the interface defined for Kubernetes, Jenkins X, and Tekton REST APIs
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/jxfactory Factory -o mocks/factory.go
type Factory interface {
	// WithBearerToken creates a factory from a k8s bearer token
	WithBearerToken(token string) Factory

	// ImpersonateUser creates a factory with an impersonated users
	ImpersonateUser(user string) Factory

	// CreateKubeClient creates a new Kubernetes client
	CreateKubeClient() (kubernetes.Interface, string, error)

	// CreateKubeConfig creates the kubernetes configuration
	CreateKubeConfig() (*rest.Config, error)

	// CreateJXClient creates a new Kubernetes client for Jenkins X CRDs
	CreateJXClient() (versioned.Interface, string, error)

	// CreateTektonClient create a new Kubernetes client for Tekton resources
	CreateTektonClient() (tektonclient.Interface, string, error)

	// KubeConfig returns a Kuber instance to interact with the kube configuration.
	KubeConfig() kube.Kuber
}
