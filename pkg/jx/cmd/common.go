package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/expose"

	"github.com/jenkins-x/jx/pkg/certmanager"

	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"

	vaultoperatorclient "github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	"github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	buildclient "github.com/knative/build/pkg/client/clientset/versioned"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/table"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
	gitcfg "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	optionServerName       = "name"
	optionServerURL        = "url"
	optionBatchMode        = "batch-mode"
	optionVerbose          = "verbose"
	optionLogLevel         = "log-level"
	optionHeadless         = "headless"
	optionNoBrew           = "no-brew"
	optionInstallDeps      = "install-dependencies"
	optionSkipAuthSecMerge = "skip-auth-secrets-merge"
	optionPullSecrets      = "pull-secrets"
)

// ModifyDevEnvironmentFn a callback to create/update the development Environment
type ModifyDevEnvironmentFn func(callback func(env *jenkinsv1.Environment) error) error

// ModifyEnvironmentFn a callback to create/update an Environment
type ModifyEnvironmentFn func(name string, callback func(env *jenkinsv1.Environment) error) error

// CommonOptions contains common options and helper methods
type CommonOptions struct {
	Factory                Factory
	In                     terminal.FileReader
	Out                    terminal.FileWriter
	Err                    io.Writer
	Cmd                    *cobra.Command
	Args                   []string
	BatchMode              bool
	Verbose                bool
	LogLevel               string
	Headless               bool
	NoBrew                 bool
	InstallDependencies    bool
	SkipAuthSecretsMerge   bool
	ServiceAccount         string
	Username               string
	ExternalJenkinsBaseURL string
	PullSecrets            string

	// common cached clients
	KubeClientCached       kubernetes.Interface
	apiExtensionsClient    apiextensionsclientset.Interface
	currentNamespace       string
	devNamespace           string
	jxClient               versioned.Interface
	knbClient              buildclient.Interface
	jenkinsClient          gojenkins.JenkinsClient
	GitClient              gits.Gitter
	helm                   helm.Helmer
	Kuber                  kube.Kuber
	vaultOperatorClient    vaultoperatorclient.Interface
	modifyDevEnvironmentFn ModifyDevEnvironmentFn
	modifyEnvironmentFn    ModifyEnvironmentFn

	Prow
}

type ServerFlags struct {
	ServerName string
	ServerURL  string
}

// IsEmpty returns true if the server flags and server URL are tempry
func (f *ServerFlags) IsEmpty() bool {
	return f.ServerName == "" && f.ServerURL == ""
}

// CreateTable creates a new Table
func (o *CommonOptions) CreateTable() table.Table {
	return o.Factory.CreateTable(o.Out)
}

// NewCommonOptions a helper method to create a new CommonOptions instance
// pre configured in a specific devNamespace
func NewCommonOptions(devNamespace string, factory Factory) CommonOptions {
	return CommonOptions{
		Factory:          factory,
		Out:              os.Stdout,
		Err:              os.Stderr,
		currentNamespace: devNamespace,
		devNamespace:     devNamespace,
	}
}

// SetDevNamespace configures the current dev namespace
func (o *CommonOptions) SetDevNamespace(ns string) {
	o.devNamespace = ns
	o.currentNamespace = ns
	o.KubeClientCached = nil
}

// Debugf outputs the given text to the console if verbose mode is enabled
func (o *CommonOptions) Debugf(format string, a ...interface{}) {
	if o.Verbose {
		log.Infof(format, a...)
	}
}

func (options *CommonOptions) addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&options.BatchMode, optionBatchMode, "b", false, "In batch mode the command never prompts for user input")
	cmd.Flags().BoolVarP(&options.Verbose, optionVerbose, "", false, "Enable verbose logging")
	cmd.Flags().StringVarP(&options.LogLevel, optionLogLevel, "", logrus.InfoLevel.String(), "Logging level. Possible values - panic, fatal, error, warning, info, debug.")
	cmd.Flags().BoolVarP(&options.Headless, optionHeadless, "", false, "Enable headless operation if using browser automation")
	cmd.Flags().BoolVarP(&options.NoBrew, optionNoBrew, "", false, "Disables the use of brew on macOS to install or upgrade command line dependencies")
	cmd.Flags().BoolVarP(&options.InstallDependencies, optionInstallDeps, "", false, "Should any required dependencies be installed automatically")
	cmd.Flags().BoolVarP(&options.SkipAuthSecretsMerge, optionSkipAuthSecMerge, "", false, "Skips merging a local git auth yaml file with any pipeline secrets that are found")
	cmd.Flags().StringVarP(&options.PullSecrets, optionPullSecrets, "", "", "The pull secrets the service account created should have (useful when deploying to your own private registry): provide multiple pull secrets by providing them in a singular block of quotes e.g. --pull-secrets \"foo, bar, baz\"")

	options.Cmd = cmd
}

func (o *CommonOptions) CreateApiExtensionsClient() (apiextensionsclientset.Interface, error) {
	var err error
	if o.apiExtensionsClient == nil {
		o.apiExtensionsClient, err = o.Factory.CreateApiExtensionsClient()
		if err != nil {
			return nil, err
		}
	}
	return o.apiExtensionsClient, nil
}

func (o *CommonOptions) KubeClient() (kubernetes.Interface, string, error) {
	if o.KubeClientCached == nil {
		kubeClient, currentNs, err := o.Factory.CreateClient()
		if err != nil {
			return nil, "", err
		}
		o.KubeClientCached = kubeClient
		o.currentNamespace = currentNs

	}
	return o.KubeClientCached, o.currentNamespace, nil
}

// KubeClientAndDevNamespace returns a kube client and the development namespace
func (o *CommonOptions) KubeClientAndDevNamespace() (kubernetes.Interface, string, error) {
	kubeClient, curNs, err := o.KubeClient()
	if err != nil {
		return nil, "", err
	}
	if o.devNamespace == "" {
		o.devNamespace, _, err = kube.GetDevNamespace(kubeClient, curNs)
	}
	return kubeClient, o.devNamespace, err
}

func (o *CommonOptions) JXClient() (versioned.Interface, string, error) {
	if o.Factory == nil {
		return nil, "", errors.New("command factory is not initialized")
	}
	if o.jxClient == nil {
		jxClient, ns, err := o.Factory.CreateJXClient()
		if err != nil {
			return nil, ns, err
		}
		o.jxClient = jxClient
		if o.currentNamespace == "" {
			o.currentNamespace = ns
		}
	}
	return o.jxClient, o.currentNamespace, nil
}

func (o *CommonOptions) KnativeBuildClient() (buildclient.Interface, string, error) {
	if o.Factory == nil {
		return nil, "", errors.New("command factory is not initialized")
	}
	if o.knbClient == nil {
		knbClient, ns, err := o.Factory.CreateKnativeBuildClient()
		if err != nil {
			return nil, ns, err
		}
		o.knbClient = knbClient
		if o.currentNamespace == "" {
			o.currentNamespace = ns
		}
	}
	return o.knbClient, o.currentNamespace, nil
}

func (o *CommonOptions) JXClientAndAdminNamespace() (versioned.Interface, string, error) {
	kubeClient, _, err := o.KubeClient()
	if err != nil {
		return nil, "", err
	}
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return nil, "", err
	}

	ns, err := kube.GetAdminNamespace(kubeClient, devNs)
	return jxClient, ns, err
}

func (o *CommonOptions) JXClientAndDevNamespace() (versioned.Interface, string, error) {
	if o.jxClient == nil {
		jxClient, ns, err := o.JXClient()
		if err != nil {
			return nil, ns, err
		}
		o.jxClient = jxClient
		if o.currentNamespace == "" {
			o.currentNamespace = ns
		}
	}
	if o.devNamespace == "" {
		client, ns, err := o.KubeClient()
		if err != nil {
			return nil, "", err
		}
		devNs, _, err := kube.GetDevNamespace(client, ns)
		if err != nil {
			return nil, "", err
		}
		o.devNamespace = devNs
	}
	return o.jxClient, o.devNamespace, nil
}

// SetJenkinsClient sets the JenkinsClient - usually used in testing
func (o *CommonOptions) SetJenkinsClient(jenkinsClient gojenkins.JenkinsClient) {
	o.jenkinsClient = jenkinsClient
}

func (o *CommonOptions) JenkinsClient() (gojenkins.JenkinsClient, error) {
	if o.jenkinsClient == nil {
		kubeClient, ns, err := o.KubeClientAndDevNamespace()
		if err != nil {
			return nil, err
		}

		jenkins, err := o.Factory.CreateJenkinsClient(kubeClient, ns, o.In, o.Out, o.Err)

		if err != nil {
			return nil, err
		}
		o.jenkinsClient = jenkins
	}
	return o.jenkinsClient, nil
}
func (o *CommonOptions) GetJenkinsURL() (string, error) {
	kubeClient, ns, err := o.KubeClient()
	if err != nil {
		return "", err
	}

	return o.Factory.GetJenkinsURL(kubeClient, ns)
}

func (o *CommonOptions) Git() gits.Gitter {
	if o.GitClient == nil {
		o.GitClient = gits.NewGitCLI()
	}
	return o.GitClient
}

func (o *CommonOptions) Helm() helm.Helmer {
	if o.helm == nil {
		helmBinary, noTiller, helmTemplate, _ := o.TeamHelmBin()
		o.helm = o.Factory.GetHelm(o.Verbose, helmBinary, noTiller, helmTemplate)
	}
	return o.helm
}

// SetHelm sets the helmer used for this object
func (o *CommonOptions) SetHelm(helmer helm.Helmer) {
	o.helm = helmer
}

func (o *CommonOptions) Kube() kube.Kuber {
	if o.Kuber == nil {
		o.Kuber = kube.NewKubeConfig()
	}
	return o.Kuber
}

func (o *CommonOptions) TeamAndEnvironmentNames() (string, string, error) {
	kubeClient, currentNs, err := o.KubeClient()
	if err != nil {
		return "", "", err
	}
	return kube.GetDevNamespace(kubeClient, currentNs)
}

func (o *CommonOptions) GetImagePullSecrets() []string {
	pullSecrets := strings.Fields(o.PullSecrets)
	return pullSecrets
}

func (o *ServerFlags) addGitServerFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.ServerName, optionServerName, "n", "", "The name of the Git server to add a user")
	cmd.Flags().StringVarP(&o.ServerURL, optionServerURL, "u", "", "The URL of the Git server to add a user")
}

// findGitServer finds the Git server from the given flags or returns an error
func (o *CommonOptions) findGitServer(config *auth.AuthConfig, serverFlags *ServerFlags) (*auth.AuthServer, error) {
	return o.findServer(config, serverFlags, "git", "Try creating one via: jx create git server", false)
}

// findIssueTrackerServer finds the issue tracker server from the given flags or returns an error
func (o *CommonOptions) findIssueTrackerServer(config *auth.AuthConfig, serverFlags *ServerFlags) (*auth.AuthServer, error) {
	return o.findServer(config, serverFlags, "issues", "Try creating one via: jx create tracker server", false)
}

// findChatServer finds the chat server from the given flags or returns an error
func (o *CommonOptions) findChatServer(config *auth.AuthConfig, serverFlags *ServerFlags) (*auth.AuthServer, error) {
	return o.findServer(config, serverFlags, "chat", "Try creating one via: jx create chat server", false)
}

// findAddonServer finds the addon server from the given flags or returns an error
func (o *CommonOptions) findAddonServer(config *auth.AuthConfig, serverFlags *ServerFlags, kind string) (*auth.AuthServer, error) {
	return o.findServer(config, serverFlags, kind, "Try creating one via: jx create addon", true)
}

func (o *CommonOptions) findServer(config *auth.AuthConfig, serverFlags *ServerFlags, defaultKind string, missingServerDescription string, lazyCreate bool) (*auth.AuthServer, error) {
	kind := defaultKind
	var server *auth.AuthServer
	if serverFlags.ServerURL != "" {
		server = config.GetServer(serverFlags.ServerURL)
		if server == nil {
			if lazyCreate {
				return config.GetOrCreateServerName(serverFlags.ServerURL, serverFlags.ServerName, kind), nil
			}
			return nil, util.InvalidOption(optionServerURL, serverFlags.ServerURL, config.GetServerURLs())
		}
	}
	if server == nil && serverFlags.ServerName != "" {
		name := serverFlags.ServerName
		if lazyCreate {
			server = config.GetOrCreateServerName(serverFlags.ServerURL, name, kind)
		} else {
			server = config.GetServerByName(name)
		}
		if server == nil {
			return nil, util.InvalidOption(optionServerName, name, config.GetServerNames())
		}
	}
	if server == nil {
		name := config.CurrentServer
		if name != "" && o.BatchMode {
			server = config.GetServerByName(name)
			if server == nil {
				log.Warnf("Current server %s no longer exists\n", name)
			}
		}
	}
	if server == nil && len(config.Servers) == 1 {
		server = config.Servers[0]
	}
	if server == nil && len(config.Servers) > 1 {
		if o.BatchMode {
			return nil, fmt.Errorf("Multiple servers found. Please specify one via the %s option", optionServerName)
		}
		defaultServerName := ""
		if config.CurrentServer != "" {
			s := config.GetServer(config.CurrentServer)
			if s != nil {
				defaultServerName = s.Name
			}
		}
		name, err := util.PickNameWithDefault(config.GetServerNames(), "Pick server to use: ", defaultServerName, "", o.In, o.Out, o.Err)
		if err != nil {
			return nil, err
		}
		server = config.GetServerByName(name)
		if server == nil {
			return nil, fmt.Errorf("Could not find the server for name %s", name)
		}
	}
	if server == nil {
		return nil, fmt.Errorf("Could not find a %s. %s", kind, missingServerDescription)
	}
	return server, nil
}

func (o *CommonOptions) findService(name string) (string, error) {
	client, ns, err := o.KubeClient()
	if err != nil {
		return "", err
	}
	devNs, _, err := kube.GetDevNamespace(client, ns)
	if err != nil {
		return "", err
	}
	url, err := services.FindServiceURL(client, ns, name)
	if url == "" {
		url, err = services.FindServiceURL(client, devNs, name)
	}
	if url == "" {
		names, err := services.GetServiceNames(client, ns, name)
		if err != nil {
			return "", err
		}
		if len(names) > 1 {
			name, err = util.PickName(names, "Pick service to open: ", "", o.In, o.Out, o.Err)
			if err != nil {
				return "", err
			}
			if name != "" {
				url, err = services.FindServiceURL(client, ns, name)
			}
		} else if len(names) == 1 {
			// must have been a filter
			url, err = services.FindServiceURL(client, ns, names[0])
		}
		if url == "" {
			return "", fmt.Errorf("Could not find URL for service %s in namespace %s", name, ns)
		}
	}
	return url, nil
}

func (o *CommonOptions) findEnvironmentNamespace(envName string) (string, error) {
	client, ns, err := o.KubeClient()
	if err != nil {
		return "", err
	}
	jxClient, _, err := o.JXClient()
	if err != nil {
		return "", err
	}

	devNs, _, err := kube.GetDevNamespace(client, ns)
	if err != nil {
		return "", err
	}

	envMap, envNames, err := kube.GetEnvironments(jxClient, devNs)
	if err != nil {
		return "", err
	}
	env := envMap[envName]
	if env == nil {
		return "", util.InvalidOption(optionEnvironment, envName, envNames)
	}
	answer := env.Spec.Namespace
	if answer == "" {
		return "", fmt.Errorf("Environment %s does not have a Namespace!", envName)
	}
	return answer, nil
}

func (o *CommonOptions) findServiceInNamespace(name string, ns string) (string, error) {
	client, curNs, err := o.KubeClient()
	if err != nil {
		return "", err
	}
	if ns == "" {
		ns = curNs
	}
	url, err := services.FindServiceURL(client, ns, name)
	if url == "" {
		names, err := services.GetServiceNames(client, ns, name)
		if err != nil {
			return "", err
		}
		if len(names) > 1 {
			name, err = util.PickName(names, "Pick service to open: ", "", o.In, o.Out, o.Err)
			if err != nil {
				return "", err
			}
			if name != "" {
				url, err = services.FindServiceURL(client, ns, name)
			}
		} else if len(names) == 1 {
			// must have been a filter
			url, err = services.FindServiceURL(client, ns, names[0])
		}
		if url == "" {
			return "", fmt.Errorf("Could not find URL for service %s in namespace %s", name, ns)
		}
	}
	return url, nil
}

func (o *CommonOptions) retry(attempts int, sleep time.Duration, call func() error) (err error) {
	for i := 0; ; i++ {
		err = call()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)

		log.Infof("retrying after error:%s\n", err)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func (o *CommonOptions) retryQuiet(attempts int, sleep time.Duration, call func() error) (err error) {
	lastMessage := ""
	dot := false

	for i := 0; ; i++ {
		err = call()
		if err == nil {
			if dot {
				log.Blank()
			}
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)

		message := fmt.Sprintf("retrying after error: %s", err)
		if lastMessage == message {
			log.Info(".")
			dot = true
		} else {
			lastMessage = message
			if dot {
				dot = false
				log.Blank()
			}
			log.Infof("%s\n", lastMessage)
		}
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func (o *CommonOptions) retryQuietlyUntilTimeout(timeout time.Duration, sleep time.Duration, call func() error) (err error) {
	timeoutTime := time.Now().Add(timeout)

	lastMessage := ""
	dot := false

	for i := 0; ; i++ {
		err = call()
		if err == nil {
			if dot {
				log.Blank()
			}
			return
		}

		if time.Now().After(timeoutTime) {
			return fmt.Errorf("Timed out after %s, last error: %s", timeout.String(), err)
		}

		time.Sleep(sleep)

		message := fmt.Sprintf("retrying after error: %s", err)
		if lastMessage == message {
			log.Info(".")
			dot = true
		} else {
			lastMessage = message
			if dot {
				dot = false
				log.Blank()
			}
			log.Infof("%s\n", lastMessage)
		}
	}
}

// retryUntilTrueOrTimeout waits until complete is true, an error occurs or the timeout
func (o *CommonOptions) retryUntilTrueOrTimeout(timeout time.Duration, sleep time.Duration, call func() (bool, error)) (err error) {
	timeoutTime := time.Now().Add(timeout)

	for i := 0; ; i++ {
		complete, err := call()
		if complete || err != nil {
			return err
		}
		if time.Now().After(timeoutTime) {
			return fmt.Errorf("Timed out after %s, last error: %s", timeout.String(), err)
		}

		time.Sleep(sleep)
	}
}

func (o *CommonOptions) getJobMap(filter string) (map[string]gojenkins.Job, error) {
	jobMap := map[string]gojenkins.Job{}
	jenkins, err := o.JenkinsClient()
	if err != nil {
		return jobMap, err
	}
	jobs, err := jenkins.GetJobs()
	if err != nil {
		return jobMap, err
	}
	o.addJobs(&jobMap, filter, "", jobs)
	return jobMap, nil
}

func (o *CommonOptions) addJobs(jobMap *map[string]gojenkins.Job, filter string, prefix string, jobs []gojenkins.Job) {
	jenkins, err := o.JenkinsClient()
	if err != nil {
		return
	}

	for _, j := range jobs {
		name := jobName(prefix, &j)
		if IsPipeline(&j) {
			if filter == "" || strings.Contains(name, filter) {
				(*jobMap)[name] = j
				continue
			}
		}
		if j.Jobs != nil {
			o.addJobs(jobMap, filter, name, j.Jobs)
		} else {
			job, err := jenkins.GetJob(name)
			if err == nil && job.Jobs != nil {
				o.addJobs(jobMap, filter, name, job.Jobs)
			}
		}
	}
}
func (o *CommonOptions) tailBuild(jobName string, build *gojenkins.Build) error {
	jenkins, err := o.JenkinsClient()
	if err != nil {
		return nil
	}

	u, err := url.Parse(build.Url)
	if err != nil {
		return err
	}
	buildPath := u.Path
	log.Infof("%s %s\n", "tailing the log of", fmt.Sprintf("%s #%d", jobName, build.Number))
	// TODO Logger
	return jenkins.TailLog(buildPath, o.Out, time.Second, time.Hour*100)
}

func (o *CommonOptions) pickRemoteURL(config *gitcfg.Config) (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	urls := []string{}
	if config.Remotes != nil {
		for _, r := range config.Remotes {
			if r.URLs != nil {
				for _, u := range r.URLs {
					urls = append(urls, u)
				}
			}
		}
	}
	if len(urls) == 1 {
		return urls[0], nil
	}
	url := ""
	if len(urls) > 1 {
		prompt := &survey.Select{
			Message: "Choose a remote git URL:",
			Options: urls,
		}
		err := survey.AskOne(prompt, &url, nil, surveyOpts)
		if err != nil {
			return "", err
		}
	}
	return url, nil
}

// todo switch to using expose as a jx plugin
// get existing config from the devNamespace and run expose in the target environment
func (o *CommonOptions) expose(devNamespace, targetNamespace, password string) error {
	return expose.Expose(devNamespace, targetNamespace, password, o.KubeClientCached, o.Helm(), defaultInstallTimeout)
}

func (o *CommonOptions) runExposecontroller(devNamespace, targetNamespace string, ic kube.IngressConfig, services ...string) error {
	return expose.RunExposecontroller(devNamespace, targetNamespace, ic, o.KubeClientCached, o.Helm(),
		defaultInstallTimeout)
}

// CleanExposecontrollerReources cleans expose controller resources
func (o *CommonOptions) CleanExposecontrollerReources(ns string) {
	expose.CleanExposecontrollerReources(o.KubeClientCached, ns)
}

func (o *CommonOptions) getDefaultAdminPassword(devNamespace string) (string, error) {
	client, _, err := o.KubeClient() // cache may not have been created yet...
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

func (o *CommonOptions) ensureAddonServiceAvailable(serviceName string) (string, error) {
	present, err := services.IsServicePresent(o.KubeClientCached, serviceName, o.currentNamespace)
	if err != nil {
		return "", fmt.Errorf("no %s provider service found, are you in your teams dev environment?  Type `jx ns` to switch.", serviceName)
	}
	if present {
		url, err := services.GetServiceURLFromName(o.KubeClientCached, serviceName, o.currentNamespace)
		if err != nil {
			return "", fmt.Errorf("no %s provider service found, are you in your teams dev environment?  Type `jx ns` to switch.", serviceName)
		}
		return url, nil
	}

	// todo ask if user wants to install addon?
	return "", nil
}

func (o *CommonOptions) copyCertmanagerResources(targetNamespace string, ic kube.IngressConfig) error {
	return certmanager.CopyCertmanagerResources(targetNamespace, ic, o.KubeClientCached)
}

func (o *CommonOptions) getJobName() string {
	owner := os.Getenv("REPO_OWNER")
	repo := os.Getenv("REPO_NAME")
	branch := os.Getenv("BRANCH_NAME")

	if owner != "" && repo != "" && branch != "" {
		return fmt.Sprintf("%s/%s/%s", owner, repo, branch)
	}

	job := os.Getenv("JOB_NAME")
	if job != "" {
		return job
	}
	return ""
}

func (o *CommonOptions) getBuildNumber() string {
	buildNumber := os.Getenv("JX_BUILD_NUMBER")
	if buildNumber != "" {
		return buildNumber
	}
	buildNumber = os.Getenv("BUILD_NUMBER")
	if buildNumber != "" {
		return buildNumber
	}
	buildID := os.Getenv("BUILD_ID")
	if buildID != "" {
		return buildID
	}
	return ""
}

func (o *CommonOptions) VaultOperatorClient() (vaultoperatorclient.Interface, error) {
	if o.Factory == nil {
		return nil, errors.New("command factory is not initialized")
	}
	if o.vaultOperatorClient == nil {
		vaultOperatorClient, err := o.Factory.CreateVaultOperatorClient()
		if err != nil {
			return nil, err
		}
		o.vaultOperatorClient = vaultOperatorClient
	}
	return o.vaultOperatorClient, nil
}

func (o *CommonOptions) GetWebHookEndpoint() (string, error) {
	_, _, err := o.JXClient()
	if err != nil {
		return "", errors.Wrap(err, "failed to get jxclient")
	}

	_, _, err = o.KubeClient()
	if err != nil {
		return "", errors.Wrap(err, "failed to get kube client")
	}

	isProwEnabled, err := o.isProw()
	if err != nil {
		return "", err
	}

	ns, _, err := kube.GetDevNamespace(o.KubeClientCached, o.currentNamespace)
	if err != nil {
		return "", err
	}

	var webHookUrl string

	if isProwEnabled {
		baseURL, err := services.GetServiceURLFromName(o.KubeClientCached, "hook", ns)
		if err != nil {
			return "", err
		}

		webHookUrl = util.UrlJoin(baseURL, "hook")
	} else {
		baseURL, err := services.GetServiceURLFromName(o.KubeClientCached, "jenkins", ns)
		if err != nil {
			return "", err
		}

		webHookUrl = util.UrlJoin(baseURL, "github-webhook/")
	}

	return webHookUrl, nil
}

func (o *CommonOptions) GetIn() terminal.FileReader {
	return o.In
}

func (o *CommonOptions) GetOut() terminal.FileWriter {
	return o.Out
}

func (o *CommonOptions) GetErr() io.Writer {
	return o.Err
}
