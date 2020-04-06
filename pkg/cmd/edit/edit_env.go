package edit

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	editEnvLong = templates.LongDesc(`
    Edits a new Environment

	An Environment maps to a Kubernetes cluster and namespace and is a place that your team's applications can be promoted to via Continuous Delivery.

	You can optionally use GitOps to manage the configuration of an Environment by storing all configuration in a Git repository and then only changing it via Pull Requests and CI/CD.

	For more documentation on Environments see: [https://jenkins-x.io/about/features/#environments](https://jenkins-x.io/about/features/#environments)
`)

	editEnvExample = templates.Examples(`
		# Edit the stating Environment, prompting for the required data
		jx edit env -n stating

		# Edit the prod Environment in batch mode (so not interactive)
		jx edit env -b -n prod -l Production --no-gitops --namespace my-prod
	`)
)

// EditEnvOptions the options for the create spring command
type EditEnvOptions struct {
	*opts.CommonOptions

	Options                v1.Environment
	HelmValuesConfig       config.HelmValuesConfig
	PromotionStrategy      string
	NoGitOps               bool
	ForkEnvironmentGitRepo string
	EnvJobCredentials      string
	GitRepositoryOptions   gits.GitRepositoryOptions
	Prefix                 string
	BranchPattern          string
}

// NewCmdEditEnv creates a command object for the "create" command
func NewCmdEditEnv(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &EditEnvOptions{
		HelmValuesConfig: config.HelmValuesConfig{
			ExposeController: &config.ExposeController{},
		},
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "environment",
		Short:   "Edits an Environment which is used to promote your Team's Applications via Continuous Delivery",
		Aliases: []string{"env"},
		Long:    editEnvLong,
		Example: editEnvExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	//addCreateAppFlags(cmd, &options.CreateOptions)

	cmd.Flags().StringVarP(&options.Options.Name, kube.OptionName, "n", "", "The Environment resource name. Must follow the Kubernetes name conventions like Services, Namespaces")
	cmd.Flags().StringVarP(&options.Options.Spec.Label, "label", "l", "", "The Environment label which is a descriptive string like 'Production' or 'Staging'")
	cmd.Flags().StringVarP(&options.Options.Spec.Namespace, kube.OptionNamespace, "s", "", "The Kubernetes namespace for the Environment")
	cmd.Flags().StringVarP(&options.Options.Spec.Cluster, "cluster", "c", "", "The Kubernetes cluster for the Environment. If blank and a namespace is specified assumes the current cluster")
	cmd.Flags().BoolVarP(&options.Options.Spec.RemoteCluster, "remote", "", false, "Indicates the Environment resides in a separate cluster to the development cluster. If this is true then we don't perform release piplines in this git repository but we use the Environment Controller inside that cluster: https://jenkins-x.io/getting-started/multi-cluster/")
	cmd.Flags().StringVarP(&options.Options.Spec.Source.URL, "git-url", "g", "", "The Git clone URL for the source code for GitOps based Environments")
	cmd.Flags().StringVarP(&options.Options.Spec.Source.Ref, "git-ref", "r", "", "The Git repo reference for the source code for GitOps based Environments")
	cmd.Flags().Int32VarP(&options.Options.Spec.Order, "order", "o", 100, "The order weighting of the Environment so that they can be sorted by this order before name")
	cmd.Flags().StringVarP(&options.Prefix, "prefix", "", "jx", "Environment repo prefix, your Git repo will be of the form 'environment-$prefix-$envName'")

	cmd.Flags().StringVarP(&options.PromotionStrategy, "promotion", "p", "", "The promotion strategy")
	cmd.Flags().StringVarP(&options.ForkEnvironmentGitRepo, "fork-git-repo", "f", kube.DefaultEnvironmentGitRepoURL, "The Git repository used as the fork when creating new Environment Git repos")
	cmd.Flags().StringVarP(&options.EnvJobCredentials, "env-job-credentials", "", "", "The Jenkins credentials used by the GitOps Job for this environment")
	cmd.Flags().StringVarP(&options.BranchPattern, "branches", "", "", "The branch pattern for branches to trigger CI/CD pipelines on the environment Git repository")

	cmd.Flags().BoolVarP(&options.NoGitOps, "no-gitops", "x", false, "Disables the use of GitOps on the environment so that promotion is implemented by directly modifying the resources via Helm instead of using a Git repository")

	opts.AddGitRepoOptionsArguments(cmd, &options.GitRepositoryOptions)
	options.HelmValuesConfig.AddExposeControllerValues(cmd, false)
	return cmd
}

// Run implements the command
func (o *EditEnvOptions) Run() error {
	jxClient, currentNs, err := o.JXClient()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	kube.RegisterEnvironmentCRD(apisClient)

	ns, currentEnv, err := kube.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return err
	}
	envDir, err := util.EnvironmentsDir()
	if err != nil {
		return err
	}

	envNames, err := kube.GetEnvironmentNames(jxClient, ns)
	if err != nil {
		return err
	}
	name := ""
	args := o.Args
	if len(args) > 0 {
		name = args[0]
	} else {
		name = o.Options.Name
		if name == "" {
			name, err = kube.PickEnvironment(envNames, currentEnv, o.GetIOFileHandles())
			if err != nil {
				return err
			}
		}
	}

	env, err := jxClient.JenkinsV1().Environments(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return util.InvalidArg(name, envNames)
	}

	devEnv, err := kube.EnsureDevEnvironmentSetup(jxClient, ns)
	if err != nil {
		return err
	}
	o.Options.Spec.PromotionStrategy = v1.PromotionStrategyType(o.PromotionStrategy)
	gitProvider, err := kube.CreateEnvironmentSurvey(o.BatchMode, authConfigSvc, devEnv, env, &o.Options, true, o.ForkEnvironmentGitRepo,
		ns, jxClient, kubeClient, envDir, &o.GitRepositoryOptions, o.HelmValuesConfig, o.Prefix, o.Git(), o.ResolveChartMuseumURL, o.GetIOFileHandles())
	if err != nil {
		return err
	}
	_, err = jxClient.JenkinsV1().Environments(ns).PatchUpdate(env)
	if err != nil {
		return err
	}
	log.Logger().Infof("Updated environment %s", util.ColorInfo(env.Name))

	err = kube.EnsureEnvironmentNamespaceSetup(kubeClient, jxClient, env, ns)
	if err != nil {
		return err
	}
	gitURL := env.Spec.Source.URL
	if gitURL != "" {
		if gitProvider == nil {
			p, err := o.GitProviderForURL(gitURL, "user name to create the Git repository")
			if err != nil {
				return err
			}
			gitProvider = p
		}
		return o.ImportProject(gitURL, envDir, jenkinsfile.Name, o.BranchPattern, o.EnvJobCredentials, false, gitProvider, authConfigSvc, true, o.BatchMode)
	}
	return nil
}
