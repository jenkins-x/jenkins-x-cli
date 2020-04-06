package create

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/kube/serviceaccount"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"fmt"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/jenkins-x/jx/pkg/util"
)

const (
	optionPullSecrets = "pull-secrets"
)

var (
	env_description = `		
	An Environment maps to a Kubernetes cluster and namespace and is a place that your team's applications can be promoted to via Continuous Delivery.

	You can optionally use GitOps to manage the configuration of an Environment by storing all configuration in a Git repository and then only changing it via Pull Requests and CI/CD.

	For more documentation on Environments see: [https://jenkins-x.io/about/features/#environments](https://jenkins-x.io/about/features/#environments)
	`
	create_env_long = templates.LongDesc(`
		Creates a new Environment
        ` + env_description + `
`)

	create_env_example = templates.Examples(`
		# Create a new Environment, prompting for the required data
		jx create env

		# Creates a new Environment passing in the required data on the command line
		jx create env -n prod -l Production --no-gitops --namespace my-prod
	`)
)

// CreateEnvOptions the options for the create env command
type CreateEnvOptions struct {
	options.CreateOptions

	Options                v1.Environment
	HelmValuesConfig       config.HelmValuesConfig
	PromotionStrategy      string
	NoGitOps               bool
	NoDevNamespaceInit     bool
	Prow                   bool
	GitOpsMode             bool
	ForkEnvironmentGitRepo string
	EnvJobCredentials      string
	GitRepositoryOptions   gits.GitRepositoryOptions
	Prefix                 string
	BranchPattern          string
	Vault                  bool
	PullSecrets            string
	Update                 bool
}

// NewCmdCreateEnv creates a command object for the "create" command
func NewCmdCreateEnv(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateEnvOptions{
		HelmValuesConfig: config.HelmValuesConfig{
			ExposeController: &config.ExposeController{},
		},
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "environment",
		Short:   "Create a new Environment which is used to promote your Team's Applications via Continuous Delivery",
		Aliases: []string{"env"},
		Long:    create_env_long,
		Example: create_env_example,
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
	cmd.Flags().BoolVarP(&options.Update, "update", "u", false, "Update environment if already exists")

	cmd.Flags().StringVarP(&options.Options.Spec.Namespace, kube.OptionNamespace, "s", "", "The Kubernetes namespace for the Environment")
	cmd.Flags().StringVarP(&options.Options.Spec.Cluster, "cluster", "c", "", "The Kubernetes cluster for the Environment. If blank and a namespace is specified assumes the current cluster")
	cmd.Flags().BoolVarP(&options.Options.Spec.RemoteCluster, "remote", "", false, "Indicates the Environment resides in a separate cluster to the development cluster. If this is true then we don't perform release piplines in this git repository but we use the Environment Controller inside that cluster: https://jenkins-x.io/getting-started/multi-cluster/")
	cmd.Flags().StringVarP(&options.Options.Spec.Source.URL, "git-url", "g", "", "The Git clone URL for the source code for GitOps based Environments")
	cmd.Flags().StringVarP(&options.Options.Spec.Source.Ref, "git-ref", "r", "", "The Git repo reference for the source code for GitOps based Environments")
	cmd.Flags().StringVarP(&options.GitRepositoryOptions.Owner, "git-owner", "", "", "Git organisation / owner")
	cmd.Flags().Int32VarP(&options.Options.Spec.Order, "order", "o", 100, "The order weighting of the Environment so that they can be sorted by this order before name")
	cmd.Flags().StringVarP(&options.Prefix, "prefix", "", "jx", "Environment repo prefix, your Git repo will be of the form 'environment-$prefix-$envName'")

	cmd.Flags().StringVarP(&options.PromotionStrategy, "promotion", "p", "", "The promotion strategy")
	cmd.Flags().StringVarP(&options.ForkEnvironmentGitRepo, "fork-git-repo", "f", kube.DefaultEnvironmentGitRepoURL, "The Git repository used as the fork when creating new Environment Git repos")
	cmd.Flags().StringVarP(&options.EnvJobCredentials, "env-job-credentials", "", "", "The Jenkins credentials used by the GitOps Job for this environment")
	cmd.Flags().StringVarP(&options.BranchPattern, "branches", "", "", "The branch pattern for branches to trigger CI/CD pipelines on the environment Git repository")

	cmd.Flags().BoolVarP(&options.NoGitOps, "no-gitops", "x", false, "Disables the use of GitOps on the environment so that promotion is implemented by directly modifying the resources via helm instead of using a Git repository")
	cmd.Flags().BoolVarP(&options.Prow, "prow", "", false, "Install and use Prow for environment promotion")
	cmd.Flags().BoolVarP(&options.Vault, "vault", "", false, "Sets up a Hashicorp Vault for storing secrets during the cluster creation")
	cmd.Flags().StringVarP(&options.PullSecrets, optionPullSecrets, "", "", "A list of Kubernetes secret names that will be attached to the service account (e.g. foo, bar, baz)")

	opts.AddGitRepoOptionsArguments(cmd, &options.GitRepositoryOptions)
	options.HelmValuesConfig.AddExposeControllerValues(cmd, false)

	return cmd
}

// Run implements the command
func (o *CreateEnvOptions) Run() error {
	args := o.Args
	if len(args) > 0 && o.Options.Name == "" {
		o.Options.Name = args[0]
	}
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	envDir, err := util.EnvironmentsDir()
	if err != nil {
		return err
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}

	prowFlag := o.Prow
	var devEnv *v1.Environment
	if o.GitOpsMode {
		err = o.ModifyDevEnvironment(func(env *v1.Environment) error {
			devEnv = env
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		devEnv, err = kube.EnsureDevEnvironmentSetup(jxClient, ns)
		if err != nil {
			return err
		}

		prowFlag, err = o.IsProw()
		if err != nil {
			return err
		}
		if prowFlag && !o.Prow {
			o.Prow = true
		}
	}

	if o.Prow {
		// lets make sure we have the prow enabled
		err = o.ModifyDevEnvironment(func(env *v1.Environment) error {
			env.Spec.TeamSettings.PromotionEngine = v1.PromotionEngineProw
			return nil
		})
		if err != nil {
			return err
		}
	}

	env := v1.Environment{}
	o.Options.Spec.PromotionStrategy = v1.PromotionStrategyType(o.PromotionStrategy)
	gitProvider, err := kube.CreateEnvironmentSurvey(o.BatchMode, authConfigSvc, devEnv, &env, &o.Options, o.Update, o.ForkEnvironmentGitRepo, ns,
		jxClient, kubeClient, envDir, &o.GitRepositoryOptions, o.HelmValuesConfig, o.Prefix, o.Git(), o.ResolveChartMuseumURL, o.GetIOFileHandles())
	if err != nil {
		return err
	}

	err = o.ModifyEnvironment(env.Name, func(env2 *v1.Environment) error {
		env2.Name = env.Name
		env2.Spec = env.Spec

		// lets copy across any labels or annotations
		if env2.Annotations == nil {
			env2.Annotations = map[string]string{}
		}
		if env2.Labels == nil {
			env2.Labels = map[string]string{}
		}
		for k, v := range env.Annotations {
			env2.Annotations[k] = v
		}
		for k, v := range env.Labels {
			env2.Labels[k] = v
		}
		return nil
	})
	log.Logger().Infof("Created environment %s", util.ColorInfo(env.Name))

	if !o.GitOpsMode {
		err = kube.EnsureEnvironmentNamespaceSetup(kubeClient, jxClient, &env, ns)
		if err != nil {
			return err
		}
	}

	/* It is important this pull secret handling goes after any namespace creation code; the service account exists in the created namespace */
	if o.PullSecrets != "" {
		// We need the namespace to be created first - do the check
		if !o.GitOpsMode {
			err = kube.EnsureEnvironmentNamespaceSetup(kubeClient, jxClient, &env, env.Spec.Namespace)
			if err != nil {
				// This can happen if, for whatever reason, the namespace takes a while to create. That shouldn't stop the entire process though
				log.Logger().Warnf("Namespace %s does not exist for jx to patch the service account for, you should patch the service account manually with your pull secret(s) ", env.Spec.Namespace)
			}
		}
		imagePullSecrets := strings.Fields(o.PullSecrets)
		saName := "default"
		//log.Logger().Infof("Patching the secrets %s for the service account %s\n", imagePullSecrets, saName)
		err = serviceaccount.PatchImagePullSecrets(kubeClient, env.Spec.Namespace, saName, imagePullSecrets)
		if err != nil {
			return fmt.Errorf("Failed to add pull secrets %s to service account %s in namespace %s: %v", imagePullSecrets, saName, env.Spec.Namespace, err)
		} else {
			log.Logger().Infof("Service account \"%s\" in namespace \"%s\" configured to use pull secret(s) %s ", saName, env.Spec.Namespace, imagePullSecrets)
			log.Logger().Infof("Pull secret(s) must exist in namespace %s before deploying your applications in this environment ", env.Spec.Namespace)
		}
	}

	// Skip the environment registration if gitops mode is active
	if o.GitOpsMode {
		return nil
	}

	err = o.RegisterEnvironment(&env, gitProvider, authConfigSvc)
	if err != nil {
		errors.Wrapf(err, "registering the environment %s/%s", env.GetNamespace(), env.GetName())
	}

	return nil
}

// RegisterEnvironment performs the environment registration
func (o *CreateEnvOptions) RegisterEnvironment(env *v1.Environment, gitProvider gits.GitProvider, authConfigSvc auth.ConfigService) error {
	gitURL := env.Spec.Source.URL
	if gitURL == "" {
		log.Logger().Warnf("environment %s does not have a git source URL", env.Name)
		return nil
	}
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrap(err, "parsing git repository URL from environment source")
	}

	if gitURL == "" {
		return nil
	}

	envDir, err := util.EnvironmentsDir()
	if err != nil {
		return errors.Wrap(err, "getting environments directory")
	}

	if gitProvider == nil {
		authConfigSvc, err = o.GitAuthConfigService()
		if err != nil {
			return err
		}
		gitKind, err := o.GitServerKind(gitInfo)
		if err != nil {
			return err
		}
		message := "user name to create the Git repository"
		commonOpts := o.CreateOptions.CommonOptions
		ghOwner, err := o.GetGitHubAppOwner(gitInfo)
		if err != nil {
			return err
		}
		p, err := commonOpts.NewGitProvider(gitURL, message, authConfigSvc, gitKind, ghOwner, o.BatchMode, o.Git())
		if err != nil {
			return err
		}
		gitProvider = p
	}

	if o.Prow {
		repo := fmt.Sprintf("%s/%s", gitInfo.Organisation, gitInfo.Name)

		kubeClient, devNs, err := o.KubeClientAndDevNamespace()
		if err != nil {
			return err
		}

		devEnv, teamSettings, err := o.DevEnvAndTeamSettings()
		if err != nil {
			return err
		}
		if teamSettings.IsSchedulerMode() {
			jxClient, _, err := o.JXClient()
			if err != nil {
				return err
			}
			sr, err := kube.GetOrCreateSourceRepository(jxClient, devNs, gitInfo.Name, gitInfo.Organisation, gitInfo.HostURLWithoutUser())
			log.Logger().Debugf("have SourceRepository: %s\n", sr.Name)

			err = o.GenerateProwConfig(devNs, devEnv)
			if err != nil {
				return err
			}
		} else {
			err = prow.AddEnvironment(kubeClient, []string{repo}, devNs, env.Spec.Namespace, teamSettings, env.Spec.RemoteCluster)
			if err != nil {
				return fmt.Errorf("failed to add repo %s to Prow config in namespace %s: %v", repo, env.Spec.Namespace, err)
			}
		}

		config := authConfigSvc.Config()
		u := gitInfo.HostURL()
		server := config.GetOrCreateServer(u)
		if len(server.Users) == 0 {
			// lets check if the host was used in `~/.jx/gitAuth.yaml` instead of URL
			s2 := config.GetOrCreateServer(gitInfo.Host)
			if s2 != nil && len(s2.Users) > 0 {
				server = s2
				u = gitInfo.Host
			}
		}
		user, err := o.PickPipelineUserAuth(config, server)
		if err != nil {
			return err
		}
		if user.Username == "" {
			return fmt.Errorf("Could not find a username for git server %s", u)
		}
		err = authConfigSvc.SaveConfig()
		if err != nil {
			return err
		}
		return o.CreateWebhookProw(gitURL, gitProvider)
	}

	return o.ImportProject(gitURL, envDir, jenkinsfile.Name, o.BranchPattern, o.EnvJobCredentials, false, gitProvider, authConfigSvc, true, o.BatchMode)
}
