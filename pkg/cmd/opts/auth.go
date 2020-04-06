package opts

import (
	"fmt"

	jxv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	v1fake "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apifake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/kube"
)

// SetFakeKubeClient creates a fake KubeClient for CommonOptions
// Use this in case there is no active cluster that can be used
// to retrieve configuration information.
func (o *CommonOptions) SetFakeKubeClient() error {
	currentNamespace := "jx"
	k8sObjects := []runtime.Object{}
	jxObjects := []runtime.Object{}
	devEnv := kube.NewPermanentEnvironment("dev")
	devEnv.Spec.Namespace = currentNamespace
	devEnv.Spec.Kind = jxv1.EnvironmentKindTypeDevelopment

	k8sObjects = append(k8sObjects, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: currentNamespace,
			Labels: map[string]string{
				"tag": "",
			},
		},
	})

	client := fake.NewSimpleClientset(k8sObjects...)
	o.SetKubeClient(client)
	jxObjects = append(jxObjects, devEnv)
	o.SetJxClient(v1fake.NewSimpleClientset(jxObjects...))
	o.SetAPIExtensionsClient(apifake.NewSimpleClientset())
	return nil
}

// CreateChatAuthConfigService creates a new chat auth service
func (o *CommonOptions) CreateChatAuthConfigService(kind string) (auth.ConfigService, error) {
	_, namespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find development namespace")
	}
	return o.factory.CreateChatAuthConfigService(namespace, kind)
}

// PickPipelineUserAuth returns the user auth for pipeline user
func (o *CommonOptions) PickPipelineUserAuth(config *auth.AuthConfig, server *auth.AuthServer) (*auth.UserAuth, error) {
	userName := config.PipeLineUsername
	if userName != "" {
		userAuth := config.GetOrCreateUserAuth(server.URL, userName)
		if userAuth != nil {
			return userAuth, nil
		}
	}
	var userAuth *auth.UserAuth
	var err error
	url := server.URL
	userAuths := config.FindUserAuths(url)
	if len(userAuths) > 1 {
		userAuth, err = config.PickServerUserAuth(server, "user name for the Pipeline", o.BatchMode, "", o.GetIOFileHandles())
		if err != nil {
			return userAuth, err
		}
	}
	if userAuth != nil {
		config.PipeLineUsername = userAuth.Username
	} else {
		// lets create an empty one for now
		userAuth = &auth.UserAuth{}
	}
	return userAuth, nil
}

// GetDefaultAdminPassword returns the default admin password from dev namespace
func (o *CommonOptions) GetDefaultAdminPassword(devNamespace string) (string, error) {
	client, err := o.KubeClient() // cache may not have been created yet...
	if err != nil {
		return "", fmt.Errorf("cannot obtain k8s client %v", err)
	}
	basicAuth, err := client.CoreV1().Secrets(devNamespace).Get(JXInstallConfig, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot find secret %s in namespace %s: %v", kube.SecretBasicAuth, devNamespace, err)
	}
	adminSecrets := basicAuth.Data[AdminSecretsFile]
	adminConfig := config.AdminSecretsConfig{}

	err = yaml.Unmarshal(adminSecrets, &adminConfig)
	if err != nil {
		return "", err
	}
	return adminConfig.Jenkins.JenkinsSecret.Password, nil
}

// JenkinsAuthConfigService creates the jenkins auth config service
func (o *CommonOptions) JenkinsAuthConfigService(namespace string, selector *JenkinsSelectorOptions) (auth.ConfigService, error) {
	if o.factory == nil {
		return nil, errors.New("command factory is not initialized")
	}
	prow, err := o.IsProw()
	if err != nil {
		return nil, err
	}
	if prow {
		selector.UseCustomJenkins = true
	}
	jenkinsServiceName := ""
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		namespace = ns
	}
	if selector.IsCustom() {
		jenkinsServiceName, err = o.PickCustomJenkinsName(selector, kubeClient, ns)
		if err != nil {
			return nil, err
		}
	}
	return o.factory.CreateJenkinsAuthConfigService(namespace, jenkinsServiceName)
}

// ChartmuseumAuthConfigService creates the chart museum auth config service
func (o *CommonOptions) ChartmuseumAuthConfigService() (auth.ConfigService, error) {
	if o.factory == nil {
		return nil, errors.New("command factory is not initialized")
	}
	_, namespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find development namespace")
	}
	return o.factory.CreateChartmuseumAuthConfigService(namespace, "")
}

// GitAuthConfigService create the git auth config service
func (o *CommonOptions) GitAuthConfigService() (auth.ConfigService, error) {
	if o.factory == nil {
		return nil, errors.New("command factory is not initialized")
	}
	_, namespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find development namespace")
	}
	return o.factory.CreateGitAuthConfigService(namespace, "")
}

// GitAuthConfigServiceGitHubAppMode create the git auth config service optionally handling github app mode
func (o *CommonOptions) GitAuthConfigServiceGitHubAppMode(serviceKind string) (auth.ConfigService, error) {
	client, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, errors.Wrap(err, "creating the kube client")
	}
	if serviceKind == "" {
		serviceKind = "github"
	}
	authService := auth.NewKubeAuthConfigService(client, ns, kube.ValueKindGit, serviceKind)
	if _, err := authService.LoadConfig(); err != nil {
		return nil, errors.Wrap(err, "loading auth config from kubernetes secrets")
	}
	return authService, nil
}

// GitLocalAuthConfigService create a git auth config service using the local gitAuth.yaml file method only
func (o *CommonOptions) GitLocalAuthConfigService() (auth.ConfigService, error) {
	if o.factory == nil {
		return nil, errors.New("command factory is not initialized")
	}
	return o.factory.CreateLocalGitAuthConfigService()
}
