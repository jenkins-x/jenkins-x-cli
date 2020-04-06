package jxfactory

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/util/trace"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	// this is so that we load the auth plugins so we can connect to, say, GCP
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type factory struct {
	kubeConfig      kube.Kuber
	impersonateUser string
	bearerToken     string
	kubeConfigCache *string
}

var _ Factory = (*factory)(nil)

// NewFactory creates a factory with the default Kubernetes resources defined
// if optionalClientConfig is nil, then flags will be bound to a new clientcmd.ClientConfig.
// if optionalClientConfig is not nil, then this factory will make use of it.
func NewFactory() Factory {
	f := &factory{}
	f.kubeConfig = kube.NewKubeConfig()
	return f
}

// ImpersonateUser returns a new factory impersonating the given user
func (f *factory) ImpersonateUser(user string) Factory {
	copy := *f
	copy.impersonateUser = user
	return &copy
}

// WithBearerToken returns a new factory with bearer token
func (f *factory) WithBearerToken(token string) Factory {
	copy := *f
	copy.bearerToken = token
	return &copy
}

// KubeConfig returns a Kuber instance to interact with the kube configuration.
func (f *factory) KubeConfig() kube.Kuber {
	return f.kubeConfig
}

func (f *factory) CreateKubeClient() (kubernetes.Interface, string, error) {
	cfg, err := f.CreateKubeConfig()
	if err != nil {
		return nil, "", err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, "", err
	}
	if client == nil {
		return nil, "", fmt.Errorf("failed to create Kubernetes Client")
	}
	ns := ""
	config, _, err := f.kubeConfig.LoadConfig()
	if err != nil {
		return client, ns, err
	}
	ns = kube.CurrentNamespace(config)
	// TODO allow namsepace to be specified as a CLI argument!
	return client, ns, nil
}

func (f *factory) CreateKubeConfig() (*rest.Config, error) {
	masterURL := ""
	kubeConfigEnv := os.Getenv("KUBECONFIG")
	if kubeConfigEnv != "" {
		pathList := filepath.SplitList(kubeConfigEnv)
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{Precedence: pathList},
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: masterURL}}).ClientConfig()
	}
	kubeconfig := f.createKubeConfigText()
	var config *rest.Config
	var err error
	if kubeconfig != nil {
		exists, err := util.FileExists(*kubeconfig)
		if err == nil && exists {
			// use the current context in kubeconfig
			config, err = clientcmd.BuildConfigFromFlags(masterURL, *kubeconfig)
			if err != nil {
				return nil, err
			}
		}
	}
	if config == nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	if config != nil && f.bearerToken != "" {
		config.BearerToken = f.bearerToken
		return config, nil
	}

	user := f.getImpersonateUser()
	if config != nil && user != "" && config.Impersonate.UserName == "" {
		config.Impersonate.UserName = user
	}

	// for testing purposes one can enable tracing of Kube REST API calls
	traceKubeAPI := os.Getenv("TRACE_KUBE_API")
	if traceKubeAPI == "1" || traceKubeAPI == "on" {
		config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return &trace.Tracer{RoundTripper: rt}
		}
	}
	return config, nil
}

func (f *factory) createKubeConfigText() *string {
	var kubeconfig *string
	if f.kubeConfigCache != nil {
		return f.kubeConfigCache
	}
	text := ""
	if home := util.HomeDir(); home != "" {
		text = filepath.Join(home, ".kube", "config")
	}
	kubeconfig = &text
	f.kubeConfigCache = kubeconfig
	return kubeconfig
}

func (f *factory) getImpersonateUser() string {
	user := f.impersonateUser
	if user == "" {
		// this is really only used for testing really
		user = os.Getenv("JX_IMPERSONATE_USER")
	}
	return user
}

func (f *factory) CreateJXClient() (versioned.Interface, string, error) {
	config, err := f.CreateKubeConfig()
	if err != nil {
		return nil, "", err
	}

	kubeConfig, _, err := f.kubeConfig.LoadConfig()
	if err != nil {
		return nil, "", err
	}
	ns := kube.CurrentNamespace(kubeConfig)
	client, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, ns, err
	}
	return client, ns, err
}

func (f *factory) CreateTektonClient() (tektonclient.Interface, string, error) {
	config, err := f.CreateKubeConfig()
	if err != nil {
		return nil, "", err
	}
	kubeConfig, _, err := f.kubeConfig.LoadConfig()
	if err != nil {
		return nil, "", err
	}
	ns := kube.CurrentNamespace(kubeConfig)
	client, err := tektonclient.NewForConfig(config)
	if err != nil {
		return nil, ns, err
	}
	return client, ns, err
}
