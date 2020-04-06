package fake

import (
	"io"
	"os"

	"github.com/jenkins-x/jx/pkg/kustomize"
	"k8s.io/client-go/dynamic"

	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/builds"
	v1fake "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	kservefake "github.com/knative/serving/pkg/client/clientset/versioned/fake"
	apifake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/client-go/kubernetes/fake"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/jenkins-x/jx/pkg/vault"
	certmngclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	fake_certmngclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned/fake"

	vaultoperatorclient "github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	fake_vaultoperatorclient "github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned/fake"
	"github.com/heptio/sonobuoy/pkg/client"
	sonoboy_dynamic "github.com/heptio/sonobuoy/pkg/dynamic"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/table"
	fake_vault "github.com/jenkins-x/jx/pkg/vault/fake"
	kserve "github.com/knative/serving/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	fake_metricsclient "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	prowjobclient "k8s.io/test-infra/prow/client/clientset/versioned"
	fake_prowjobclient "k8s.io/test-infra/prow/client/clientset/versioned/fake"
)

// FakeFactory points to a fake factory implementation
type FakeFactory struct {
	Batch bool

	delegate        clients.Factory
	namespace       string
	kubeConfig      kube.Kuber
	impersonateUser string
	bearerToken     string
	secretLocation  secrets.SecretLocation
	offline         bool

	// cached fake clients
	apiClient     apiextensionsclientset.Interface
	jxClient      versioned.Interface
	kubeClient    kubernetes.Interface
	kserveClient  kserve.Interface
	tektonClient  tektonclient.Interface
	prowJobClient prowjobclient.Interface
	dyncClient    dynamic.Interface
}

var _ clients.Factory = (*FakeFactory)(nil)

// NewFakeFactory creates a fake factory which uses fake k8s clients for testing
func NewFakeFactory() clients.Factory {
	f := &FakeFactory{
		namespace: "jx",
	}
	f.kubeConfig = kube.NewKubeConfig()
	return f
}

// NewFakeFactoryFromClients creates a fake factory which uses fake k8s clients for testing
func NewFakeFactoryFromClients(apiClient apiextensionsclientset.Interface,
	jxClient versioned.Interface,
	kubeClient kubernetes.Interface,
	tektonClient tektonclient.Interface,
	dyncClient dynamic.Interface) *FakeFactory {
	f := &FakeFactory{
		namespace:    "jx",
		apiClient:    apiClient,
		jxClient:     jxClient,
		kubeClient:   kubeClient,
		tektonClient: tektonClient,
		dyncClient:   dyncClient,
	}
	f.kubeConfig = kube.NewKubeConfig()
	return f
}

// SetDelegateFactory sets the delegate factory
func (f *FakeFactory) SetDelegateFactory(factory clients.Factory) {
	f.delegate = factory
}

// GetDelegateFactory returns the delegate factory
func (f *FakeFactory) GetDelegateFactory() clients.Factory {
	if f.delegate == nil {
		f.delegate = clients.NewFactory()
	}
	return f.delegate
}

// SetNamespace sets the default namespace
func (f *FakeFactory) SetNamespace(ns string) {
	f.namespace = ns
}

// SetBatch sets batch
func (f *FakeFactory) SetBatch(batch bool) {
	f.Batch = batch
}

// SetOffline sets offline
func (f *FakeFactory) SetOffline(offline bool) {
	f.offline = offline
}

// ImpersonateUser returns a new factory impersonating the given user
func (f *FakeFactory) ImpersonateUser(user string) clients.Factory {
	copy := *f
	copy.impersonateUser = user
	return &copy
}

// WithBearerToken returns a new factory with bearer token
func (f *FakeFactory) WithBearerToken(token string) clients.Factory {
	copy := *f
	copy.bearerToken = token
	return &copy
}

// CreateJenkinsClient creates a new Jenkins client
func (f *FakeFactory) CreateJenkinsClient(kubeClient kubernetes.Interface, ns string, handles util.IOFileHandles) (gojenkins.JenkinsClient, error) {
	return f.GetDelegateFactory().CreateJenkinsClient(kubeClient, ns, handles)
}

// CreateCustomJenkinsClient creates a new Jenkins client for the given custom Jenkins App
func (f *FakeFactory) CreateCustomJenkinsClient(kubeClient kubernetes.Interface, ns string, jenkinsServiceName string, handles util.IOFileHandles) (gojenkins.JenkinsClient, error) {
	return f.GetDelegateFactory().CreateCustomJenkinsClient(kubeClient, ns, jenkinsServiceName, handles)
}

// GetJenkinsURL gets the Jenkins URL for the given namespace
func (f *FakeFactory) GetJenkinsURL(kubeClient kubernetes.Interface, ns string) (string, error) {
	return f.GetDelegateFactory().GetJenkinsURL(kubeClient, ns)
}

// GetCustomJenkinsURL gets a custom jenkins App service URL
func (f *FakeFactory) GetCustomJenkinsURL(kubeClient kubernetes.Interface, ns string, jenkinsServiceName string) (string, error) {
	return f.GetDelegateFactory().GetCustomJenkinsURL(kubeClient, ns, jenkinsServiceName)
}

// CreateJenkinsAuthConfigService creates a new Jenkins authentication configuration service
func (f *FakeFactory) CreateJenkinsAuthConfigService(namespace string, jenkinsServiceName string) (auth.ConfigService, error) {
	return f.CreateAuthConfigService(auth.JenkinsAuthConfigFile, namespace, kube.ValueKindJenkins, "")
}

// CreateChartmuseumAuthConfigService creates a new Chartmuseum authentication configuration service
func (f *FakeFactory) CreateChartmuseumAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.CreateAuthConfigService(auth.ChartmuseumAuthConfigFile, namespace, kube.ValueKindChartmuseum, serviceKind)
}

// CreateIssueTrackerAuthConfigService creates a new issuer tracker configuration service
func (f *FakeFactory) CreateIssueTrackerAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.CreateAuthConfigService(auth.IssuesAuthConfigFile, namespace, kube.ValueKindIssue, serviceKind)
}

// CreateChatAuthConfigService creates a new chat configuration service
func (f *FakeFactory) CreateChatAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.CreateAuthConfigService(auth.ChatAuthConfigFile, namespace, kube.ValueKindChat, serviceKind)
}

// CreateAddonAuthConfigService creates a new addon auth configuration service
func (f *FakeFactory) CreateAddonAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.CreateAuthConfigService(auth.AddonAuthConfigFile, namespace, kube.ValueKindAddon, serviceKind)
}

// CreateGitAuthConfigService creates a new git  auth configuration service
func (f *FakeFactory) CreateGitAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.CreateAuthConfigService(auth.GitAuthConfigFile, namespace, kube.ValueKindGit, serviceKind)
}

// CreateAuthConfigService creates a new service which loads/saves the auth config from/to different sources depending
// on the current secrets location and cluster context. The sources can be vault, kubernetes secrets or local file.
func (f *FakeFactory) CreateAuthConfigService(fileName string, namespace string,
	serverKind string, serviceKind string) (auth.ConfigService, error) {
	configService := auth.NewMemoryAuthConfigService()
	username := "fake-username"
	url := "https://fake-server.org"
	kind := serviceKind
	if serverKind == kube.ValueKindGit {
		kind = gits.KindGitFake
	}
	config := &auth.AuthConfig{
		Servers: []*auth.AuthServer{
			{
				URL: url,
				Users: []*auth.UserAuth{
					{
						Username: username,
						ApiToken: "fake-token",
					},
				},
				Kind:        kind,
				Name:        serviceKind,
				CurrentUser: username,
			},
		},
		CurrentServer:    url,
		PipeLineUsername: username,
		PipeLineServer:   url,
	}
	configService.SetConfig(config)
	return configService, nil
}

// SecretsLocation indicates the location where the secrets are stored
func (f *FakeFactory) SecretsLocation() secrets.SecretsLocationKind {
	return secrets.FileSystemLocationKind
}

// SetSecretsLocation configures the secrets location. It will persist the value in a config map
// if the persist flag is set.
func (f *FakeFactory) SetSecretsLocation(location secrets.SecretsLocationKind, persist bool) error {
	return nil
}

// ResetSecretsLocation resets the location of the secrets stored in memory
func (f *FakeFactory) ResetSecretsLocation() {
	f.secretLocation = nil
}

// CreateSystemVaultClient gets the system vault client for managing the secrets
func (f *FakeFactory) CreateSystemVaultClient(namespace string) (vault.Client, error) {
	return fake_vault.NewFakeVaultClient(), nil
}

// CreateVaultClient returns the given vault client for managing secrets
// Will use default values for name and namespace if nil values are applied
func (f *FakeFactory) CreateVaultClient(name string, namespace string) (vault.Client, error) {
	return fake_vault.NewFakeVaultClient(), nil
}

// CreateKubeClient creates a new Kubernetes client
func (f *FakeFactory) CreateKubeClient() (kubernetes.Interface, string, error) {
	if f.kubeClient == nil {
		f.kubeClient = fake.NewSimpleClientset()
	}
	return f.kubeClient, f.namespace, nil
}

// CreateJXClient creates a new Kubernetes client for Jenkins X CRDs
func (f *FakeFactory) CreateJXClient() (versioned.Interface, string, error) {
	if f.jxClient == nil {
		f.jxClient = v1fake.NewSimpleClientset()
	}
	return f.jxClient, f.namespace, nil
}

// CreateApiExtensionsClient creates a new Kubernetes ApiExtensions client
func (f *FakeFactory) CreateApiExtensionsClient() (apiextensionsclientset.Interface, error) {
	if f.apiClient == nil {
		f.apiClient = apifake.NewSimpleClientset()
	}
	return f.apiClient, nil
}

// CreateProwJobClient creates a new Kubernetes client for ProwJob resources
func (f *FakeFactory) CreateProwJobClient() (prowjobclient.Interface, string, error) {
	if f.prowJobClient == nil {
		f.prowJobClient = fake_prowjobclient.NewSimpleClientset()
	}
	return f.prowJobClient, f.namespace, nil
}

// CreateKnativeServeClient create a new Kubernetes client for Knative serve resources
func (f *FakeFactory) CreateKnativeServeClient() (kserve.Interface, string, error) {
	if f.kserveClient == nil {
		f.kserveClient = kservefake.NewSimpleClientset()
	}
	return f.kserveClient, f.namespace, nil
}

// CreateTektonClient create a new Kubernetes client for Tekton resources
func (f *FakeFactory) CreateTektonClient() (tektonclient.Interface, string, error) {
	if f.tektonClient == nil {
		f.tektonClient = tektonfake.NewSimpleClientset()
	}
	return f.tektonClient, f.namespace, nil
}

// CreateDynamicClient creates a new Kubernetes Dynamic client
func (f *FakeFactory) CreateDynamicClient() (dynamic.Interface, string, error) {
	if f.dyncClient == nil {
		config, err := f.CreateKubeConfig()
		if err != nil {
			return nil, "", err
		}
		kubeConfig, _, err := f.kubeConfig.LoadConfig()
		if err != nil {
			return nil, "", err
		}
		ns := kube.CurrentNamespace(kubeConfig)
		f.dyncClient, err = dynamic.NewForConfig(config)
		if err != nil {
			return nil, ns, err
		}
		return f.dyncClient, ns, err
	}
	return f.dyncClient, f.namespace, nil
}

// CreateMetricsClient creates a new Kubernetes metrics client
func (f *FakeFactory) CreateMetricsClient() (metricsclient.Interface, error) {
	return fake_metricsclient.NewSimpleClientset(), nil
}

// CreateGitProvider creates a new Git provider
func (f *FakeFactory) CreateGitProvider(gitURL string, message string, authConfigSvc auth.ConfigService,
	gitKind string, ghOwner string, batchMode bool, gitter gits.Gitter, handles util.IOFileHandles) (gits.GitProvider, error) {
	return f.GetDelegateFactory().CreateGitProvider(gitURL, message, authConfigSvc, gitKind, ghOwner, batchMode, gitter, handles)
}

// CreateKubeConfig creates the kubernetes configuration
func (f *FakeFactory) CreateKubeConfig() (*rest.Config, error) {
	return f.GetDelegateFactory().CreateKubeConfig()
}

// CreateTable creates a new table
func (f *FakeFactory) CreateTable(out io.Writer) table.Table {
	return table.CreateTable(out)
}

// IsInCDPipeline we should only load the git / issue tracker API tokens if the current pod
// is in a pipeline and running as the Jenkins service account
func (f *FakeFactory) IsInCDPipeline() bool {
	// TODO should we let RBAC decide if we can see the Secrets in the dev namespace?
	// or we should test if we are in the cluster and get the current ServiceAccount name?
	buildNumber := builds.GetBuildNumber()
	return buildNumber != "" || os.Getenv("PIPELINE_KIND") != ""
}

// function to tell if we are running incluster
func (f *FakeFactory) IsInCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}

// CreateComplianceClient creates a new Sonobuoy compliance client
func (f *FakeFactory) CreateComplianceClient() (*client.SonobuoyClient, error) {
	config, err := f.CreateKubeConfig()
	if err != nil {
		return nil, errors.Wrap(err, "compliance client failed to load the Kubernetes configuration")
	}
	skc, err := sonoboy_dynamic.NewAPIHelperFromRESTConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "compliance dynamic client failed to be created")
	}
	return client.NewSonobuoyClient(config, skc)
}

// CreateVaultOperatorClient creates a new vault operator client
func (f *FakeFactory) CreateVaultOperatorClient() (vaultoperatorclient.Interface, error) {
	return fake_vaultoperatorclient.NewSimpleClientset(), nil
}

// CreateHelm creates a new Helm client
func (f *FakeFactory) CreateHelm(verbose bool,
	helmBinary string,
	noTiller bool,
	helmTemplate bool) helm.Helmer {

	return f.GetDelegateFactory().CreateHelm(verbose,
		helmBinary,
		noTiller,
		helmTemplate)
}

// CreateCertManagerClient creates a new Kuberntes client for cert-manager resources
func (f *FakeFactory) CreateCertManagerClient() (certmngclient.Interface, error) {
	return fake_certmngclient.NewSimpleClientset(), nil
}

// CreateLocalGitAuthConfigService creates a new service which loads/saves the auth config from/to a local file.
func (f *FakeFactory) CreateLocalGitAuthConfigService() (auth.ConfigService, error) {
	return f.GetDelegateFactory().CreateLocalGitAuthConfigService()
}

// CreateKustomizer creates a Kustomizer client
func (f *FakeFactory) CreateKustomizer() kustomize.Kustomizer {
	return f.GetDelegateFactory().CreateKustomizer()
}
