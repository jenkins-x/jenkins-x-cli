package create

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/upgrade"

	"github.com/jenkins-x/jx/pkg/vault/create"

	createoptions "github.com/jenkins-x/jx/pkg/cmd/create/options"
	cmdvault "github.com/jenkins-x/jx/pkg/cmd/create/vault"

	"github.com/jenkins-x/jx/pkg/packages"

	"github.com/jenkins-x/jx/pkg/prow"

	"github.com/jenkins-x/jx/pkg/platform"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/cmd/edit"
	"github.com/jenkins-x/jx/pkg/cmd/initcmd"
	"github.com/jenkins-x/jx/pkg/kube/naming"

	"github.com/spf13/viper"

	"github.com/jenkins-x/jx/pkg/cmd/step/env"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	gkeStorage "github.com/jenkins-x/jx/pkg/cloud/gke/storage"
	"github.com/jenkins-x/jx/pkg/kube/cluster"

	"k8s.io/helm/pkg/chartutil"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	version2 "github.com/jenkins-x/jx/pkg/version"

	"github.com/ghodss/yaml"

	"github.com/Pallinder/go-randomdata"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	kubevault "github.com/jenkins-x/jx/pkg/kube/vault"
	"github.com/jenkins-x/jx/pkg/vault"

	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"

	"github.com/jenkins-x/jx/pkg/addon"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cloud/aks"
	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cloud/iks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/features"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	configio "github.com/jenkins-x/jx/pkg/io"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	pkgvault "github.com/jenkins-x/jx/pkg/vault"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1"
	"gopkg.in/src-d/go-git.v4"
	core_v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ModifySecretCallback a callback for modifying a Secret for a given name
type ModifySecretCallback func(string, func(*core_v1.Secret) error) (*core_v1.Secret, error)

// ModifyConfigMapCallback a callback for modifying a ConfigMap for a given name
type ModifyConfigMapCallback func(string, func(*core_v1.ConfigMap) error) (*core_v1.ConfigMap, error)

// InstallOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type InstallOptions struct {
	*opts.CommonOptions
	gits.GitRepositoryOptions
	CreateJenkinsUserOptions
	CreateEnvOptions
	config.AdminSecretsService
	kubevault.AWSConfig

	InitOptions initcmd.InitOptions
	Flags       InstallFlags `mapstructure:"install"`

	modifyConfigMapCallback ModifyConfigMapCallback
	modifySecretCallback    ModifySecretCallback

	installValues map[string]string
}

// InstallFlags flags for the install command
type InstallFlags struct {
	ConfigFile                  string
	InstallOnly                 bool
	Domain                      string
	ExposeControllerURLTemplate string
	ExposeControllerPathMode    string
	AzureRegistrySubscription   string
	DockerRegistry              string
	DockerRegistryOrg           string
	Provider                    string
	VersionsRepository          string
	VersionsGitRef              string
	Version                     string
	LocalHelmRepoName           string
	Namespace                   string
	CloudEnvRepository          string
	NoDefaultEnvironments       bool
	RemoteEnvironments          bool
	DefaultEnvironmentPrefix    string
	LocalCloudEnvironment       bool
	EnvironmentGitOwner         string
	Timeout                     string
	HelmTLS                     bool
	RegisterLocalHelmRepo       bool
	CleanupTempFiles            bool
	Prow                        bool
	DisableSetKubeContext       bool
	Dir                         string
	Vault                       bool
	RecreateVaultBucket         bool
	Tekton                      bool
	BuildPackName               string
	Kaniko                      bool
	GitOpsMode                  bool
	NoGitOpsEnvApply            bool
	NoGitOpsEnvRepo             bool
	NoGitOpsEnvSetup            bool
	NoGitOpsVault               bool
	NextGeneration              bool `mapstructure:"next-generation"`
	StaticJenkins               bool
	LongTermStorage             bool   `mapstructure:"long-term-storage"`
	LongTermStorageBucketName   string `mapstructure:"lts-bucket"`
}

// Secrets struct for secrets
type Secrets struct {
	Login string
	Token string
}

const (
	JX_GIT_TOKEN = "JX_GIT_TOKEN" // #nosec
	JX_GIT_USER  = "JX_GIT_USER"

	ServerlessJenkins   = "Serverless Jenkins X Pipelines with Tekton"
	StaticMasterJenkins = "Static Jenkins Server and Jenkinsfiles"

	GitOpsChartYAML = `name: env
version: 0.0.1
description: GitOps Environment for this Environment
maintainers:
  - name: Team
icon: https://www.cloudbees.com/sites/default/files/Jenkins_8.png
`

	devGitOpsGitIgnore = `
# lets not accidentally check in Secret YAMLs!
secrets.yaml
mysecrets.yaml
`

	devGitOpsReadMe = `
## Jenkins X Development Environment

This repository contains the source code for the Jenkins X Development Environment so that it can be managed via GitOps.
`

	devGitOpsJenkinsfile = `pipeline {
  agent {
    label "jenkins-jx-base"
  }
  environment {
    DEPLOY_NAMESPACE = "%s"
  }
  stages {
    stage('Validate Environment') {
      steps {
        container('jx-base') {
          dir('env') {
            sh 'jx step helm build'
          }
        }
      }
    }
    stage('Update Environment') {
      when {
        branch 'master'
      }
      steps {
        container('jx-base') {
          dir('env') {
            sh 'jx step env apply'
          }
        }
      }
    }
  }
}
`

	devGitOpsJenkinsfileProw = `pipeline {
  agent any
  environment {
    DEPLOY_NAMESPACE = "%s"
  }
  stages {
    stage('Validate Environment') {
      steps {
        dir('env') {
          sh 'jx step helm build'
        }
      }
    }
    stage('Update Environment') {
      when {
        branch 'master'
      }
      steps {
        dir('env') {
          sh 'jx step env apply'
        }
      }
    }
  }
}
`
	longTermStorageFlagName = "long-term-storage"
	ltsBucketFlagName       = "lts-bucket"
	kanikoFlagName          = "kaniko"
	namespaceFlagName       = "namespace"
	tektonFlagName          = "tekton"
	prowFlagName            = "prow"
	staticJenkinsFlagName   = "static-jenkins"
	gitOpsFlagName          = "gitops"
)

var (
	InstalLong = templates.LongDesc(`
		Installs the Jenkins X platform on a Kubernetes cluster

		Requires a --git-username and --git-api-token that can be used to create a new token.
		This is so the Jenkins X platform can git tag your releases

		For more documentation see: [https://jenkins-x.io/getting-started/install-on-cluster/](https://jenkins-x.io/getting-started/install-on-cluster/)

		The current requirements are:

		*RBAC is enabled on the cluster

		*Insecure Docker registry is enabled for Docker registries running locally inside Kubernetes on the service IP range. See the above documentation for more detail

`)

	InstalExample = templates.Examples(`
		# Default installer which uses interactive prompts to generate git secrets
		jx install

		# Install with a GitHub personal access token
		jx install --git-username jenkins-x-bot --git-api-token 9fdbd2d070cd81eb12bca87861bcd850

		# If you know the cloud provider you can pass this as a CLI argument. E.g. for AWS
		jx install --provider=aws
`)
)

// NewCmdInstall creates a command object for the generic "install" action, which
// installs the jenkins-x platform on a Kubernetes cluster.
func NewCmdInstall(commonOpts *opts.CommonOptions) *cobra.Command {

	options := CreateInstallOptions(commonOpts)

	cmd := &cobra.Command{
		Use:     "install [flags]",
		Short:   "Install Jenkins X in the current Kubernetes cluster",
		Long:    InstalLong,
		Example: InstalExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddInstallFlags(cmd, false)

	cmd.Flags().StringVarP(&options.Flags.Provider, "provider", "", "", "Cloud service providing the Kubernetes cluster.  Supported providers: "+cloud.KubernetesProviderOptions())

	cmd.AddCommand(NewCmdInstallDependencies(commonOpts))
	cmdvault.AwsCreateVaultOptions(cmd, &options.AWSConfig)

	return cmd
}

// CreateInstallOptions creates the options for jx install
func CreateInstallOptions(commonOpts *opts.CommonOptions) InstallOptions {
	commonOptsBatch := *commonOpts
	commonOptsBatch.BatchMode = true
	options := InstallOptions{
		CreateJenkinsUserOptions: CreateJenkinsUserOptions{
			Username: "admin",
			CreateOptions: createoptions.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
		GitRepositoryOptions: gits.GitRepositoryOptions{},
		CommonOptions:        commonOpts,
		CreateEnvOptions: CreateEnvOptions{
			HelmValuesConfig: config.HelmValuesConfig{
				ExposeController: &config.ExposeController{
					Config: config.ExposeControllerConfig{
						HTTP:    "true",
						TLSAcme: "false",
						Exposer: "Ingress",
					},
				},
			},
			Options: v1.Environment{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.EnvironmentSpec{
					PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
				},
			},
			PromotionStrategy:      string(v1.PromotionStrategyTypeAutomatic),
			ForkEnvironmentGitRepo: kube.DefaultEnvironmentGitRepoURL,
			CreateOptions: createoptions.CreateOptions{
				CommonOptions: &commonOptsBatch,
			},
		},
		InitOptions: initcmd.InitOptions{
			CommonOptions: commonOpts,
			Flags:         initcmd.InitFlags{},
		},
		AdminSecretsService: config.AdminSecretsService{},
	}
	return options
}

func (options *InstallOptions) AddInstallFlags(cmd *cobra.Command, includesInit bool) {
	flags := &options.Flags
	flags.AddCloudEnvOptions(cmd)
	cmd.Flags().StringVarP(&flags.LocalHelmRepoName, "local-helm-repo-name", "", kube.LocalHelmRepoName, "The name of the helm repository for the installed ChartMuseum")
	cmd.Flags().BoolVarP(&flags.NoDefaultEnvironments, "no-default-environments", "", false, "Disables the creation of the default Staging and Production environments")
	cmd.Flags().BoolVarP(&flags.RemoteEnvironments, "remote-environments", "", false, "Indicates you intend Staging and Production environments to run in remote clusters. See https://jenkins-x.io/getting-started/multi-cluster/")
	cmd.Flags().StringVarP(&flags.DefaultEnvironmentPrefix, "default-environment-prefix", "", "", "Default environment repo prefix, your Git repos will be of the form 'environment-$prefix-$envName'")
	cmd.Flags().StringVarP(&flags.Namespace, namespaceFlagName, "", "jx", "The namespace the Jenkins X platform should be installed into")
	cmd.Flags().StringVarP(&flags.Timeout, "timeout", "", opts.DefaultInstallTimeout, "The number of seconds to wait for the helm install to complete")
	cmd.Flags().StringVarP(&flags.EnvironmentGitOwner, "environment-git-owner", "", "", "The Git provider organisation to create the environment Git repositories in")
	cmd.Flags().BoolVarP(&flags.RegisterLocalHelmRepo, "register-local-helmrepo", "", false, "Registers the Jenkins X ChartMuseum registry with your helm client [default false]")
	cmd.Flags().BoolVarP(&flags.CleanupTempFiles, "cleanup-temp-files", "", true, "Cleans up any temporary values.yaml used by helm install [default true]")
	cmd.Flags().BoolVarP(&flags.HelmTLS, "helm-tls", "", false, "Whether to use TLS with helm")
	cmd.Flags().BoolVarP(&flags.InstallOnly, "install-only", "", false, "Force the install command to fail if there is already an installation. Otherwise lets update the installation")
	cmd.Flags().StringVarP(&flags.AzureRegistrySubscription, "azure-acr-subscription", "", "", "The Azure subscription under which the specified docker-registry is located")
	cmd.Flags().StringVarP(&flags.DockerRegistry, "docker-registry", "", "", "The Docker Registry host or host:port which is used when tagging and pushing images. If not specified it defaults to the internal registry unless there is a better provider default (e.g. ECR on AWS/EKS)")
	cmd.Flags().StringVarP(&flags.DockerRegistryOrg, "docker-registry-org", "", "", "The Docker Registry organiation/user to create images inside. On GCP this is typically your Google Project ID.")
	cmd.Flags().StringVarP(&flags.ExposeControllerURLTemplate, "exposecontroller-urltemplate", "", "", "The ExposeController urltemplate for how services should be exposed as URLs. Defaults to being empty, which in turn defaults to \"{{.Service}}.{{.Namespace}}.{{.Domain}}\".")
	cmd.Flags().StringVarP(&flags.ExposeControllerPathMode, "exposecontroller-pathmode", "", "", "The ExposeController path mode for how services should be exposed as URLs. Defaults to using subnets. Use a value of `path` to use relative paths within the domain host such as when using AWS ELB host names")

	cmd.Flags().StringVarP(&flags.Version, "version", "", "", "The specific platform version to install")
	cmd.Flags().BoolVarP(&flags.Prow, prowFlagName, "", false, "Enable Prow to implement Serverless Jenkins and support ChatOps on Pull Requests")
	cmd.Flags().BoolVarP(&flags.Tekton, tektonFlagName, "", false, "Enables the Tekton pipeline engine (which used to be called knative build pipeline) along with Prow to provide Serverless Jenkins. Otherwise we default to use Knative Build if you enable Prow")
	cmd.Flags().BoolVarP(&flags.GitOpsMode, gitOpsFlagName, "", false, "Creates a git repository for the Dev environment to manage the installation, configuration, upgrade and addition of Apps in Jenkins X all via GitOps")
	cmd.Flags().BoolVarP(&flags.NoGitOpsEnvApply, "no-gitops-env-apply", "", false, "When using GitOps to create the source code for the development environment and installation, don't run 'jx step env apply' to perform the install")
	cmd.Flags().BoolVarP(&flags.NoGitOpsEnvRepo, "no-gitops-env-repo", "", false, "When using GitOps to create the source code for the development environment this flag disables the creation of a git repository for the source code")
	cmd.Flags().BoolVarP(&flags.NoGitOpsVault, "no-gitops-vault", "", false, "When using GitOps to create the source code for the development environment this flag disables the creation of a vault")
	cmd.Flags().BoolVarP(&flags.NoGitOpsEnvSetup, "no-gitops-env-setup", "", false, "When using GitOps to install the development environment this flag skips the post-install setup")
	cmd.Flags().BoolVarP(&flags.Vault, "vault", "", false, "Sets up a Hashicorp Vault for storing secrets during installation (supported only for GKE)")
	cmd.Flags().BoolVarP(&flags.RecreateVaultBucket, "vault-bucket-recreate", "", true, "If the vault bucket already exists delete it then create it empty")
	cmd.Flags().StringVarP(&flags.BuildPackName, "buildpack", "", "", "The name of the build pack to use for the Team")
	cmd.Flags().BoolVarP(&flags.Kaniko, kanikoFlagName, "", false, "Use Kaniko for building docker images")
	cmd.Flags().BoolVarP(&flags.NextGeneration, "ng", "", false, "Use the Next Generation Jenkins X features like Prow, Tekton, No Tiller, Vault, Dev GitOps")
	cmd.Flags().BoolVarP(&flags.StaticJenkins, staticJenkinsFlagName, "", false, "Install a static Jenkins master to use as the pipeline engine. Note this functionality is deprecated in favour of running serverless Tekton builds")
	cmd.Flags().BoolVarP(&flags.LongTermStorage, longTermStorageFlagName, "", false, "Enable the Long Term Storage option to save logs and other assets into a GCS bucket (supported only for GKE)")
	cmd.Flags().StringVarP(&flags.LongTermStorageBucketName, ltsBucketFlagName, "", "", "The bucket to use for Long Term Storage. If the bucket doesn't exist, an attempt will be made to create it, otherwise random naming will be used")
	cmd.Flags().StringVar(&options.ConfigFile, "config-file", "", "Configuration file used for installation")
	cmd.Flags().BoolVar(&options.NoBrew, opts.OptionNoBrew, false, "Disables brew package manager on MacOS when installing binary dependencies")
	cmd.Flags().BoolVar(&options.InstallDependencies, opts.OptionInstallDeps, false, "Enables automatic dependencies installation when required")
	cmd.Flags().BoolVar(&options.SkipAuthSecretsMerge, opts.OptionSkipAuthSecMerge, false, "Skips merging the secrets from local files with the secrets from Kubernetes cluster")

	bindInstallConfigToFlags(cmd)
	opts.AddGitRepoOptionsArgumentsWithGithubDefault(cmd, &options.GitRepositoryOptions)
	options.HelmValuesConfig.AddExposeControllerValues(cmd, true)
	options.AdminSecretsService.AddAdminSecretsValues(cmd)
	options.InitOptions.AddInitFlags(cmd)
}

func bindInstallConfigToFlags(cmd *cobra.Command) {
	_ = viper.BindPFlag(installConfigKey(namespaceFlagName), cmd.Flags().Lookup(namespaceFlagName))
	_ = viper.BindPFlag(installConfigKey(kanikoFlagName), cmd.Flags().Lookup(kanikoFlagName))
	_ = viper.BindPFlag(installConfigKey(tektonFlagName), cmd.Flags().Lookup(tektonFlagName))
	_ = viper.BindPFlag(installConfigKey(prowFlagName), cmd.Flags().Lookup(prowFlagName))
	_ = viper.BindPFlag(installConfigKey(gitOpsFlagName), cmd.Flags().Lookup(gitOpsFlagName))
	_ = viper.BindPFlag(installConfigKey("next-generation"), cmd.Flags().Lookup("ng"))
	_ = viper.BindPFlag(installConfigKey(staticJenkinsFlagName), cmd.Flags().Lookup(staticJenkinsFlagName))
}

func (flags *InstallFlags) AddCloudEnvOptions(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flags.CloudEnvRepository, "cloud-environment-repo", "", opts.DefaultCloudEnvironmentsURL, "Cloud Environments Git repo")
	cmd.Flags().StringVarP(&flags.VersionsRepository, "versions-repo", "", config.DefaultVersionsURL, "Jenkins X versions Git repo")
	cmd.Flags().StringVarP(&flags.VersionsGitRef, "versions-ref", "", "", "Jenkins X versions Git repository reference (tag, branch, sha etc)")
	cmd.Flags().BoolVarP(&flags.LocalCloudEnvironment, "local-cloud-environment", "", false, "Ignores default cloud-environment-repo and uses current directory ")
}

// CheckFlags validates & configures install flags
func (options *InstallOptions) CheckFlags() error {
	log.Logger().Debug("checking installation flags")
	flags := &options.Flags

	if flags.NextGeneration && flags.StaticJenkins {
		return fmt.Errorf("incompatible options '--ng' and '--static-jenkins'. Please pick only one of them. We recommend --ng as --static-jenkins is deprecated")
	}

	if flags.Tekton && flags.StaticJenkins {
		return fmt.Errorf("incompatible options '--tekton' and '--static-jenkins'. Please pick only one of them. We recommend --tekton as --static-jenkins is deprecated")
	}

	if flags.Prow {
		flags.StaticJenkins = false
		flags.Tekton = true
	}

	if flags.Tekton {
		flags.Prow = true
		if !options.InitOptions.Flags.NoTiller {
			log.Logger().Warnf("note that if using Serverless Jenkins with Tekton we recommend the extra flag: %s", util.ColorInfo("--no-tiller"))
		}
	}

	if flags.NextGeneration {
		flags.StaticJenkins = false
		flags.GitOpsMode = true
		flags.Vault = true
		flags.Prow = true
		flags.Tekton = true
		flags.Kaniko = true
		options.InitOptions.Flags.NoTiller = true
	}

	if options.BatchMode && !flags.NextGeneration && !flags.Prow && !flags.Tekton {
		flags.StaticJenkins = true
	}

	// only kaniko is supported as a builder in tekton
	if flags.Tekton {
		if flags.Provider == cloud.GKE {
			if !flags.Kaniko {
				log.Logger().Warnf("When using tekton, only kaniko is supported as a builder")
			}
			flags.Kaniko = true
		}
	}

	// check some flags combination for GitOps mode
	if flags.GitOpsMode {
		options.SkipAuthSecretsMerge = true
		flags.DisableSetKubeContext = true
		if !flags.Vault {
			log.Logger().Warnf("GitOps mode requires %s.", util.ColorInfo("vault"))
		}
		initFlags := &options.InitOptions.Flags
		if !initFlags.NoTiller {
			log.Logger().Warnf("GitOps mode requires helm without tiller server. %s flag is automatically set", util.ColorInfo("no-tiller"))
			initFlags.NoTiller = true
		}
	}

	// If we're using external-dns then remove the namespace subdomain from the URLTemplate
	if options.InitOptions.Flags.ExternalDNS {
		flags.ExposeControllerURLTemplate = `"{{.Service}}-{{.Namespace}}.{{.Domain}}"`
	}

	// Make sure that the default environment prefix is configured. Typically it is the cluster
	// name when the install command is called from create cluster.
	if flags.DefaultEnvironmentPrefix == "" {
		clusterName := options.installValues[kube.ClusterName]
		if clusterName == "" {
			flags.DefaultEnvironmentPrefix = strings.ToLower(randomdata.SillyName())
		} else {
			flags.DefaultEnvironmentPrefix = clusterName
		}
	}

	if flags.DockerRegistry == "" {
		dockerReg, err := options.dockerRegistryValue()
		if err != nil {
			log.Logger().Warnf("unable to calculate docker registry values - %s", err)
		}
		flags.DockerRegistry = dockerReg
	}

	// lets default the docker registry org to the project id
	if flags.DockerRegistryOrg == "" {
		flags.DockerRegistryOrg = options.installValues[kube.ProjectID]
	}

	log.Logger().Debugf("flags after checking - %+v", flags)

	if flags.Tekton {
		kind := options.GitRepositoryOptions.ServerKind
		if kind != "" && kind != gits.KindGitHub {
			return fmt.Errorf("Git provider: %s is not yet supported for Tekton.\nYou can work around this with '--static-jenkins'\nFor more details see: https://jenkins-x.io/about/status/", kind)
		}
	}
	return nil
}

// CheckFeatures - determines if the various features have been enabled
func (options *InstallOptions) CheckFeatures() error {
	if options.Flags.Tekton {
		return features.CheckTektonEnabled()
	}
	return nil
}

// Run implements this command
func (options *InstallOptions) Run() error {

	err := options.GetConfiguration(&options)
	if err != nil {
		return errors.Wrap(err, "getting install configuration")
	}

	err = options.selectJenkinsInstallation()
	if err != nil {
		return errors.Wrap(err, "selecting the Jenkins installation type")
	}

	// Check the provided flags before starting any installation
	err = options.CheckFlags()
	if err != nil {
		return errors.Wrap(err, "checking the provided flags")
	}

	configStore := configio.NewFileStore()

	ns, originalNs, err := options.setupNamespace()
	if err != nil {
		return errors.Wrap(err, "setting up current namespace")
	}
	client, err := options.KubeClient()
	if err != nil {
		return errors.Wrap(err, "creating the kube client")
	}

	err = options.registerAllCRDs()
	if err != nil {
		return errors.Wrap(err, "registering all CRDs")
	}

	gitOpsDir, gitOpsEnvDir, err := options.configureGitOpsMode(configStore, ns)
	if err != nil {
		return errors.Wrap(err, "configuring the GitOps mode")
	}

	options.configureHelm(client, ns)
	err = options.installHelmBinaries()
	if err != nil {
		return errors.Wrap(err, "installing helm binaries")
	}

	err = options.configureKubectl(ns)
	if err != nil {
		return errors.Wrap(err, "configure the kubectl")
	}

	err = options.installCloudProviderDependencies()
	if err != nil {
		return errors.Wrap(err, "installing cloud provider dependencies")
	}

	options.Flags.Provider, err = options.GetCloudProvider(options.Flags.Provider)
	if err != nil {
		return errors.Wrapf(err, "retrieving cloud provider '%s'", options.Flags.Provider)
	}

	err = options.configureTeamSettings()
	if err != nil {
		return errors.Wrap(err, "configuring the team settings in the dev environment")
	}

	err = options.configureCloudProviderPreInit(client)
	if err != nil {
		return errors.Wrap(err, "configuring the cloud provider before initializing the platform")
	}

	err = options.init()
	if err != nil {
		return errors.Wrap(err, "initializing the Jenkins X platform")
	}

	err = options.configureCloudProivderPostInit(client, ns)
	if err != nil {
		return errors.Wrap(err, "configuring the cloud provider after initializing the platform")
	}

	ic, err := options.saveIngressConfig()
	if err != nil {
		return errors.Wrap(err, "saving the ingress configuration in a ConfigMap")
	}

	err = options.configureLongTermStorageBucket()
	if err != nil {
		return errors.Wrap(err, "configuring Long Term Storage")
	}

	err = options.createSystemVault(client, ns, ic)
	if err != nil {
		return errors.Wrap(err, "creating the system vault")
	}

	err = options.saveClusterConfig()
	if err != nil {
		return errors.Wrap(err, "saving the cluster configuration in a ConfigMap")
	}

	err = options.configureGitAuth()
	if err != nil {
		return errors.Wrap(err, "configuring the git auth")
	}

	err = options.configureDockerRegistry(client, ns)
	if err != nil {
		return errors.Wrap(err, "configuring the docker registry")
	}

	versionsRepoDir, _, err := options.CloneJXVersionsRepo(options.Flags.VersionsRepository, options.Flags.VersionsGitRef)
	if err != nil {
		return errors.Wrap(err, "cloning the jx versions repo")
	}

	cloudEnvDir, err := options.CloneJXCloudEnvironmentsRepo()
	if err != nil {
		return errors.Wrap(err, "cloning the jx cloud environments repo")
	}

	err = options.ConfigureKaniko()
	if err != nil {
		return errors.Wrap(err, "unable to generate the Kaniko configuration")
	}

	err = options.configureHelmValues(ns)
	if err != nil {
		return errors.Wrap(err, "configuring helm values")
	}

	if options.Flags.Provider == "" {
		return fmt.Errorf("no Kubernetes provider found to match cloud-environment with")
	}
	providerEnvDir := filepath.Join(cloudEnvDir, fmt.Sprintf("env-%s", strings.ToLower(options.Flags.Provider)))
	valuesFiles, secretsFiles, temporaryFiles, err := options.getHelmValuesFiles(configStore, providerEnvDir)
	if err != nil {
		return errors.Wrap(err, "getting the helm value files")
	}

	log.Logger().Debugf("Installing Jenkins X platform helm chart from: %s", providerEnvDir)

	err = options.configureHelmRepo()
	if err != nil {
		return errors.Wrap(err, "configuring the Jenkins X helm repository")
	}

	err = options.configureProwInTeamSettings()
	if err != nil {
		return errors.Wrap(err, "configuring Prow in team settings")
	}

	err = options.configureAndInstallProw(ns, gitOpsEnvDir, valuesFiles)
	if err != nil {
		return errors.Wrap(err, "configuring and installing Prow")
	}

	err = options.verifyTiller(client, ns)
	if err != nil {
		return errors.Wrap(err, "verifying Tiller is running")
	}

	err = options.configureBuildPackMode()
	if err != nil {
		return errors.Wrap(err, "configuring the build pack mode")
	}

	log.Logger().Infof("Installing jx into namespace %s", util.ColorInfo(ns))

	version, err := options.getPlatformVersion(versionsRepoDir, configStore)
	if err != nil {
		return errors.Wrap(err, "getting the platform version")
	}

	log.Logger().Infof("Installing jenkins-x-platform version: %s", util.ColorInfo(version))

	if options.Flags.GitOpsMode {
		err := options.installPlatformGitOpsMode(gitOpsEnvDir, gitOpsDir, configStore, kube.DefaultChartMuseumURL,
			platform.JenkinsXPlatformChartName, ns, version, valuesFiles, secretsFiles)
		if err != nil {
			return errors.Wrap(err, "installing the Jenkins X platform in GitOps mode")
		}
	} else {
		err := options.installPlatform(providerEnvDir, platform.JenkinsXPlatformChart, platform.JenkinsXPlatformRelease,
			ns, version, valuesFiles, secretsFiles)
		if err != nil {
			return errors.Wrap(err, "installing the Jenkins X platform")
		}
	}

	if options.Flags.CleanupTempFiles {
		err := options.cleanupTempFiles(temporaryFiles)
		if err != nil {
			return errors.Wrap(err, "cleaning up the temporary files")
		}
	}

	err = options.configureImportModeInTeamSettings()
	if err != nil {
		return errors.Wrap(err, "configuring ImportMode in team settings")
	}

	err = options.configureTillerInDevEnvironment()
	if err != nil {
		return errors.Wrap(err, "configuring Tiller in the dev environment")
	}

	err = options.configureHelm3(ns)
	if err != nil {
		return errors.Wrap(err, "configuring helm3")
	}

	err = options.installAddons()
	if err != nil {
		return errors.Wrap(err, "installing the Jenkins X Addons")
	}

	// Jenkins needs to be configured already here if running in non GitOps mode
	// in order to be able to create the environments
	if !options.Flags.GitOpsMode {
		err = options.configureJenkins(ns)
		if err != nil {
			return errors.Wrap(err, "configuring Jenkins")
		}
	}

	err = options.createEnvironments(ns)
	if err != nil {
		if strings.Contains(err.Error(), "com.atlassian.bitbucket.project.NoSuchProjectException") {
			log.Logger().Infof("\nProject %s cannot be found. If you are using BitBucket Server, please use "+
				"a project code instead of a project name (for example 'MYPR' instead of 'myproject'). ",
				util.ColorInfo(options.CreateEnvOptions.GitRepositoryOptions.Owner))
			return nil
		}
		return errors.Wrap(err, "creating the environments")
	}

	err = options.saveChartmuseumAuthConfig()
	if err != nil {
		return errors.Wrap(err, "saving the ChartMuseum auth configuration")
	}

	if options.Flags.RegisterLocalHelmRepo {
		err = options.RegisterLocalHelmRepo(options.Flags.LocalHelmRepoName, ns)
		if err != nil {
			return errors.Wrapf(err, "registering the local helm repo '%s'", options.Flags.LocalHelmRepoName)
		}
	}

	gitOpsEnvDir, err = options.generateGitOpsDevEnvironmentConfig(gitOpsDir)
	if err != nil {
		return errors.Wrap(err, "generating the GitOps development environment config")
	}

	err = options.applyGitOpsDevEnvironmentConfig(gitOpsEnvDir, ns)
	if err != nil {
		return errors.Wrap(err, "applying the GitOps development environment config")
	}

	err = options.setupGitOpsPostApply(ns)
	if err != nil {
		return errors.Wrap(err, "setting up GitOps post installation")
	}

	log.Logger().Infof("\nJenkins X installation completed successfully")

	options.logAdminPassword()

	options.logNameServers()

	log.Logger().Infof("Your Kubernetes context is now set to the namespace: %s ", util.ColorInfo(ns))
	log.Logger().Infof("To switch back to your original namespace use: %s", util.ColorInfo("jx namespace "+originalNs))
	log.Logger().Infof("Or to use this context/namespace in just one terminal use: %s", util.ColorInfo("jx shell"))
	log.Logger().Infof("For help on switching contexts see: %s\n", util.ColorInfo("https://jenkins-x.io/developing/kube-context/"))

	log.Logger().Infof("To import existing projects into Jenkins X:       %s", util.ColorInfo("jx import"))
	log.Logger().Infof("To create a new Spring Boot microservice:       %s", util.ColorInfo("jx create spring -d web -d actuator"))
	log.Logger().Infof("To create a new microservice from a quickstart: %s", util.ColorInfo("jx create quickstart"))
	return nil
}

func (options *InstallOptions) configureKubectl(namespace string) error {
	if !options.Flags.DisableSetKubeContext {
		context, err := options.GetCommandOutput("", "kubectl", "config", "current-context")
		if err != nil {
			return errors.Wrap(err, "failed to retrieve the current context from kube configuration")
		}
		err = options.RunCommand("kubectl", "config", "set-context", context, "--namespace", namespace)
		if err != nil {
			return errors.Wrapf(err, "failed to set the context '%s' in kube configuration", context)
		}
	}

	return nil
}

func (options *InstallOptions) setupNamespace() (string, string, error) {
	_, originalNs, err := options.KubeClientAndNamespace()
	if err != nil {
		return "", "", errors.Wrap(err, "creating kube client")
	}
	ns := options.Flags.Namespace
	if ns == "" {
		ns = originalNs
	}
	options.SetDevNamespace(ns)

	return ns, originalNs, nil
}

func (options *InstallOptions) init() error {
	initOpts := &options.InitOptions
	initOpts.Flags.Provider = options.Flags.Provider
	initOpts.Flags.Namespace = options.Flags.Namespace
	initOpts.BatchMode = options.BatchMode
	initOpts.Flags.VersionsRepository = options.Flags.VersionsRepository
	initOpts.Flags.Http = true
	exposeController := options.CreateEnvOptions.HelmValuesConfig.ExposeController
	if exposeController != nil {
		initOpts.Flags.Http = exposeController.Config.HTTP == "true"
	}
	if initOpts.Flags.Domain == "" && options.Flags.Domain != "" {
		initOpts.Flags.Domain = options.Flags.Domain
	}
	if initOpts.Flags.NoTiller {
		initOpts.SetHelm(nil)
	}
	// configure local tiller if this is required
	if !initOpts.Flags.RemoteTiller && !initOpts.Flags.NoTiller {
		err := helm.RestartLocalTiller()
		if err != nil {
			return errors.Wrap(err, "restarting local tiller")
		}
		initOpts.SetHelm(options.Helm())
	}

	// configure the helm values for expose controller
	if exposeController != nil {
		ecConfig := &exposeController.Config
		if ecConfig.Domain == "" && options.Flags.Domain != "" {
			ecConfig.Domain = options.Flags.Domain
			log.Logger().Infof("set exposeController Config Domain %s", ecConfig.Domain)
		}
		if ecConfig.PathMode == "" && options.Flags.ExposeControllerPathMode != "" {
			ecConfig.PathMode = options.Flags.ExposeControllerPathMode
			log.Logger().Infof("set exposeController Config PathMode %s", ecConfig.PathMode)
		}
		if (ecConfig.URLTemplate == "" && options.Flags.ExposeControllerURLTemplate != "") ||
			(options.Flags.ExposeControllerURLTemplate != "" && options.InitOptions.Flags.ExternalDNS) {
			ecConfig.URLTemplate = options.Flags.ExposeControllerURLTemplate
			log.Logger().Infof("set exposeController Config URLTemplate %s", ecConfig.URLTemplate)
		}
		if isOpenShiftProvider(options.Flags.Provider) {
			ecConfig.Exposer = "Route"
		}
	}

	err := initOpts.Run()
	if err != nil {
		return errors.Wrap(err, "initializing the Jenkins X platform")
	}

	// update the domain if was modified during the initialization
	domain := exposeController.Config.Domain
	if domain == "" {
		domain = initOpts.Flags.Domain
	}
	if domain == "" {
		client, err := options.KubeClient()
		if err != nil {
			return errors.Wrap(err, "getting the kubernetes client")
		}
		ingNamespace := initOpts.Flags.IngressNamespace
		ingService := initOpts.Flags.IngressService
		extIP := initOpts.Flags.ExternalIP
		domain, err = options.GetDomain(client, domain,
			options.Flags.Provider,
			ingNamespace,
			ingService,
			extIP)
		if err != nil {
			return errors.Wrapf(err, "getting a domain for ingress service %s/%s", ingNamespace, ingService)
		}
	}

	// checking if the domain is by any chance empty and bail out
	if domain == "" {
		return fmt.Errorf("the installation cannot proceed with an empty domain. Please provide a domain in the %s option",
			util.ColorInfo("domain"))
	}

	options.Flags.Domain = domain
	exposeController.Config.Domain = domain

	return nil
}

func (options *InstallOptions) getPlatformVersion(cloudEnvDir string,
	configStore configio.ConfigStore) (string, error) {
	version := options.Flags.Version
	var err error
	if version == "" {
		version, err = LoadVersionFromCloudEnvironmentsDir(cloudEnvDir, configStore)
		if err != nil {
			return "", errors.Wrap(err, "failed to load version from cloud environments dir")
		}
	}
	return version, nil
}

func (options *InstallOptions) installPlatform(providerEnvDir string, jxChart string, jxRelName string,
	namespace string, version string, valuesFiles []string, secretsFiles []string) error {

	options.Helm().SetCWD(providerEnvDir)

	timeout := options.Flags.Timeout
	if timeout == "" {
		timeout = opts.DefaultInstallTimeout
	}

	allValuesFiles := []string{}
	allValuesFiles = append(allValuesFiles, valuesFiles...)
	allValuesFiles = append(allValuesFiles, secretsFiles...)
	for _, f := range allValuesFiles {
		log.Logger().Debugf("Adding values file %s", util.ColorInfo(f))
	}

	helmOpts := helm.InstallChartOptions{
		ReleaseName: jxRelName,
		Chart:       jxChart,
		Ns:          namespace,
		Version:     version,
		ValueFiles:  allValuesFiles,
		InstallOnly: options.Flags.InstallOnly,
		NoForce:     true,
	}
	err := options.InstallChartWithOptionsAndTimeout(helmOpts, timeout)

	if err != nil {
		return errors.Wrap(err, "failed to install/upgrade the jenkins-x platform chart")
	}

	err = options.waitForInstallToBeReady(namespace)
	if err != nil {
		return errors.Wrap(err, "failed to wait for jenkins-x chart installation to be ready")
	}
	log.Logger().Infof("Jenkins X deployments ready in namespace %s", namespace)
	return nil
}

func (options *InstallOptions) installPlatformGitOpsMode(gitOpsEnvDir string, gitOpsDir string, configStore configio.ConfigStore,
	chartRepository string, chartName string, namespace string, version string, valuesFiles []string, secretsFiles []string) error {
	options.CreateEnvOptions.NoDevNamespaceInit = true

	chartFile := filepath.Join(gitOpsEnvDir, helm.ChartFileName)
	requirementsFile := filepath.Join(gitOpsEnvDir, helm.RequirementsFileName)
	secretsFile := filepath.Join(gitOpsEnvDir, helm.SecretsFileName)
	valuesFile := filepath.Join(gitOpsEnvDir, helm.ValuesFileName)

	platformDep := &helm.Dependency{
		Name:       platform.JenkinsXPlatformChartName,
		Version:    version,
		Repository: kube.DefaultChartMuseumURL,
	}
	requirements := &helm.Requirements{
		Dependencies: []*helm.Dependency{platformDep},
	}

	// lets handle if the requirements.yaml already exists we may have added some initial apps like prow etc
	exists, err := util.FileExists(requirementsFile)
	if err != nil {
		return err
	}
	if exists {
		requirements, err = helm.LoadRequirementsFile(requirementsFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load helm requirements file %s", requirementsFile)
		}
		requirements.Dependencies = append(requirements.Dependencies, platformDep)
	}
	err = helm.SaveFile(requirementsFile, requirements)
	if err != nil {
		return errors.Wrapf(err, "failed to save GitOps helm requirements file %s", requirementsFile)
	}

	err = configStore.Write(chartFile, []byte(GitOpsChartYAML))
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", chartFile)
	}

	err = helm.CombineValueFilesToFile(secretsFile, secretsFiles, platform.JenkinsXPlatformChartName, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to generate %s by combining helm Secret YAML files %s", secretsFile, strings.Join(secretsFiles, ", "))
	}

	if options.Flags.Vault {
		err := options.storeSecretYamlFilesInVault(vault.GitOpsSecretsPath, secretsFile)
		if err != nil {
			return errors.Wrapf(err, "storing in Vault the secrets files: %s", secretsFile)
		}

		err = util.DestroyFile(secretsFile)
		if err != nil {
			return errors.Wrapf(err, "destroying the secrets file '%s' after storing it in Vault", secretsFile)
		}
	}

	extraValues := map[string]interface{}{
		"postinstalljob": map[string]interface{}{"enabled": "true"},
	}

	err = options.setValuesFileValue(filepath.Join(gitOpsEnvDir, "jenkins", helm.ValuesFileName), "enabled", !options.Flags.Prow)
	if err != nil {
		return err
	}
	err = options.setValuesFileValue(filepath.Join(gitOpsEnvDir, "controllerbuild", helm.ValuesFileName), "enabled", options.Flags.Prow)
	if err != nil {
		return err
	}
	err = options.setValuesFileValue(filepath.Join(gitOpsEnvDir, "controllerworkflow", helm.ValuesFileName), "enabled", !options.Flags.Tekton)
	if err != nil {
		return err
	}

	// lets load any existing values.yaml data as we may have created this via additional apps like Prow
	exists, err = util.FileExists(valuesFile)
	if err != nil {
		return err
	}
	if exists {
		currentValues, err := chartutil.ReadValuesFile(valuesFile)
		if err != nil {
			return err
		}
		util.CombineMapTrees(extraValues, currentValues)
	}

	err = helm.CombineValueFilesToFile(valuesFile, valuesFiles, platform.JenkinsXPlatformChartName, extraValues)
	if err != nil {
		return errors.Wrapf(err, "failed to generate %s by combining helm value YAML files %s", valuesFile, strings.Join(valuesFiles, ", "))
	}

	gitIgnore := filepath.Join(gitOpsDir, ".gitignore")
	err = configStore.Write(gitIgnore, []byte(devGitOpsGitIgnore))
	if err != nil {
		return errors.Wrapf(err, "failed to write %s", gitIgnore)
	}

	readme := filepath.Join(gitOpsDir, "README.md")
	err = configStore.Write(readme, []byte(devGitOpsReadMe))
	if err != nil {
		return errors.Wrapf(err, "failed to write %s", readme)
	}

	jenkinsFile := filepath.Join(gitOpsDir, "Jenkinsfile")
	jftTmp := devGitOpsJenkinsfile
	isProw := options.Flags.Prow
	if isProw {
		jftTmp = devGitOpsJenkinsfileProw
	}
	text := fmt.Sprintf(jftTmp, namespace)
	err = configStore.Write(jenkinsFile, []byte(text))
	if err != nil {
		return errors.Wrapf(err, "failed to write %s", jenkinsFile)
	}
	return nil
}

func (options *InstallOptions) configureAndInstallProw(namespace string, gitOpsEnvDir string, valuesFiles []string) error {
	options.SetCurrentNamespace(namespace)
	if options.Flags.Prow {
		_, pipelineUser, err := options.GetPipelineGitAuth()
		if err != nil || pipelineUser == nil {
			return errors.Wrap(err, "retrieving the pipeline Git Auth")
		}
		options.OAUTHToken = pipelineUser.ApiToken
		err = options.InstallProw(options.Flags.Tekton, options.InitOptions.Flags.ExternalDNS, options.Flags.GitOpsMode, gitOpsEnvDir, pipelineUser.Username, valuesFiles)
		if err != nil {
			return errors.Wrap(err, "installing Prow")
		}
	}
	return nil
}

func (options *InstallOptions) configureHelm3(namespace string) error {
	initOpts := &options.InitOptions
	helmBinary := initOpts.HelmBinary()
	if helmBinary != "helm" {
		helmOptions := edit.EditHelmBinOptions{}
		helmOptions.CommonOptions = options.CommonOptions
		helmOptions.CommonOptions.BatchMode = true
		helmOptions.CommonOptions.Args = []string{helmBinary}
		helmOptions.SetDevNamespace(namespace)
		err := helmOptions.Run()
		if err != nil {
			return errors.Wrap(err, "failed to edit the helm options")
		}
	}
	return nil
}

func (options *InstallOptions) configureHelm(client kubernetes.Interface, namespace string) {
	initOpts := &options.InitOptions
	helmBinary := initOpts.HelmBinary()
	options.Helm().SetHelmBinary(helmBinary)
	if initOpts.Flags.NoTiller {
		helmer := options.Helm()
		helmCli, ok := helmer.(*helm.HelmCLI)
		if ok && helmCli != nil {
			helm := helm.NewHelmTemplate(helmCli, helmCli.CWD, client, namespace)
			options.SetHelm(helm)
		} else {
			helmTemplate, ok := helmer.(*helm.HelmTemplate)
			if ok {
				options.SetHelm(helmTemplate)
			} else {
				log.Logger().Warnf("Helm facade is not a *helm.HelmCLI or *helm.HelmTemplate: %#v", helmer)
			}
		}
	}
}

func (options *InstallOptions) configureHelmRepo() error {
	_, err := options.AddHelmBinaryRepoIfMissing(kube.DefaultChartMuseumURL, "jenkins-x", "", "")
	if err != nil {
		return errors.Wrap(err, "failed to add the jenkinx-x helm repo")
	}

	err = options.Helm().UpdateRepo()
	if err != nil {
		return errors.Wrap(err, "failed to update the helm repo")
	}
	return nil
}

func (options *InstallOptions) selectJenkinsInstallation() error {
	if !options.BatchMode {
		//install type has not been configured
		if !options.Flags.Prow && !options.Flags.StaticJenkins {
			jenkinsInstallOptions := []string{
				ServerlessJenkins,
				StaticMasterJenkins,
			}
			jenkinsInstallOption, err := util.PickNameWithDefault(jenkinsInstallOptions, "Select Jenkins installation type:", ServerlessJenkins, "", options.GetIOFileHandles())
			if err != nil {
				return errors.Wrap(err, "picking Jenkins installation type")
			}
			if jenkinsInstallOption == ServerlessJenkins {
				options.Flags.Prow = true
				options.Flags.Tekton = true
			}
		} else {
			//determine which install type is configured
			jenkinsInstallOption := ServerlessJenkins
			if options.Flags.StaticJenkins {
				jenkinsInstallOption = StaticMasterJenkins
			}
			log.Logger().Infof(util.QuestionAnswer("Configured Jenkins installation type", jenkinsInstallOption))
		}
	}
	return nil
}

func (options *InstallOptions) configureTillerNamespace() error {
	helmConfig := &options.CreateEnvOptions.HelmValuesConfig
	initOpts := &options.InitOptions
	if initOpts.Flags.TillerNamespace != "" {
		if helmConfig.Jenkins.Servers.Global.EnvVars == nil {
			helmConfig.Jenkins.Servers.Global.EnvVars = map[string]string{}
		}
		helmConfig.Jenkins.Servers.Global.EnvVars["TILLER_NAMESPACE"] = initOpts.Flags.TillerNamespace
		os.Setenv("TILLER_NAMESPACE", initOpts.Flags.TillerNamespace)
	}
	return nil
}

func (options *InstallOptions) configureHelmValues(namespace string) error {
	helmConfig := &options.CreateEnvOptions.HelmValuesConfig

	domain := helmConfig.ExposeController.Config.Domain
	if domain != "" && addon.IsAddonEnabled("gitea") {
		helmConfig.Jenkins.Servers.GetOrCreateFirstGitea().Url = "http://gitea-gitea." + namespace + "." + domain
	}

	err := options.addGitServersToJenkinsConfig(helmConfig)
	if err != nil {
		return errors.Wrap(err, "configuring the Git Servers into Jenkins configuration")
	}

	err = options.configureTillerNamespace()
	if err != nil {
		return errors.Wrap(err, "configuring the tiller namespace")
	}

	if !options.Flags.GitOpsMode {
		options.SetDevNamespace(namespace)
	}

	isProw := options.Flags.Prow
	if isProw {
		enableJenkins := false
		helmConfig.Jenkins.Enabled = &enableJenkins
		helmConfig.ControllerBuild = &config.EnabledConfig{true}
		helmConfig.ControllerWorkflow = &config.EnabledConfig{false}
		if options.Flags.Tekton && options.Flags.Provider == cloud.GKE {
			helmConfig.DockerRegistryEnabled = &config.EnabledConfig{false}
		}
	}
	return nil
}

func (options *InstallOptions) getHelmValuesFiles(configStore configio.ConfigStore, providerEnvDir string) ([]string, []string, []string, error) {
	helmConfig := &options.CreateEnvOptions.HelmValuesConfig
	cloudEnvironmentValuesLocation := filepath.Join(providerEnvDir, opts.CloudEnvValuesFile)
	cloudEnvironmentSecretsLocation := filepath.Join(providerEnvDir, opts.CloudEnvSecretsFile)

	valuesFiles := []string{}
	secretsFiles := []string{}
	temporaryFiles := []string{}

	adminSecretsFileName, adminSecrets, err := options.getAdminSecrets(configStore,
		providerEnvDir, cloudEnvironmentSecretsLocation)
	if err != nil {
		return valuesFiles, secretsFiles, temporaryFiles,
			errors.Wrap(err, "creating the admin secrets")
	}

	dir, err := util.ConfigDir()
	if err != nil {
		return valuesFiles, secretsFiles, temporaryFiles,
			errors.Wrap(err, "creating a temporary config dir for Git credentials")
	}

	extraValuesFileName := filepath.Join(dir, opts.ExtraValuesFile)
	err = configStore.WriteObject(extraValuesFileName, helmConfig)
	if err != nil {
		return valuesFiles, secretsFiles, temporaryFiles,
			errors.Wrapf(err, "writing the helm config in the file '%s'", extraValuesFileName)
	}
	log.Logger().Debugf("Generated helm values %s", util.ColorInfo(extraValuesFileName))

	err = options.modifySecrets(helmConfig, adminSecrets)
	if err != nil {
		return valuesFiles, temporaryFiles, secretsFiles, errors.Wrap(err, "updating the secrets data in Kubernetes cluster")
	}

	valuesFiles = append(valuesFiles, cloudEnvironmentValuesLocation)
	valuesFiles, err = helm.AppendMyValues(valuesFiles)
	if err != nil {
		return valuesFiles, secretsFiles, temporaryFiles,
			errors.Wrap(err, "failed to append the myvalues.yaml file")
	}
	secretsFiles = append(secretsFiles,
		[]string{adminSecretsFileName, extraValuesFileName, cloudEnvironmentSecretsLocation}...)

	if options.Flags.Vault {
		temporaryFiles = append(temporaryFiles, adminSecretsFileName, extraValuesFileName, cloudEnvironmentSecretsLocation)
	} else {
		temporaryFiles = append(temporaryFiles, extraValuesFileName, cloudEnvironmentSecretsLocation)
	}

	return util.FilterFileExists(valuesFiles), util.FilterFileExists(secretsFiles), util.FilterFileExists(temporaryFiles), nil
}

func (options *InstallOptions) configureGitAuth() error {
	log.Logger().Infof("Set up a Git username and API token to be able to perform CI/CD")
	gitUsername := options.GitRepositoryOptions.Username
	gitServer := options.GitRepositoryOptions.ServerURL
	gitAPIToken := options.GitRepositoryOptions.ApiToken
	if gitUsername == "" {
		gitUsernameEnv := os.Getenv(JX_GIT_USER)
		if gitUsernameEnv != "" {
			gitUsername = gitUsernameEnv
		}
	}

	if gitAPIToken == "" {
		gitAPITokenEnv := os.Getenv(JX_GIT_TOKEN)
		if gitAPITokenEnv != "" {
			gitAPIToken = gitAPITokenEnv
		}
	}

	authConfigSvc, err := options.GitAuthConfigService()
	if err != nil {
		return errors.Wrap(err, "creating the git auth config service")
	}

	authConfig := authConfigSvc.Config()
	var userAuth *auth.UserAuth
	if gitUsername != "" && gitAPIToken != "" && gitServer != "" {
		userAuth = &auth.UserAuth{
			ApiToken: gitAPIToken,
			Username: gitUsername,
		}
		authConfig.SetUserAuth(gitServer, userAuth)
	}

	var authServer *auth.AuthServer
	if gitServer != "" {
		kind := ""
		if options.GitRepositoryOptions.ServerKind == "" {
			kind = gits.SaasGitKind(gitServer)
		} else {
			kind = options.GitRepositoryOptions.ServerKind
		}
		authServer = authConfig.GetOrCreateServerName(gitServer, "", kind)
	} else {
		authServer, err = authConfig.PickServer("Which Git provider:", options.BatchMode, options.GetIOFileHandles())
		if err != nil {
			return errors.Wrap(err, "getting the git provider from user")
		}
	}

	message := fmt.Sprintf("local Git user for %s server:", authServer.Label())
	userAuth, err = authConfig.PickServerUserAuth(authServer, message, options.BatchMode, "", options.GetIOFileHandles())
	if err != nil {
		return errors.Wrapf(err, "selecting the local user for git server %s", authServer.Label())
	}

	if userAuth.IsInvalid() {
		log.Logger().Infof("Creating a local Git user for %s server", authServer.Label())
		f := func(username string) error {
			options.Git().PrintCreateRepositoryGenerateAccessToken(authServer, username, options.Out)
			return nil
		}
		defaultUserName := ""
		err = authConfig.EditUserAuth(authServer.Label(), userAuth, defaultUserName, false, options.BatchMode, f,
			options.GetIOFileHandles())
		if err != nil {
			return errors.Wrapf(err, "creating a user authentication for git server %s", authServer.Label())
		}
		if userAuth.IsInvalid() {
			return fmt.Errorf("invalid user authentication for git server %s", authServer.Label())
		}
		authConfig.SetUserAuth(gitServer, userAuth)
	}

	log.Logger().Infof("Select the CI/CD pipelines Git server and user")
	var pipelineAuthServer *auth.AuthServer
	if options.BatchMode {
		pipelineAuthServer = authServer
	} else {
		surveyOpts := survey.WithStdio(options.In, options.Out, options.Err)
		confirm := &survey.Confirm{
			Message: fmt.Sprintf("Do you wish to use %s as the pipelines Git server:", authServer.Label()),
			Default: true,
		}
		yes := false
		err = survey.AskOne(confirm, &yes, nil, surveyOpts)
		if err != nil {
			return errors.Wrap(err, "selecting pipelines Git server")
		}
		if yes {
			pipelineAuthServer = authServer
		} else {
			pipelineAuthServerURL, err := util.PickValue("Git Service URL:", gits.GitHubURL, true, "",
				options.GetIOFileHandles())
			if err != nil {
				return errors.Wrap(err, "reading the pipelines Git service URL")
			}
			pipelineAuthServer, err = authConfig.PickOrCreateServer(gits.GitHubURL, pipelineAuthServerURL,
				"Which Git Service do you wish to use:",
				options.BatchMode, options.GetIOFileHandles())
			if err != nil {
				return errors.Wrap(err, "selecting the pipelines Git Service")
			}
		}
	}

	// lets default the values from the CLI arguments
	if options.GitRepositoryOptions.Username != "" {
		authConfig.PipeLineUsername = options.GitRepositoryOptions.Username
	}
	if options.GitRepositoryOptions.ServerURL != "" {
		authConfig.PipeLineServer = options.GitRepositoryOptions.ServerURL
	}
	pipelineUserAuth, err := options.PickPipelineUserAuth(authConfig, authServer)
	if err != nil {
		return errors.Wrapf(err, "selecting the pipeline user for git server %s", authServer.Label())
	}
	if pipelineUserAuth.IsInvalid() {
		log.Logger().Infof("Creating a pipelines Git user for %s server", authServer.Label())
		f := func(username string) error {
			options.Git().PrintCreateRepositoryGenerateAccessToken(pipelineAuthServer, username, options.Out)
			return nil
		}
		defaultUserName := ""
		err = authConfig.EditUserAuth(pipelineAuthServer.Label(), pipelineUserAuth, defaultUserName, false, options.BatchMode,
			f, options.GetIOFileHandles())
		if err != nil {
			return errors.Wrapf(err, "creating a pipeline user authentication for git server %s", authServer.Label())
		}
		if userAuth.IsInvalid() {
			return fmt.Errorf("invalid pipeline user authentication for git server %s", authServer.Label())
		}
		authConfig.SetUserAuth(pipelineAuthServer.URL, pipelineUserAuth)
	}

	pipelineAuthServerURL := pipelineAuthServer.URL
	pipelineAuthUsername := pipelineUserAuth.Username

	log.Logger().Infof("Setting the pipelines Git server %s and user name %s.",
		util.ColorInfo(pipelineAuthServerURL), util.ColorInfo(pipelineAuthUsername))
	authConfig.UpdatePipelineServer(pipelineAuthServer, pipelineUserAuth)

	log.Logger().Debugf("Saving the Git authentication configuration")
	err = authConfigSvc.SaveConfig()
	if err != nil {
		return errors.Wrap(err, "saving the Git authentication configuration")
	}

	editTeamSettingsCallback := func(env *v1.Environment) error {
		teamSettings := &env.Spec.TeamSettings
		teamSettings.GitServer = pipelineAuthServerURL
		teamSettings.PipelineUsername = pipelineAuthUsername
		teamSettings.Organisation = options.Owner
		teamSettings.GitPublic = options.GitRepositoryOptions.Public
		return nil
	}
	err = options.ModifyDevEnvironment(editTeamSettingsCallback)
	if err != nil {
		return errors.Wrap(err, "updating the team settings into the environment configuration")
	}

	return nil
}

func (options *InstallOptions) buildGitRepositoryOptionsForEnvironments() (*gits.GitRepositoryOptions, error) {
	authConfigSvc, err := options.GitAuthConfigService()
	if err != nil {
		return nil, errors.Wrap(err, "creating Git authentication config service")
	}
	config := authConfigSvc.Config()

	server := config.CurrentAuthServer()
	if server == nil {
		return nil, fmt.Errorf("no current git server set in the configuration")
	}
	user := config.CurrentUser(server, false)
	if user == nil {
		return nil, fmt.Errorf("no current git user set in configuration for server '%s'", server.Label())
	}

	org := options.Flags.EnvironmentGitOwner
	if org == "" {
		if options.BatchMode {
			jxClient, _, err := options.JXClientAndDevNamespace()
			if err != nil {
				return nil, errors.Wrap(err, "determining the git owner for environments")
			}
			org, _ = kube.GetDevEnvGitOwner(jxClient)
			if org == "" {
				org = user.Username
			}

			log.Logger().Infof("Using %s environment git owner in batch mode.", util.ColorInfo(org))
		} else {
			provider, err := gits.CreateProvider(server, user, options.Git())
			if err != nil {
				return nil, errors.Wrap(err, "creating the Git provider")
			}

			orgs := gits.GetOrganizations(provider, user.Username)
			if len(orgs) == 0 {
				return nil, fmt.Errorf("user '%s' has no organizations", user.Username)
			}

			surveyOpts := survey.WithStdio(options.In, options.Out, options.Err)
			sort.Strings(orgs)
			prompt := &survey.Select{
				Message: "Select the organization where you want to create the environment repository:",
				Options: orgs,
			}
			err = survey.AskOne(prompt, &org, survey.Required, surveyOpts)
			if err != nil {
				return nil, errors.Wrap(err, "selecting the organization for environment repository")
			}
		}
	}

	//Save selected organisation for Environment repos.
	err = options.ModifyDevEnvironment(func(env *v1.Environment) error {
		env.Spec.TeamSettings.EnvOrganisation = org
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "updating the TeamSettings with Environments organisation")
	}

	return &gits.GitRepositoryOptions{
		ServerURL: server.URL,
		Username:  user.Username,
		ApiToken:  user.ApiToken,
		Owner:     org,
		Public:    options.GitRepositoryOptions.Public,
	}, nil
}

func (options *InstallOptions) cleanupTempFiles(temporaryFiles []string) error {
	for _, tempFile := range temporaryFiles {
		exists, err := util.FileExists(tempFile)
		if exists && err == nil {
			err := util.DestroyFile(tempFile)
			if err != nil {
				return errors.Wrapf(err, "removing temporary file '%s'", tempFile)
			}
		}
	}
	return nil
}

func (options *InstallOptions) verifyTiller(client kubernetes.Interface, namespace string) error {
	initOpts := &options.InitOptions
	if !initOpts.Flags.NoTiller {
		serviceAccountName := "tiller"
		tillerNamespace := options.InitOptions.Flags.TillerNamespace

		log.Logger().Infof("Waiting for %s pod to be ready, service account name is %s, namespace is %s, tiller namespace is %s",
			util.ColorInfo("tiller"), util.ColorInfo(serviceAccountName), util.ColorInfo(namespace), util.ColorInfo(tillerNamespace))

		clusterRoleBindingName := serviceAccountName + "-role-binding"
		role := options.InitOptions.Flags.TillerClusterRole

		log.Logger().Infof("Waiting for cluster role binding to be defined, named %s in namespace %s", util.ColorInfo(clusterRoleBindingName), util.ColorInfo(namespace))
		err := options.EnsureClusterRoleBinding(clusterRoleBindingName, role, namespace, serviceAccountName)
		if err != nil {
			return errors.Wrap(err, "tiller cluster role not defined")
		}
		log.Logger().Infof("tiller cluster role defined: %s in namespace %s", util.ColorInfo(role), util.ColorInfo(namespace))

		err = kube.WaitForDeploymentToBeReady(client, "tiller-deploy", tillerNamespace, 10*time.Minute)
		if err != nil {
			msg := fmt.Sprintf("tiller pod (tiller-deploy in namespace %s) is not running after 10 minutes", tillerNamespace)
			return errors.Wrap(err, msg)
		}
		log.Logger().Info("tiller pod running")
	}
	return nil
}

func (options *InstallOptions) configureTillerInDevEnvironment() error {
	initOpts := &options.InitOptions
	if !initOpts.Flags.RemoteTiller && !initOpts.Flags.NoTiller {
		callback := func(env *v1.Environment) error {
			env.Spec.TeamSettings.NoTiller = true
			log.Logger().Info("Disabling the server side use of tiller in the TeamSettings")
			return nil
		}
		err := options.ModifyDevEnvironment(callback)
		if err != nil {
			return err
		}
	}
	return nil
}

func (options *InstallOptions) configureProwInTeamSettings() error {
	if options.Flags.Prow {
		callback := func(env *v1.Environment) error {
			env.Spec.WebHookEngine = v1.WebHookEngineProw
			settings := &env.Spec.TeamSettings
			settings.PromotionEngine = v1.PromotionEngineProw
			settings.ProwEngine = v1.ProwEngineTypeTekton
			settings.ImportMode = v1.ImportModeTypeYAML
			log.Logger().Debugf("Configuring the TeamSettings for Prow with engine %s", string(settings.ProwEngine))
			return nil
		}
		err := options.ModifyDevEnvironment(callback)
		if err != nil {
			return err
		}
	}
	return nil
}

func (options *InstallOptions) configureImportModeInTeamSettings() error {
	callback := func(env *v1.Environment) error {
		settings := &env.Spec.TeamSettings
		if string(settings.ImportMode) == "" {
			if options.Flags.Tekton {
				settings.ImportMode = v1.ImportModeTypeYAML
			} else {
				settings.ImportMode = v1.ImportModeTypeJenkinsfile
			}
		}
		log.Logger().Infof("Configuring the TeamSettings for ImportMode %s", string(settings.ImportMode))
		return nil
	}
	return options.ModifyDevEnvironment(callback)
}

func (options *InstallOptions) configureGitOpsMode(configStore configio.ConfigStore, namespace string) (string, string, error) {
	gitOpsDir := ""
	gitOpsEnvDir := ""
	if options.Flags.GitOpsMode {
		var err error
		if options.Flags.Dir == "" {
			options.Flags.Dir, err = util.ConfigDir()
			if err != nil {
				return "", "", err
			}
		}

		envName := fmt.Sprintf("environment-%s-dev", options.Flags.DefaultEnvironmentPrefix)
		gitOpsDir = filepath.Join(options.Flags.Dir, envName)
		gitOpsEnvDir = filepath.Join(gitOpsDir, "env")
		templatesDir := filepath.Join(gitOpsEnvDir, "templates")
		err = os.MkdirAll(templatesDir, util.DefaultWritePermissions)
		if err != nil {
			return "", "", errors.Wrapf(err, "Failed to make GitOps templates directory %s", templatesDir)
		}

		options.ModifyDevEnvironmentFn = func(callback func(env *v1.Environment) error) error {
			defaultEnv := kube.CreateDefaultDevEnvironment(namespace)
			_, err := gitOpsModifyEnvironment(templatesDir, kube.LabelValueDevEnvironment, defaultEnv, configStore, callback)
			return err
		}
		options.ModifyEnvironmentFn = func(name string, callback func(env *v1.Environment) error) error {
			defaultEnv := &v1.Environment{}
			defaultEnv.Labels = map[string]string{}
			_, err := gitOpsModifyEnvironment(templatesDir, name, defaultEnv, configStore, callback)
			return err
		}
		options.InitOptions.ModifyDevEnvironmentFn = options.ModifyDevEnvironmentFn
		options.modifyConfigMapCallback = func(name string, callback func(configMap *core_v1.ConfigMap) error) (*core_v1.ConfigMap, error) {
			return gitOpsModifyConfigMap(templatesDir, name, nil, configStore, callback)
		}
		options.modifySecretCallback = func(name string, callback func(secret *core_v1.Secret) error) (*core_v1.Secret, error) {
			if options.Flags.Vault {
				_, devNamespace, err := options.KubeClientAndDevNamespace()
				if err != nil {
					return nil, errors.Wrap(err, "getting team's dev namesapces")
				}
				vaultClient, err := options.SystemVaultClient(devNamespace)
				if err != nil {
					return nil, errors.Wrapf(err, "retrieving the system vault client in namespace %s", devNamespace)
				}
				vaultConfigStore := configio.NewVaultStore(vaultClient, vault.GitOpsSecretsPath)
				return gitOpsModifySecret(vault.GitOpsTemplatesPath, name, nil, vaultConfigStore, callback)
			}
			return gitOpsModifySecret(templatesDir, name, nil, configStore, callback)
		}
	}

	return gitOpsDir, gitOpsEnvDir, nil
}

func (options *InstallOptions) generateGitOpsDevEnvironmentConfig(gitOpsDir string) (string, error) {
	if options.Flags.GitOpsMode {
		log.Logger().Infof("\n\nGenerated the source code for the GitOps development environment at %s", util.ColorInfo(gitOpsDir))
		log.Logger().Infof("You can apply this to the kubernetes cluster at any time in this directory via: %s\n", util.ColorInfo("jx step env apply"))

		if !options.Flags.NoGitOpsEnvRepo {
			authConfigSvc, err := options.GitAuthConfigService()
			if err != nil {
				return "", errors.Wrap(err, "creating git auth config service")
			}
			config := &v1.Environment{
				Spec: v1.EnvironmentSpec{
					Label:             "Development",
					PromotionStrategy: v1.PromotionStrategyTypeNever,
					Kind:              v1.EnvironmentKindTypeDevelopment,
				},
			}
			config.Name = kube.LabelValueDevEnvironment
			var devEnv *v1.Environment
			err = options.ModifyDevEnvironment(func(env *v1.Environment) error {
				devEnv = env
				devEnv.Spec.TeamSettings.UseGitOps = true
				return nil
			})
			if err != nil {
				return "", errors.Wrap(err, "modifying the dev environment configuration")
			}
			envDir, err := util.EnvironmentsDir()
			if err != nil {
				return "", errors.Wrap(err, "getting the environments directory")
			}
			forkEnvGitURL := ""
			prefix := options.Flags.DefaultEnvironmentPrefix

			git := options.Git()
			gitRepoOptions, err := options.buildGitRepositoryOptionsForEnvironments()
			if err != nil || gitRepoOptions == nil {
				if err == nil {
					err = errors.New("empty git repository options")
				}
				return "", errors.Wrap(err, "building the git repository options for environment")
			}
			repo, gitProvider, err := kube.CreateEnvGitRepository(options.BatchMode, authConfigSvc, devEnv, devEnv, config, forkEnvGitURL, envDir,
				gitRepoOptions, options.CreateEnvOptions.HelmValuesConfig, prefix, git, options.ResolveChartMuseumURL, options.GetIOFileHandles())
			if err != nil || repo == nil || gitProvider == nil {
				return "", errors.Wrap(err, "creating git repository for the dev environment source")
			}

			dir := gitOpsDir
			err = git.Init(dir)
			if err != nil {
				return "", errors.Wrap(err, "initializing the dev environment repository")
			}
			err = options.ModifyDevEnvironment(func(env *v1.Environment) error {
				env.Spec.Source.URL = repo.CloneURL
				env.Spec.Source.Ref = "master"
				return nil
			})
			if err != nil {
				return "", errors.Wrap(err, "updating the source in the dev environment")
			}

			err = git.Add(dir, ".gitignore")
			if err != nil {
				return "", errors.Wrap(err, "adding gitignore to the dev environment")
			}
			err = git.Add(dir, "*")
			if err != nil {
				return "", errors.Wrap(err, "adding all files from dev environment repo to git")
			}
			err = options.Git().CommitIfChanges(dir, "Initial import of Dev Environment source")
			if err != nil {
				return "", errors.Wrap(err, "committing in git if there are changes")
			}
			userAuth := gitProvider.UserAuth()
			pushGitURL, err := git.CreateAuthenticatedURL(repo.CloneURL, &userAuth)
			if err != nil {
				return "", errors.Wrapf(err, "creating push URL for %q", repo.CloneURL)
			}
			err = git.SetRemoteURL(dir, "origin", pushGitURL)
			if err != nil {
				return "", errors.Wrapf(err, "setting remote origin to %q", pushGitURL)
			}
			err = git.PushMaster(dir)
			if err != nil {
				return "", errors.Wrapf(err, "pushing master from repository %q", dir)
			}
			log.Logger().Infof("Pushed Git repository to %s\n", util.ColorInfo(repo.HTMLURL))

			dir = filepath.Join(envDir, gitRepoOptions.Owner)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return "", errors.Wrapf(err, "creating directory %q", dir)
				}
			}
			dir = filepath.Join(dir, repo.Name)
			if err := util.RenameDir(gitOpsDir, dir, true); err != nil {
				return "", errors.Wrap(err, "renaming dev environment dir")
			}
			return filepath.Join(dir, "env"), nil
		}
	}

	return "", nil
}

func (options *InstallOptions) applyGitOpsDevEnvironmentConfig(gitOpsEnvDir string, namespace string) error {
	if options.Flags.GitOpsMode && !options.Flags.NoGitOpsEnvApply {
		applyEnv := true
		if !options.BatchMode {
			if answer, err := util.Confirm("Would you like to setup the Development Environment from the source code now?", true, "Do you want to apply the development environment helm charts now?", options.GetIOFileHandles()); err != nil {
				return err
			} else if !answer {
				applyEnv = false
			}
		}

		if applyEnv {
			// Reset the secret location cached in memory before creating the dev
			// environment. The location might have been changed in the cluster configuration.
			options.ResetSecretsLocation()

			envApplyOptions := &env.StepEnvApplyOptions{
				StepEnvOptions: env.StepEnvOptions{
					StepOptions: step.StepOptions{
						CommonOptions: options.CommonOptions,
					},
				},
				Dir:         gitOpsEnvDir,
				Namespace:   namespace,
				ChangeNs:    true,
				Vault:       options.Flags.Vault,
				ReleaseName: "jenkins-x",
			}

			err := envApplyOptions.Run()
			if err != nil {
				return errors.Wrap(err, "applying the dev environment configuration")
			}
		}
	}

	return nil
}

func (options *InstallOptions) setupGitOpsPostApply(ns string) error {
	if options.Flags.GitOpsMode && !options.Flags.NoGitOpsEnvSetup {
		if !options.Flags.Prow {
			err := options.configureJenkins(ns)
			if err != nil {
				return errors.Wrap(err, "configuring Jenkins")
			}
		} else {
			client, devNamespace, err := options.KubeClientAndDevNamespace()

			settings, err := options.TeamSettings()
			if err != nil {
				return errors.Wrap(err, "reading the team settings")
			}

			prow.AddDummyApplication(client, devNamespace, settings)
			if err != nil {
				return errors.Wrap(err, "adding dummy application")
			}
		}

		jxClient, devNs, err := options.JXClientAndDevNamespace()
		if err != nil {
			return errors.Wrap(err, "getting jx client and dev namesapce")
		}

		envs, err := kube.GetPermanentEnvironments(jxClient, devNs)
		if err != nil {
			return errors.Wrapf(err, "retrieving the current permanent environments in namespace %q", devNs)
		}
		devEnv, err := kube.GetDevEnvironment(jxClient, devNs)
		if err != nil {
			return errors.Wrapf(err, "get the dev environment namespace %q", devNs)
		}
		if devEnv != nil {
			envs = append(envs, devEnv)
		}

		errs := []error{}
		createEnvOpts := CreateEnvOptions{
			CreateOptions: createoptions.CreateOptions{
				CommonOptions: options.CommonOptions,
			},
			Prefix: options.Flags.DefaultEnvironmentPrefix,
			Prow:   options.Flags.Prow,
		}
		if options.BatchMode {
			createEnvOpts.BatchMode = options.BatchMode
		}
		for _, env := range envs {
			err := createEnvOpts.RegisterEnvironment(env, nil, nil)
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "registering environment %q", env.GetName()))
			}
			log.Logger().Infof("Registered environment %s", util.ColorInfo(env.GetName()))
		}
		return util.CombineErrors(errs...)
	}
	return nil
}

func (options *InstallOptions) installHelmBinaries() error {
	initOpts := &options.InitOptions
	helmBinary := initOpts.HelmBinary()
	dependencies := []string{}
	if !initOpts.Flags.RemoteTiller && !initOpts.Flags.NoTiller {
		binDir, err := util.JXBinLocation()
		if err != nil {
			return errors.Wrap(err, "reading jx bin location")
		}
		_, install, err := packages.ShouldInstallBinary("tiller")
		if !install && err == nil {
			confirm := &survey.Confirm{
				Message: "Uninstalling existing tiller binary:",
				Default: true,
			}
			flag := true
			err = survey.AskOne(confirm, &flag, nil)
			if err != nil || flag == false {
				return errors.New("Existing tiller must be uninstalled first in order to use the jx in tiller less mode")
			}
			// Uninstall helm and tiller first to avoid using some older version
			err = packages.UninstallBinary(binDir, "tiller")
			if err != nil {
				return errors.Wrap(err, "uninstalling existing tiller binary")
			}
		}

		_, install, err = packages.ShouldInstallBinary(helmBinary)
		if !install && err == nil {
			confirm := &survey.Confirm{
				Message: "Uninstalling existing helm binary:",
				Default: true,
			}
			flag := true
			err = survey.AskOne(confirm, &flag, nil)
			if err != nil || flag == false {
				return errors.New("Existing helm must be uninstalled first in order to use the jx in tiller less mode")
			}
			// Uninstall helm and tiller first to avoid using some older version
			err = packages.UninstallBinary(binDir, helmBinary)
			if err != nil {
				return errors.Wrap(err, "uninstalling existing helm binary")
			}
		}
		dependencies = append(dependencies, "tiller")
		options.Helm().SetHost(helm.GetTillerAddress())
	}
	dependencies = append(dependencies, helmBinary)
	return options.InstallMissingDependencies(dependencies)
}

// SetInstallValues sets the install values
func (options *InstallOptions) SetInstallValues(values map[string]string) {
	if values != nil {
		if options.installValues == nil {
			options.installValues = map[string]string{}
		}
		for k, v := range values {
			options.installValues[k] = v
		}
	}
}

func (options *InstallOptions) configureCloudProviderPreInit(client kubernetes.Interface) error {
	switch options.Flags.Provider {
	case cloud.AKS:
		err := options.CreateClusterAdmin()
		if err != nil {
			return errors.Wrap(err, "creating cluster admin for AKS cloud provider")
		}
		log.Logger().Info("created role cluster-admin")
	case cloud.AWS:
		fallthrough
	case cloud.EKS:
		err := options.ensureDefaultStorageClass(client, "gp2", "kubernetes.io/aws-ebs", "gp2")
		if err != nil {
			return errors.Wrap(err, "ensuring default storage for EKS/AWS cloud provider")
		}
	default:
		return nil
	}
	return nil
}

func (options *InstallOptions) configureCloudProivderPostInit(client kubernetes.Interface, namespace string) error {
	switch options.Flags.Provider {
	case cloud.OPENSHIFT:
		err := options.enableOpenShiftSCC(namespace)
		if err != nil {
			return errors.Wrap(err, "failed to enable the OpenShiftSCC")
		}
	case cloud.IKS:
		_, err := options.AddHelmBinaryRepoIfMissing(DEFAULT_IBMREPO_URL, "ibm", "", "")
		if err != nil {
			return errors.Wrap(err, "failed to add the IBM helm repo")
		}
		err = options.Helm().UpdateRepo()
		if err != nil {
			return errors.Wrap(err, "failed to update the helm repo")
		}
		helmOptions := helm.InstallChartOptions{
			Chart:       "ibm/ibmcloud-block-storage-plugin",
			ReleaseName: "ibmcloud-block-storage-plugin",
			NoForce:     true,
		}
		err = options.InstallChartWithOptions(helmOptions)
		if err != nil {
			return errors.Wrap(err, "failed to install/upgrade the IBM Cloud Block Storage drivers")
		}
		return options.changeDefaultStorageClass(client, "ibmc-block-bronze")
	default:
		return nil
	}

	return nil
}

func (options *InstallOptions) configureDockerRegistry(client kubernetes.Interface, namespace string) error {
	helmConfig := &options.CreateEnvOptions.HelmValuesConfig
	dockerRegistryConfig, dockerRegistry, err := options.configureCloudProviderRegistry(client, namespace)
	if err != nil {
		return errors.Wrap(err, "configure cloud provider docker registry")
	}
	if dockerRegistryConfig != "" {
		helmConfig.PipelineSecrets.DockerConfig = dockerRegistryConfig
	}
	if dockerRegistry != "" {
		if !options.Flags.Prow {
			if helmConfig.Jenkins.Servers.Global.EnvVars == nil {
				helmConfig.Jenkins.Servers.Global.EnvVars = map[string]string{}
			}
			helmConfig.Jenkins.Servers.Global.EnvVars["DOCKER_REGISTRY"] = dockerRegistry
		} else {
			helmConfig.DockerRegistry = dockerRegistry
		}
	}
	return nil
}

func (options *InstallOptions) configureCloudProviderRegistry(client kubernetes.Interface, namespace string) (string, string, error) {
	dockerRegistry, err := options.dockerRegistryValue()
	if err != nil {
		return "", "", err
	}
	kubeConfig, _, err := options.Kube().LoadConfig()
	if err != nil {
		return "", "", err
	}
	switch options.Flags.Provider {
	case cloud.AKS:
		server := kube.CurrentServer(kubeConfig)
		azureCLI := aks.NewAzureRunner()
		resourceGroup, name, cluster, err := azureCLI.GetClusterClient(server)
		if err != nil {
			return "", "", errors.Wrap(err, "getting cluster from Azure")
		}
		registryID := ""
		config, dockerRegistry, registryID, err := azureCLI.GetRegistry(options.Flags.AzureRegistrySubscription, resourceGroup, name, dockerRegistry)
		if err != nil {
			return "", "", errors.Wrap(err, "getting registry configuration from Azure")
		}
		azureCLI.AssignRole(cluster, registryID)
		log.Logger().Infof("Assign AKS %s a reader role for ACR %s", util.ColorInfo(server), util.ColorInfo(dockerRegistry))
		return config, dockerRegistry, nil
	case cloud.IKS:
		dockerRegistry = iks.GetClusterRegistry(client)
		config, err := iks.GetRegistryConfigJSON(dockerRegistry)
		if err != nil {
			return "", "", errors.Wrap(err, "getting IKS registry configuration")
		}
		return config, dockerRegistry, nil
	case cloud.OPENSHIFT:
		if dockerRegistry == "docker-registry.default.svc:5000" {
			config, err := options.enableOpenShiftRegistryPermissions(namespace, dockerRegistry)
			if err != nil {
				return "", "", errors.Wrap(err, "enabling OpenShift registry permissions")
			}
			return config, dockerRegistry, nil
		}
	}

	helmConfig := &options.CreateEnvOptions.HelmValuesConfig
	return helmConfig.PipelineSecrets.DockerConfig, dockerRegistry, nil
}

func (options *InstallOptions) registerAllCRDs() error {
	if !options.GitOpsMode {
		apisClient, err := options.ApiExtensionsClient()
		if err != nil {
			return errors.Wrap(err, "failed to create the API extensions client")
		}
		err = kube.RegisterAllCRDs(apisClient)
		if err != nil {
			return err
		}
	}
	return nil
}

func (options *InstallOptions) installCloudProviderDependencies() error {
	dependencies := []string{}
	err := options.InstallRequirements(options.Flags.Provider, dependencies...)
	if err != nil {
		return errors.Wrap(err, "installing cloud provider dependencies")
	}
	return nil
}

func (options *InstallOptions) getAdminSecrets(configStore configio.ConfigStore, providerEnvDir string, cloudEnvironmentSecretsLocation string) (string, *config.AdminSecretsConfig, error) {
	cloudEnvironmentSopsLocation := filepath.Join(providerEnvDir, opts.CloudEnvSopsConfigFile)
	if _, err := os.Stat(providerEnvDir); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("cloud environment dir %s not found", providerEnvDir)
	}
	sopsFileExists, err := util.FileExists(cloudEnvironmentSopsLocation)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to look for "+cloudEnvironmentSopsLocation)
	}

	adminSecretsServiceInit := false

	if sopsFileExists {
		log.Logger().Infof("Attempting to decrypt secrets file %s", util.ColorInfo(cloudEnvironmentSecretsLocation))
		// need to decrypt secrets now
		err = options.Helm().DecryptSecrets(cloudEnvironmentSecretsLocation)
		if err != nil {
			return "", nil, errors.Wrap(err, "failed to decrypt "+cloudEnvironmentSecretsLocation)
		}

		cloudEnvironmentSecretsDecryptedLocation := filepath.Join(providerEnvDir, opts.CloudEnvSecretsFile+".dec")
		decryptedSecretsFile, err := util.FileExists(cloudEnvironmentSecretsDecryptedLocation)
		if err != nil {
			return "", nil, errors.Wrap(err, "failed to look for "+cloudEnvironmentSecretsDecryptedLocation)
		}

		if decryptedSecretsFile {
			log.Logger().Infof("Successfully decrypted %s", util.ColorInfo(cloudEnvironmentSecretsDecryptedLocation))
			cloudEnvironmentSecretsLocation = cloudEnvironmentSecretsDecryptedLocation

			err = options.AdminSecretsService.NewAdminSecretsConfigFromSecret(cloudEnvironmentSecretsDecryptedLocation)
			if err != nil {
				return "", nil, errors.Wrap(err, "failed to create the admin secret config service from the decrypted secrets file")
			}
			adminSecretsServiceInit = true
		}
	}

	if !adminSecretsServiceInit {
		err = options.AdminSecretsService.NewAdminSecretsConfig()
		if err != nil {
			return "", nil, errors.Wrap(err, "failed to create the admin secret config service")
		}
	}

	dir, err := util.ConfigDir()
	if err != nil {
		return "", nil, errors.Wrap(err, "creating a temporary config dir for Git credentials")
	}

	adminSecrets := &options.AdminSecretsService.Secrets
	adminSecretsFileName := filepath.Join(dir, opts.AdminSecretsFile)
	err = configStore.WriteObject(adminSecretsFileName, adminSecrets)
	if err != nil {
		return "", nil, errors.Wrapf(err, "writing the admin secrets in the secrets file '%s'", adminSecretsFileName)
	}

	if options.Flags.Vault {
		// lets make sure the devNamespace hasn't been overwritten to "default"
		if options.Flags.Namespace != "" {
			options.SetDevNamespace(options.Flags.Namespace)
		}
		err := options.storeAdminCredentialsInVault(&options.AdminSecretsService)
		if err != nil {
			return "", nil, errors.Wrapf(err, "storing the admin credentials in vault")
		}
	}

	return adminSecretsFileName, adminSecrets, nil
}

// ConfigureKaniko configures the kaniko SA and secret
func (options *InstallOptions) ConfigureKaniko() error {
	if options.Flags.Kaniko {
		if options.Flags.Provider != cloud.GKE {
			log.Logger().Infof("we are assuming your IAM roles are setup so that Kaniko can push images to your docker registry\n")
			return nil
		}

		serviceAccountDir, err := ioutil.TempDir("", "gke")
		if err != nil {
			return errors.Wrap(err, "creating a temporary folder where the service account will be stored")
		}
		defer os.RemoveAll(serviceAccountDir)

		clusterName := options.installValues[kube.ClusterName]
		projectID := options.installValues[kube.ProjectID]
		if projectID == "" || clusterName == "" {
			if kubeClient, ns, err := options.KubeClientAndDevNamespace(); err == nil {
				if data, err := kube.ReadInstallValues(kubeClient, ns); err == nil && data != nil {
					if projectID == "" {
						projectID = data[kube.ProjectID]
					}
					if clusterName == "" {
						clusterName = data[kube.ClusterName]
					}
				}
			}
		}
		if projectID == "" {
			projectID, err = options.GetGoogleProjectID("")
			if err != nil {
				return errors.Wrap(err, "getting the GCP project ID")
			}
		}
		if clusterName == "" {
			clusterName, err = options.GetGKEClusterNameFromContext()
			if err != nil {
				return errors.Wrap(err, "getting the GKE cluster name from current context")
			}
		}

		serviceAccountName := naming.ToValidGCPServiceAccount(fmt.Sprintf("%s-ko", clusterName))
		log.Logger().Infof("Configuring Kaniko service account %s for project %s", util.ColorInfo(serviceAccountName), util.ColorInfo(projectID))
		serviceAccountPath, err := options.GCloud().GetOrCreateServiceAccount(serviceAccountName, projectID, serviceAccountDir, gke.KanikoServiceAccountRoles)
		if err != nil {
			return errors.Wrap(err, "creating the service account")
		}

		serviceAccount, err := ioutil.ReadFile(serviceAccountPath)
		if err != nil {
			return errors.Wrapf(err, "reading the service account from file '%s'", serviceAccountPath)
		}

		options.AdminSecretsService.Flags.KanikoSecret = string(serviceAccount)

	}
	return nil
}

func (options *InstallOptions) createSystemVault(client kubernetes.Interface, namespace string, ic *kube.IngressConfig) error {
	if options.Flags.GitOpsMode && !options.Flags.NoGitOpsVault || options.Flags.Vault {
		if options.Flags.Provider != cloud.GKE && options.Flags.Provider != cloud.EKS && options.Flags.Provider != cloud.AWS {
			return fmt.Errorf("system vault is not supported for %s provider", options.Flags.Provider)
		}

		if options.installValues == nil {
			return errors.New("no install values provided")
		}

		// Configure the vault flag if only GitOps mode is on
		options.Flags.Vault = true

		vaultOperatorClient, err := options.VaultOperatorClient()
		if err != nil {
			return err
		}

		systemVaultName, err := kubevault.SystemVaultName(options.Kube())
		if err != nil {
			return errors.Wrap(err, "building the system vault name from cluster name")
		}

		options.installValues[kube.SystemVaultName] = systemVaultName

		if kubevault.FindVault(vaultOperatorClient, systemVaultName, namespace) {
			log.Logger().Infof("System vault named %s in namespace %s already exists",
				util.ColorInfo(systemVaultName), util.ColorInfo(namespace))
		} else {
			log.Logger().Info("Creating new system vault")

			resolver, err := options.CreateVersionResolver("", "")
			if err != nil {
				return errors.Wrap(err, "creating the docker image version resolver")
			}

			err = options.installOperator(resolver, namespace)
			if err != nil {
				return errors.Wrap(err, "installing Vault operator")
			}

			vaultCreateParam := create.VaultCreationParam{
				VaultName:            systemVaultName,
				Namespace:            namespace,
				ClusterName:          options.installValues[kube.ClusterName],
				SecretsPathPrefix:    pkgvault.DefaultSecretsPathPrefix,
				KubeProvider:         options.Flags.Provider,
				KubeClient:           client,
				VaultOperatorClient:  vaultOperatorClient,
				VersionResolver:      *resolver,
				FileHandles:          options.GetIOFileHandles(),
				CreateCloudResources: true,
				Boot:                 false,
				BatchMode:            true,
			}

			if options.Flags.Provider == cloud.GKE {
				gkeParam := &create.GKEParam{
					ProjectID: options.installValues[kube.ProjectID],
					Zone:      options.installValues[kube.Zone],
				}
				vaultCreateParam.GKE = gkeParam
			}

			if options.Flags.Provider == cloud.EKS {
				awsParam, err := options.createAWSParam(options.installValues[kube.Region])
				if err != nil {
					return errors.Wrap(err, "unable to create Vault creation parameter from requirements")
				}
				vaultCreateParam.AWS = &awsParam
			}

			vaultCreator := create.NewVaultCreator()
			err = vaultCreator.CreateOrUpdateVault(vaultCreateParam)
			if err != nil {
				return errors.Wrap(err, "unable to create/update Vault")
			}

			err = options.exposeVault(systemVaultName, namespace, ic)
			if err != nil {
				return errors.Wrap(err, "unable to expose Vault")
			}

			log.Logger().Infof("System vault created named %s in namespace %s.",
				util.ColorInfo(systemVaultName), util.ColorInfo(namespace))
		}

		// Make sure that the dev namespace wasn't overwritten
		options.SetDevNamespace(namespace)

		err = options.SetSecretsLocation(secrets.VaultLocationKind, false)
		if err != nil {
			return errors.Wrap(err, "setting the secrets location as vault")
		}
	}
	return nil
}

func (options *InstallOptions) installOperator(resolver *versionstream.VersionResolver, ns string) error {
	tag, err := options.vaultOperatorImageTag(resolver)
	if err != nil {
		return errors.Wrap(err, "unable to determine Vault operator version")
	}

	values := []string{
		"image.repository=" + kubevault.VaultOperatorImage,
		"image.tag=" + tag,
	}
	log.Logger().Infof("Installing %s operator with helm values: %v", util.ColorInfo(kube.DefaultVaultOperatorReleaseName), util.ColorInfo(values))

	helmOptions := helm.InstallChartOptions{
		Chart:       kube.ChartVaultOperator,
		ReleaseName: kube.DefaultVaultOperatorReleaseName,
		Version:     options.Version,
		Ns:          ns,
		SetValues:   values,
	}
	err = options.InstallChartWithOptions(helmOptions)
	if err != nil {
		return errors.Wrap(err, "unable to install vault operator")
	}

	log.Logger().Infof("Vault operator installed in namespace %s", ns)
	return nil
}

// vaultOperatorImageTag lookups the vault operator image tag in the version stream
func (options *InstallOptions) vaultOperatorImageTag(resolver *versionstream.VersionResolver) (string, error) {
	fullImage, err := resolver.ResolveDockerImage(kubevault.VaultOperatorImage)
	if err != nil {
		return "", errors.Wrapf(err, "looking up the vault-operator %q image into the version stream",
			kubevault.VaultOperatorImage)
	}
	parts := strings.Split(fullImage, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("no tag found for image %q in version stream", kubevault.VaultOperatorImage)
	}
	return parts[1], nil
}

func (options *InstallOptions) createAWSParam(defaultRegion string) (create.AWSParam, error) {
	if defaultRegion == "" {
		return create.AWSParam{}, errors.New("unable to find cluster region in requirements")
	}

	dynamoDBRegion := options.DynamoDBRegion
	if dynamoDBRegion == "" {
		dynamoDBRegion = defaultRegion
		log.Logger().Infof("Region not specified for DynamoDB, defaulting to %s", util.ColorInfo(defaultRegion))
	}

	kmsRegion := options.KMSRegion
	if kmsRegion == "" {
		kmsRegion = defaultRegion
		log.Logger().Infof("Region not specified for KMS, defaulting to %s", util.ColorInfo(defaultRegion))

	}

	s3Region := options.S3Region
	if s3Region == "" {
		s3Region = defaultRegion
		log.Logger().Infof("Region not specified for S3, defaulting to %s", util.ColorInfo(defaultRegion))
	}

	awsParam := create.AWSParam{
		IAMUsername:     options.ProvidedIAMUsername,
		S3Bucket:        options.S3Bucket,
		S3Region:        s3Region,
		S3Prefix:        options.S3Prefix,
		DynamoDBTable:   options.DynamoDBTable,
		DynamoDBRegion:  dynamoDBRegion,
		KMSKeyID:        options.KMSKeyID,
		KMSRegion:       kmsRegion,
		AccessKeyID:     options.AccessKeyID,
		SecretAccessKey: options.SecretAccessKey,
		AutoCreate:      options.AutoCreate,
	}

	return awsParam, nil
}

func (options *InstallOptions) exposeVault(vaultService string, namespace string, ic *kube.IngressConfig) error {
	client, err := options.KubeClient()
	if err != nil {
		return err
	}
	svc, err := client.CoreV1().Services(namespace).Get(vaultService, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "getting the vault service: %s", vaultService)
	}
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	if svc.Annotations[kube.AnnotationExpose] == "" {
		svc.Annotations[kube.AnnotationExpose] = "true"
		svc.Annotations[kube.AnnotationExposePort] = vault.DefaultVaultPort
		svc, err = client.CoreV1().Services(namespace).Update(svc)
		if err != nil {
			return errors.Wrapf(err, "updating %s service annotations", vaultService)
		}
	}

	upgradeIngOpts := &upgrade.UpgradeIngressOptions{
		CommonOptions:       options.CommonOptions,
		Namespaces:          []string{namespace},
		Services:            []string{vaultService},
		IngressConfig:       *ic,
		SkipResourcesUpdate: true,
		WaitForCerts:        true,
	}
	return upgradeIngOpts.Run()
}

func (options *InstallOptions) storeSecretYamlFilesInVault(path string, files ...string) error {
	_, devNamespace, err := options.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "getting team's dev namespace")
	}
	vaultClient, err := options.SystemVaultClient(devNamespace)
	if err != nil {
		return errors.Wrapf(err, "retrieving the system vault client in namespace %s", devNamespace)
	}

	err = vault.WriteYamlFiles(vaultClient, path, files...)
	if err != nil {
		return errors.Wrapf(err, "storing in vault the secret YAML files: %s", strings.Join(files, ","))
	}

	return nil
}

func (options *InstallOptions) storeAdminCredentialsInVault(svc *config.AdminSecretsService) error {
	_, devNamespace, err := options.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "getting team's dev namespace")
	}
	vaultClient, err := options.SystemVaultClient(devNamespace)
	if err != nil {
		return errors.Wrapf(err, "retrieving the system vault client in namespace %s", devNamespace)
	}
	secrets := map[vault.AdminSecret]config.BasicAuth{
		vault.JenkinsAdminSecret:     svc.JenkinsAuth(),
		vault.IngressAdminSecret:     svc.IngressAuth(),
		vault.ChartmuseumAdminSecret: svc.ChartMuseumAuth(),
		vault.GrafanaAdminSecret:     svc.GrafanaAuth(),
		vault.NexusAdminSecret:       svc.NexusAuth(),
	}
	for secretName, secret := range secrets {
		path := vault.AdminSecretPath(secretName)
		err := vault.WriteBasicAuth(vaultClient, path, secret)
		if err != nil {
			return errors.Wrapf(err, "storing in vault the basic auth credentials for %s", secretName)
		}
	}
	return nil
}

func (options *InstallOptions) configureBuildPackMode() error {
	ebp := &edit.EditBuildPackOptions{
		BuildPackName: options.Flags.BuildPackName,
	}
	ebp.CommonOptions = options.CommonOptions

	return ebp.Run()
}

func (options *InstallOptions) configureLongTermStorageBucket() error {

	if options.IsConfigExplicitlySet("install", longTermStorageFlagName) && !options.Flags.LongTermStorage {
		return nil
	}

	if !options.BatchMode && !options.Flags.LongTermStorage {
		if options.AdvancedMode {
			surveyOpts := survey.WithStdio(options.In, options.Out, options.Err)
			confirm := &survey.Confirm{
				Message: fmt.Sprintf("Would you like to enable long term logs storage?"+
					" A bucket for provider %s will be created", options.Flags.Provider),
				Default: true,
			}
			err := survey.AskOne(confirm, &options.Flags.LongTermStorage, nil, surveyOpts)
			if err != nil {
				return errors.Wrap(err, "asking to enable Long Term Storage")
			}
		} else {
			if options.Flags.Provider == cloud.GKE {
				options.Flags.LongTermStorage = true
				log.Logger().Infof(util.QuestionAnswer("Default enabling long term logs storage", util.YesNo(options.Flags.LongTermStorage)))
			} else {
				options.Flags.LongTermStorage = false
				log.Logger().Debugf("Long Term Storage not supported by provider '%s', disabling this option", options.Flags.Provider)
			}
		}
	} else {
		log.Logger().Infof(util.QuestionAnswer("Configured to use long term logs storage", util.YesNo(options.Flags.LongTermStorage)))
	}

	if options.Flags.LongTermStorage {

		var bucketURL string
		switch strings.ToUpper(options.Flags.Provider) {
		case "GKE":
			err := options.ensureGKEInstallValuesAreFilled()
			if err != nil {
				return errors.Wrap(err, "filling install values with cluster information")
			}
			bucketURL, err = gkeStorage.EnableLongTermStorage(options.GCloud(), options.installValues,
				options.Flags.LongTermStorageBucketName)
			if err != nil {
				return errors.Wrap(err, "enabling long term storage on GKE")
			}
			break
		default:
			return errors.Errorf("long term storage is not yet supported for provider %s", options.Flags.Provider)
		}
		return options.assignBucketToTeamStorage(bucketURL)
	}
	return nil
}

func (options *InstallOptions) assignBucketToTeamStorage(bucketURL string) error {
	//Enable storage of logs into the bucketURL
	eso := edit.EditStorageOptions{
		CommonOptions: options.CommonOptions,
		StorageLocation: v1.StorageLocation{
			Classifier: "default",
			BucketURL:  bucketURL,
		},
	}
	infoBucketURL := util.ColorInfo(bucketURL)
	log.Logger().Debugf("Enabling default storage for current team in the bucket %s", infoBucketURL)
	err := eso.Run()
	if err != nil {
		return errors.Wrapf(err, "there was a problem executing `jx edit -c default --bucket-url=%s",
			infoBucketURL)
	}

	eso.StorageLocation.Classifier = "logs"
	log.Logger().Debugf("Enabling logs storage for current team in the bucket %s", infoBucketURL)
	//Only GCS seems to be supported atm
	err = eso.Run()
	if err != nil {
		return errors.Wrapf(err, "there was a problem executing `jx edit -c logs --bucket-url=%s",
			infoBucketURL)
	}

	return nil
}

func (options *InstallOptions) ensureGKEInstallValuesAreFilled() error {
	if options.installValues == nil {
		options.installValues = make(map[string]string)
	}

	if options.installValues[kube.ProjectID] == "" {
		currentProjectID, err := gke.GetCurrentProject()
		if err != nil {
			return errors.Wrap(err, "obtaining the current project from GKE context")
		}
		options.installValues[kube.ProjectID] = currentProjectID
	}

	if options.installValues[kube.Zone] == "" {
		gcpCurrentZone, err := options.GetGoogleZone(options.installValues[kube.ProjectID], "")
		if err != nil {
			return errors.Wrap(err, "asking for the zone to create the bucket into")
		}
		options.installValues[kube.Zone] = gcpCurrentZone
	}

	if options.installValues[kube.ClusterName] == "" {
		clusterName, err := cluster.Name(options.Kube())
		if err != nil {
			return errors.Wrap(err, "obtaining the current cluster name")
		}
		options.installValues[kube.ClusterName] = clusterName
	}

	return nil
}

func (options *InstallOptions) saveIngressConfig() (*kube.IngressConfig, error) {
	exposeController := options.CreateEnvOptions.HelmValuesConfig.ExposeController
	tls, err := util.ParseBool(exposeController.Config.TLSAcme)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TLS exposecontroller boolean %v", err)
	}
	domain := exposeController.Config.Domain
	ic := kube.IngressConfig{
		Domain:      domain,
		TLS:         tls,
		Exposer:     exposeController.Config.Exposer,
		UrlTemplate: exposeController.Config.URLTemplate,
	}
	// save ingress config details to a configmap
	_, err = options.saveAsConfigMap(kube.IngressConfigConfigmap, ic)
	if err != nil {
		return nil, err
	}
	return &ic, nil
}

func (options *InstallOptions) saveClusterConfig() error {
	jxInstallConfig := &kube.JXInstallConfig{
		KubeProvider: options.Flags.Provider,
	}
	kubeConfig, _, err := options.Kube().LoadConfig()
	if err != nil {
		return errors.Wrap(err, "retrieving the current kube config")
	}
	if kubeConfig != nil {
		kubeConfigContext := kube.CurrentContext(kubeConfig)
		if kubeConfigContext != nil {
			server := kube.Server(kubeConfig, kubeConfigContext)
			certificateAuthorityData := kube.CertificateAuthorityData(kubeConfig, kubeConfigContext)
			jxInstallConfig.Server = server
			jxInstallConfig.CA = certificateAuthorityData
		}
	}

	if options.installValues == nil {
		options.installValues = map[string]string{}
	}
	installVersionKey := "jx-install-version"
	if options.installValues[installVersionKey] == "" {
		options.installValues[installVersionKey] = version2.GetVersion()
	}
	var secretsLocation secrets.SecretsLocationKind
	if options.Flags.Vault {
		secretsLocation = secrets.VaultLocationKind
	} else {
		secretsLocation = secrets.FileSystemLocationKind
	}
	options.installValues[secrets.SecretsLocationKey] = string(secretsLocation)

	_, err = options.ModifyConfigMap(kube.ConfigMapNameJXInstallConfig, func(cm *core_v1.ConfigMap) error {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		data := util.ToStringMapStringFromStruct(jxInstallConfig)
		for k, v := range data {
			cm.Data[k] = v
		}
		iv := options.installValues
		for k, v := range iv {
			cm.Data[k] = v
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "saving cluster config into config map %q", kube.ConfigMapNameJXInstallConfig)
	}
	return nil
}

func (options *InstallOptions) configureJenkins(namespace string) error {
	if !options.Flags.Prow {
		log.Logger().Info("Configure Jenkins API Token")
		if isOpenShiftProvider(options.Flags.Provider) {
			options.CreateJenkinsUserOptions.CommonOptions = options.CommonOptions
			options.CreateJenkinsUserOptions.Password = options.AdminSecretsService.Flags.DefaultAdminPassword
			options.CreateJenkinsUserOptions.Username = "jenkins-admin"
			options.CreateJenkinsUserOptions.Verbose = false
			jenkinsSaToken, err := options.GetCommandOutput("", "oc", "serviceaccounts", "get-token", "jenkins", "-n", namespace)
			if err != nil {
				return errors.Wrap(err, "getting token from service account jenkins")
			}
			options.CreateJenkinsUserOptions.BearerToken = jenkinsSaToken
			err = options.CreateJenkinsUserOptions.Run()
			if err != nil {
				return errors.Wrap(err, "creating Jenkins API token")
			}
		} else {
			err := options.Retry(3, 2*time.Second, func() (err error) {
				_, devNamespace, err := options.KubeClientAndDevNamespace()
				if err != nil {
					return errors.Wrap(err, "getting team's dev namespace")
				}
				options.CreateJenkinsUserOptions.CommonOptions = options.CommonOptions
				options.CreateJenkinsUserOptions.Namespace = devNamespace
				options.CreateJenkinsUserOptions.RecreateToken = true
				options.CreateJenkinsUserOptions.Username = options.AdminSecretsService.Flags.DefaultAdminUsername
				options.CreateJenkinsUserOptions.Password = options.AdminSecretsService.Flags.DefaultAdminPassword
				options.CreateJenkinsUserOptions.Verbose = false
				options.CreateJenkinsUserOptions.RecreateToken = true
				if options.BatchMode {
					options.CreateJenkinsUserOptions.BatchMode = true
				}
				err = options.CreateJenkinsUserOptions.Run()
				return
			})
			if err != nil {
				return errors.Wrap(err, "creating Jenkins API token")
			}
		}

		err := options.UpdateJenkinsURL([]string{namespace})
		if err != nil {
			log.Logger().Warnf("Failed to update the Jenkins external URL: %s", err)
		}
	}
	return nil
}

func (options *InstallOptions) installAddons() error {
	if !options.Flags.GitOpsMode {
		addonConfig, err := addon.LoadAddonsConfig()
		if err != nil {
			return errors.Wrap(err, "failed to load the addons configuration")
		}

		for _, ac := range addonConfig.Addons {
			if ac.Enabled {
				err = options.installAddon(ac.Name)
				if err != nil {
					return fmt.Errorf("failed to install addon %s: %s", ac.Name, err)
				}
			}
		}
	}
	return nil
}

func (options *InstallOptions) createEnvironments(namespace string) error {
	if !options.Flags.NoDefaultEnvironments {
		if options.Flags.GitOpsMode {
			options.SetDevNamespace(namespace)
			options.CreateEnvOptions.CommonOptions = options.CommonOptions
			options.CreateEnvOptions.GitOpsMode = true
			options.CreateEnvOptions.ModifyDevEnvironmentFn = options.ModifyDevEnvironmentFn
			options.CreateEnvOptions.ModifyEnvironmentFn = options.ModifyEnvironmentFn
		}

		log.Logger().Info("Creating default staging and production environments")
		_, devNamespace, err := options.KubeClientAndDevNamespace()
		if err != nil {
			return errors.Wrap(err, "getting team's dev namespace")
		}
		gitRepoOptions, err := options.buildGitRepositoryOptionsForEnvironments()
		if err != nil || gitRepoOptions == nil {
			return errors.Wrap(err, "building the Git repository options for environments")
		}
		options.CreateEnvOptions.GitRepositoryOptions = *gitRepoOptions
		// lets not fail if environments already exist
		options.CreateEnvOptions.Update = true

		options.CreateEnvOptions.Prefix = options.Flags.DefaultEnvironmentPrefix
		options.CreateEnvOptions.Prow = options.Flags.Prow
		if options.BatchMode {
			options.CreateEnvOptions.BatchMode = options.BatchMode
		}
		options.CreateEnvOptions.Options.Name = "staging"
		options.CreateEnvOptions.Options.Spec.Label = "Staging"
		options.CreateEnvOptions.Options.Spec.Order = 100
		options.CreateEnvOptions.Options.Spec.RemoteCluster = options.Flags.RemoteEnvironments
		err = options.CreateEnvOptions.Run()
		if err != nil {
			return errors.Wrapf(err, "failed to create staging environment in namespace %s", devNamespace)
		}
		options.CreateEnvOptions.Options.Name = "production"
		options.CreateEnvOptions.Options.Spec.Label = "Production"
		options.CreateEnvOptions.Options.Spec.Order = 200
		options.CreateEnvOptions.Options.Spec.RemoteCluster = options.Flags.RemoteEnvironments
		options.CreateEnvOptions.Options.Spec.PromotionStrategy = v1.PromotionStrategyTypeManual
		options.CreateEnvOptions.PromotionStrategy = string(v1.PromotionStrategyTypeManual)

		err = options.CreateEnvOptions.Run()
		if err != nil {
			return errors.Wrapf(err, "failed to create the production environment in namespace %s", devNamespace)
		}
	}
	return nil
}

func (options *InstallOptions) modifySecrets(helmConfig *config.HelmValuesConfig, adminSecrets *config.AdminSecretsConfig) error {
	var err error
	data := make(map[string][]byte)
	data[opts.ExtraValuesFile], err = yaml.Marshal(helmConfig)
	if err != nil {
		return err
	}
	data[opts.AdminSecretsFile], err = yaml.Marshal(adminSecrets)
	if err != nil {
		return err
	}
	_, err = options.ModifySecret(opts.JXInstallConfig, func(secret *core_v1.Secret) error {
		secret.Data = data
		return nil
	})
	return err
}

// ModifySecret modifies the Secret either live or via the file system if generating the GitOps source
func (options *InstallOptions) ModifySecret(name string, callback func(*core_v1.Secret) error) (*core_v1.Secret, error) {
	if options.modifySecretCallback == nil {
		options.modifySecretCallback = func(name string, callback func(*core_v1.Secret) error) (*core_v1.Secret, error) {
			kubeClient, ns, err := options.KubeClientAndDevNamespace()
			if err != nil {
				return nil, err
			}
			return kube.DefaultModifySecret(kubeClient, ns, name, callback, nil)
		}
	}
	return options.modifySecretCallback(name, callback)
}

// ModifyConfigMap modifies the ConfigMap either live or via the file system if generating the GitOps source
func (options *InstallOptions) ModifyConfigMap(name string, callback func(*core_v1.ConfigMap) error) (*core_v1.ConfigMap, error) {
	if options.modifyConfigMapCallback == nil {
		options.modifyConfigMapCallback = func(name string, callback func(*core_v1.ConfigMap) error) (*core_v1.ConfigMap, error) {
			kubeClient, ns, err := options.KubeClientAndDevNamespace()
			if err != nil {
				return nil, err
			}
			return kube.DefaultModifyConfigMap(kubeClient, ns, name, callback, nil)
		}
	}
	return options.modifyConfigMapCallback(name, callback)
}

// gitOpsModifyConfigMap provides a helper function to lazily create, modify and save the YAML file in the given directory
func gitOpsModifyConfigMap(dir string, name string, defaultResource *core_v1.ConfigMap, configStore configio.ConfigStore,
	callback func(configMap *core_v1.ConfigMap) error) (*core_v1.ConfigMap, error) {
	answer := core_v1.ConfigMap{}
	fileName := filepath.Join(dir, name+"-configmap.yaml")
	exists, err := util.FileExists(fileName)
	if err != nil {
		return &answer, errors.Wrapf(err, "Could not check if file exists %s", fileName)
	}
	if exists {
		err = configStore.ReadObject(fileName, &answer)
		if err != nil {
			return &answer, errors.Wrapf(err, "Failed to unmarshall YAML file %s", fileName)
		}
	} else if defaultResource != nil {
		answer = *defaultResource
	} else {
		answer.Name = name
	}
	err = callback(&answer)
	if err != nil {
		return &answer, err
	}
	if answer.APIVersion == "" {
		answer.APIVersion = "v1"
	}
	if answer.Kind == "" {
		answer.Kind = "ConfigMap"
	}
	if answer.Data == nil {
		answer.Data = make(map[string]string)
	}
	err = configStore.WriteObject(fileName, &answer)
	if err != nil {
		return &answer, errors.Wrapf(err, "Could not save file %s", fileName)
	}
	return &answer, nil
}

// gitOpsModifySecret provides a helper function to lazily create, modify and save the YAML file in the given directory
func gitOpsModifySecret(dir string, name string, defaultResource *core_v1.Secret, configStore configio.ConfigStore,
	callback func(secret *core_v1.Secret) error) (*core_v1.Secret, error) {
	answer := core_v1.Secret{}
	fileName := filepath.Join(dir, name+"-secret.yaml")
	exists, err := util.FileExists(fileName)
	if err != nil {
		return &answer, errors.Wrapf(err, "checking if file exists %s", fileName)
	}
	if exists {
		// lets unmarshall the data
		err = configStore.ReadObject(fileName, &answer)
		if err != nil {
			return &answer, err
		}
	} else if defaultResource != nil {
		answer = *defaultResource
	} else {
		answer.Name = name
	}
	err = callback(&answer)
	if err != nil {
		return &answer, err
	}
	if answer.APIVersion == "" {
		answer.APIVersion = "v1"
	}
	if answer.Kind == "" {
		answer.Kind = "Secret"
	}
	err = configStore.WriteObject(fileName, &answer)
	if err != nil {
		return &answer, errors.Wrapf(err, "Could not save file %s", fileName)
	}
	return &answer, nil
}

// gitOpsModifyEnvironment provides a helper function to lazily create, modify and save the YAML file in the given directory
func gitOpsModifyEnvironment(dir string, name string, defaultEnvironment *v1.Environment, configStore configio.ConfigStore,
	callback func(*v1.Environment) error) (*v1.Environment, error) {
	answer := v1.Environment{}
	fileName := filepath.Join(dir, name+"-env.yaml")
	exists, err := util.FileExists(fileName)
	if err != nil {
		return &answer, errors.Wrapf(err, "Could not check if file exists %s", fileName)
	}
	if exists {
		// lets unmarshal the data
		err := configStore.ReadObject(fileName, &answer)
		if err != nil {
			return &answer, err
		}
	} else if defaultEnvironment != nil {
		answer = *defaultEnvironment
	}
	err = callback(&answer)
	if err != nil {
		return &answer, err
	}
	answer.Name = name
	if answer.APIVersion == "" {
		answer.APIVersion = jenkinsio.GroupAndVersion
	}
	if answer.Kind == "" {
		answer.Kind = "Environment"
	}
	err = configStore.WriteObject(fileName, &answer)
	if err != nil {
		return &answer, errors.Wrapf(err, "Could not save file %s", fileName)
	}
	return &answer, nil
}

func isOpenShiftProvider(provider string) bool {
	switch provider {
	case cloud.OPENSHIFT:
		return true
	default:
		return false
	}
}

func (options *InstallOptions) enableOpenShiftSCC(ns string) error {
	log.Logger().Infof("Enabling anyuid for the Jenkins service account in namespace %s", ns)
	err := options.RunCommand("oc", "adm", "policy", "add-scc-to-user", "anyuid", "system:serviceaccount:"+ns+":jenkins")
	if err != nil {
		return err
	}
	err = options.RunCommand("oc", "adm", "policy", "add-scc-to-user", "hostaccess", "system:serviceaccount:"+ns+":jenkins")
	if err != nil {
		return err
	}
	err = options.RunCommand("oc", "adm", "policy", "add-scc-to-user", "privileged", "system:serviceaccount:"+ns+":jenkins")
	if err != nil {
		return err
	}
	// try fix monocular
	return options.RunCommand("oc", "adm", "policy", "add-scc-to-user", "anyuid", "system:serviceaccount:"+ns+":default")
}

func (options *InstallOptions) enableOpenShiftRegistryPermissions(ns string, dockerRegistry string) (string, error) {
	log.Logger().Infof("Enabling permissions for OpenShift registry in namespace %s", ns)
	// Open the registry so any authenticated user can pull images from the jx namespace
	err := options.RunCommand("oc", "adm", "policy", "add-role-to-group", "system:image-puller", "system:authenticated", "-n", ns)
	if err != nil {
		return "", err
	}
	err = options.EnsureServiceAccount(ns, "jenkins-x-registry")
	if err != nil {
		return "", err
	}
	err = options.RunCommand("oc", "adm", "policy", "add-cluster-role-to-user", "registry-admin", "system:serviceaccount:"+ns+":jenkins-x-registry")
	if err != nil {
		return "", err
	}
	registryToken, err := options.GetCommandOutput("", "oc", "serviceaccounts", "get-token", "jenkins-x-registry", "-n", ns)
	if err != nil {
		return "", err
	}
	return `{"auths": {"` + dockerRegistry + `": {"auth": "` + base64.StdEncoding.EncodeToString([]byte("serviceaccount:"+registryToken)) + `"}}}`, nil
}

func (options *InstallOptions) logAdminPassword() {
	astrix := `

	********************************************************

	     NOTE: %s

	********************************************************

	`
	if options.Flags.Vault {
		log.Logger().Infof(astrix+"\n", fmt.Sprintf("Your admin password is in vault: %s", util.ColorInfo("eval `jx get vault-config` && vault kv get secret/admin/jenkins")))
	} else {
		log.Logger().Infof(astrix+"\n", fmt.Sprintf("Your admin password is: %s", util.ColorInfo(options.AdminSecretsService.Flags.DefaultAdminPassword)))
	}
}

func (options *InstallOptions) logNameServers() {
	output := `
	********************************************************

	    External DNS: %s

	********************************************************
	`
	if options.InitOptions.Flags.ExternalDNS {
		log.Logger().Infof(output, fmt.Sprintf("Please delegate %s via \n\tyour registrar onto the following name servers: \n\t\t%s",
			util.ColorInfo(options.Flags.Domain), util.ColorInfo(strings.Join(options.CommonOptions.NameServers, "\n\t\t"))))
	}
}

// LoadVersionFromCloudEnvironmentsDir lets load the jenkins-x-platform version
func LoadVersionFromCloudEnvironmentsDir(wrkDir string, configStore configio.ConfigStore) (string, error) {
	version, err := versionstream.LoadStableVersionNumber(wrkDir, versionstream.KindChart, platform.JenkinsXPlatformChart)
	if err != nil {
		return version, errors.Wrapf(err, "failed to load version of chart %s in dir %s", platform.JenkinsXPlatformChart, wrkDir)
	}
	return version, nil
}

// clones the jenkins-x cloud-environments repo to a local working dir
func (options *InstallOptions) CloneJXCloudEnvironmentsRepo() (string, error) {
	surveyOpts := survey.WithStdio(options.In, options.Out, options.Err)
	configDir, err := util.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("error determining config dir %v", err)
	}
	wrkDir := filepath.Join(configDir, "cloud-environments")

	log.Logger().Debugf("Current configuration dir: %s", configDir)
	log.Logger().Debugf("options.Flags.CloudEnvRepository: %s", options.Flags.CloudEnvRepository)
	log.Logger().Debugf("options.Flags.LocalCloudEnvironment: %t", options.Flags.LocalCloudEnvironment)

	if options.Flags.LocalCloudEnvironment {
		currentDir, err := os.Getwd()
		if err != nil {
			return wrkDir, fmt.Errorf("error getting current working directory %v", err)
		}
		log.Logger().Infof("Copying local dir %s to %s", currentDir, wrkDir)

		return wrkDir, util.CopyDir(currentDir, wrkDir, true)
	}
	if options.Flags.CloudEnvRepository == "" {
		options.Flags.CloudEnvRepository = opts.DefaultCloudEnvironmentsURL
	}
	log.Logger().Debugf("Cloning the Jenkins X cloud environments repo to %s", wrkDir)
	_, err = git.PlainClone(wrkDir, false, &git.CloneOptions{
		URL:           options.Flags.CloudEnvRepository,
		ReferenceName: "refs/heads/master",
		SingleBranch:  true,
		Progress:      options.Out,
	})
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			flag := false
			if options.BatchMode {
				flag = true
			} else if options.AdvancedMode {
				confirm := &survey.Confirm{
					Message: "A local Jenkins X cloud environments repository already exists, recreate with latest?",
					Default: true,
				}
				err := survey.AskOne(confirm, &flag, nil, surveyOpts)
				if err != nil {
					return wrkDir, err
				}
			} else {
				flag = true
				log.Logger().Infof(util.QuestionAnswer("A local Jenkins X cloud environments repository already exists, recreating with latest", util.YesNo(flag)))
			}

			if flag {
				err := os.RemoveAll(wrkDir)
				if err != nil {
					return wrkDir, err
				}

				return options.CloneJXCloudEnvironmentsRepo()
			}
		} else {
			return wrkDir, err
		}
	}
	return wrkDir, nil
}

func (options *InstallOptions) waitForInstallToBeReady(ns string) error {
	client, err := options.KubeClient()
	if err != nil {
		return err
	}

	log.Logger().Warnf("waiting for install to be ready, if this is the first time then it will take a while to download images")

	return kube.WaitForAllDeploymentsToBeReady(client, ns, 30*time.Minute)

}

func (options *InstallOptions) saveChartmuseumAuthConfig() error {

	authConfigSvc, err := options.ChartmuseumAuthConfigService()
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	var server *auth.AuthServer
	if options.ServerFlags.IsEmpty() {
		url := ""
		url, err = options.FindService(kube.ServiceChartMuseum)
		if err != nil {
			log.Logger().Warnf("No service called %s could be found so couldn't wire up the local auth file to talk to chart museum", kube.ServiceChartMuseum)
			return nil
		}
		server = config.GetOrCreateServer(url)
	} else {
		server, err = options.FindServer(config, &options.ServerFlags, "ChartMuseum server", "Try installing one via: jx create team", false)
		if err != nil {
			return err
		}
	}

	user := &auth.UserAuth{
		Username: options.AdminSecretsService.Flags.DefaultAdminUsername,
		Password: options.AdminSecretsService.Flags.DefaultAdminPassword,
	}

	server.Users = append(server.Users, user)

	config.CurrentServer = server.URL
	return authConfigSvc.SaveConfig()
}

func (options *InstallOptions) installAddon(name string) error {
	log.Logger().Infof("Installing addon %s", util.ColorInfo(name))

	opts := &CreateAddonOptions{
		CreateOptions: createoptions.CreateOptions{
			CommonOptions: options.CommonOptions,
		},
		HelmUpdate: true,
	}
	if name == "gitea" {
		opts.ReleaseName = defaultGiteaReleaseName
		giteaOptions := &CreateAddonGiteaOptions{
			CreateAddonOptions: *opts,
			Chart:              kube.ChartGitea,
		}
		return giteaOptions.Run()
	}
	return opts.CreateAddon(name)
}

func (options *InstallOptions) addGitServersToJenkinsConfig(helmConfig *config.HelmValuesConfig) error {
	gitAuthCfg, err := options.GitAuthConfigService()
	if err != nil {
		return errors.Wrap(err, "failed to create the git auth config service")
	}
	cfg := gitAuthCfg.Config()
	for _, server := range cfg.Servers {
		if server.Kind == "github" {
			u := server.URL
			if !gits.IsGitHubServerURL(u) {
				sc := config.JenkinsGithubServersValuesConfig{
					Name: server.Name,
					Url:  gits.GitHubEnterpriseApiEndpointURL(u),
				}
				helmConfig.Jenkins.Servers.GHE = append(helmConfig.Jenkins.Servers.GHE, sc)
			}
		}
	}
	return nil
}

func (options *InstallOptions) ensureDefaultStorageClass(client kubernetes.Interface, name string, provisioner string, typeName string) error {
	storageClassInterface := client.StorageV1().StorageClasses()
	storageClasses, err := storageClassInterface.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	var foundSc *storagev1.StorageClass
	for idx, sc := range storageClasses.Items {
		ann := sc.Annotations
		if ann != nil && ann[kube.AnnotationIsDefaultStorageClass] == "true" {
			return nil
		}
		if sc.Name == name {
			foundSc = &storageClasses.Items[idx]
		}
	}

	if foundSc != nil {
		// lets update the storageclass to be default
		if foundSc.Annotations == nil {
			foundSc.Annotations = map[string]string{}
		}
		foundSc.Annotations[kube.AnnotationIsDefaultStorageClass] = "true"

		log.Logger().Infof("Updating storageclass %s to be the default", util.ColorInfo(name))
		_, err = storageClassInterface.Update(foundSc)
		return err
	}

	// lets create a default storage class
	reclaimPolicy := core_v1.PersistentVolumeReclaimRetain

	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				kube.AnnotationIsDefaultStorageClass: "true",
			},
		},
		Provisioner: provisioner,
		Parameters: map[string]string{
			"type": typeName,
		},
		ReclaimPolicy: &reclaimPolicy,
		MountOptions:  []string{"debug"},
	}
	log.Logger().Infof("Creating default storageclass %s with provisioner %s", util.ColorInfo(name), util.ColorInfo(provisioner))
	_, err = storageClassInterface.Create(sc)
	return err
}

func (options *InstallOptions) changeDefaultStorageClass(client kubernetes.Interface, defaultName string) error {
	storageClassInterface := client.StorageV1().StorageClasses()
	storageClasses, err := storageClassInterface.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	var foundSc *storagev1.StorageClass
	for idx, sc := range storageClasses.Items {
		ann := sc.Annotations
		foundSc = &storageClasses.Items[idx]
		if sc.Name == defaultName {
			if ann == nil {
				foundSc.Annotations = map[string]string{}
			}
			foundSc.Annotations[kube.AnnotationIsDefaultStorageClass] = "true"
			_, err = storageClassInterface.Update(foundSc)
		} else {
			if ann != nil && ann[kube.AnnotationIsDefaultStorageClass] == "true" {
				foundSc.Annotations[kube.AnnotationIsDefaultStorageClass] = "false"
				_, err = storageClassInterface.Update(foundSc)
			}
		}
	}
	return nil
}

// returns the docker registry string for the given provider
func (options *InstallOptions) dockerRegistryValue() (string, error) {
	if options.Flags.DockerRegistry != "" {
		return options.Flags.DockerRegistry, nil
	}
	if options.Flags.Provider == cloud.AWS || options.Flags.Provider == cloud.EKS {
		return amazon.GetContainerRegistryHost()
	}
	if options.Flags.Provider == cloud.OPENSHIFT {
		return "docker-registry.default.svc:5000", nil
	}
	if options.Flags.Provider == cloud.GKE {
		if options.Flags.Kaniko {
			return "gcr.io", nil
		}
	}

	log.Logger().Debugf("unable to determine the dockerRegistryValue - provider=%s, defaulting to in-cluster registry", options.Flags.Provider)

	return "", nil
}

func (options *InstallOptions) saveAsConfigMap(name string, config interface{}) (*core_v1.ConfigMap, error) {
	return options.ModifyConfigMap(name, func(cm *core_v1.ConfigMap) error {
		data := util.ToStringMapStringFromStruct(config)
		cm.Data = data
		return nil
	})
}

func (options *InstallOptions) configureTeamSettings() error {
	initOpts := &options.InitOptions
	callback := func(env *v1.Environment) error {
		if env.Spec.TeamSettings.KubeProvider == "" {
			env.Spec.TeamSettings.KubeProvider = options.Flags.Provider
			log.Logger().Debugf("Storing the kubernetes provider %s in the TeamSettings", env.Spec.TeamSettings.KubeProvider)
		}

		if initOpts.Flags.Helm3 {
			env.Spec.TeamSettings.HelmTemplate = false
			env.Spec.TeamSettings.HelmBinary = "helm3"
			log.Logger().Debugf("Enabling helm3 / non template mode in the TeamSettings")
		} else if initOpts.Flags.NoTiller {
			env.Spec.TeamSettings.HelmTemplate = true
			log.Logger().Debugf("Enabling helm template mode in the TeamSettings")
		}

		if options.Flags.DockerRegistryOrg != "" {
			env.Spec.TeamSettings.DockerRegistryOrg = options.Flags.DockerRegistryOrg
			log.Logger().Infof("Setting the docker registry organisation to %s in the TeamSettings", env.Spec.TeamSettings.DockerRegistryOrg)
		}

		if options.Flags.VersionsRepository != "" {
			env.Spec.TeamSettings.VersionStreamURL = options.Flags.VersionsRepository
		}

		if options.Flags.VersionsGitRef != "" {
			env.Spec.TeamSettings.VersionStreamRef = options.Flags.VersionsGitRef
		}
		return nil
	}
	err := options.ModifyDevEnvironment(callback)
	if err != nil {
		return errors.Wrap(err, "updating the team setttings in the dev environment")
	}
	return nil
}

// setValuesFileValue lazily creates the values.yaml file possibly in a new directory and ensures there is the key in the values with the given value
func (options *InstallOptions) setValuesFileValue(fileName string, key string, value interface{}) error {
	dir, _ := filepath.Split(fileName)
	err := os.MkdirAll(dir, util.DefaultWritePermissions)
	if err != nil {
		return err
	}
	answerMap := map[string]interface{}{}

	// lets load any previous values if they exist
	exists, err := util.FileExists(fileName)
	if err != nil {
		return err
	}
	if exists {
		answerMap, err = helm.LoadValuesFile(fileName)
		if err != nil {
			return err
		}
	}
	answerMap[key] = value
	answer := chartutil.Values(answerMap)
	text, err := answer.YAML()
	if err != nil {
		return errors.Wrap(err, "Failed to marshal the updated values YAML files back to YAML")
	}
	err = ioutil.WriteFile(fileName, []byte(text), util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "Failed to save updated helm values YAML file %s", fileName)
	}
	return nil
}

// validateClusterName checks for compliance of a user supplied
// cluster name against GKE's rules for these names.
func validateClusterName(clustername string) error {
	// Check for length greater than 27.
	if len(clustername) > maxGKEClusterNameLength {
		err := fmt.Errorf("cluster name %s is greater than the maximum %d characters", clustername, maxGKEClusterNameLength)
		return err
	}
	// Now we need only make sure that clustername is limited to
	// lowercase alphanumerics and dashes.
	if util.DisallowedLabelCharacters.MatchString(clustername) {
		err := fmt.Errorf("cluster name %v contains invalid characters. Permitted are lowercase alphanumerics and `-`", clustername)
		return err
	}
	return nil
}

func installConfigKey(key string) string {
	return fmt.Sprintf("install.%s", key)
}
