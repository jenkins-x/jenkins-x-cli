package post

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"

	"github.com/spf13/cobra"
)

// StepPostInstallOptions contains the command line flags
type StepPostInstallOptions struct {
	step.StepOptions

	EnvJobCredentials string

	Results StepPostInstallResults
}

// StepPostInstallResults contains the command outputs mostly for testing purposes
type StepPostInstallResults struct {
	GitProviders map[string]gits.GitProvider
}

var (
	stepPostInstallLong = templates.LongDesc(`
		This pipeline step ensures that all the necessary jobs are imported and the webhooks set up - e.g. for the current Environments.

		It is designed to work with GitOps based development environments where the permanent Environments like Staging and Production are defined in a git repository.
		This step is used to ensure that all the 'Environment' resources have their associated CI+CD jobs setup in Jenkins or Prow with the necessary webhooks in place.
`)

	stepPostInstallExample = templates.Examples(`
		jx step post install
`)
)

// NewCmdStepPostInstall creates the command object
func NewCmdStepPostInstall(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepPostInstallOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Runs any post install actions",
		Long:    stepPostInstallLong,
		Example: stepPostInstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.EnvJobCredentials, "env-job-credentials", "", "", "The Jenkins credentials used by the GitOps Job for this environment")
	return cmd
}

// Run implements this command
func (o *StepPostInstallOptions) Run() (err error) {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return errors.Wrap(err, "failed to create the API extensions client")
	}
	kube.RegisterAllCRDs(apisClient)
	if err != nil {
		return err
	}
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "cannot create the JX client")
	}

	envMap, names, err := kube.GetEnvironments(jxClient, ns)
	if err != nil {
		return errors.Wrapf(err, "cannot load Environments in namespace %s", ns)
	}

	teamSettings, err := o.TeamSettings()
	if err != nil {
		return errors.Wrapf(err, "cannot load the TeamSettings from dev namespace %s", ns)
	}
	branchPattern := teamSettings.BranchPatterns

	envDir, err := util.EnvironmentsDir()
	if err != nil {
		return errors.Wrapf(err, "cannot find the environments git clone local directory")
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return errors.Wrapf(err, "cannot create the git auth config service")
	}

	prow, err := o.IsProw()
	if err != nil {
		return errors.Wrapf(err, "cannot determine if the current team is using Prow")
	}

	errs := []error{}
	for _, name := range names {
		env := envMap[name]
		if env == nil || (env.Spec.Kind != v1.EnvironmentKindTypePermanent && env.Spec.Kind != v1.EnvironmentKindTypeDevelopment) {
			continue
		}
		//gitRef := env.Spec.Source.GitRef
		gitURL := env.Spec.Source.URL
		if gitURL == "" {
			continue
		}
		gitInfo, err := gits.ParseGitURL(gitURL)
		if err != nil {
			log.Logger().Errorf("failed to parse git URL %s for Environment %s due to: %s", gitURL, name, err)
			errs = append(errs, errors.Wrapf(err, "failed to parse git URL %s for Environment %s", gitURL, name))
			continue
		}

		gitProvider, err := o.GitProviderForURL(gitURL, fmt.Sprintf("Environment %s", name))
		if err != nil {
			log.Logger().Errorf("failed to create git provider for Environment %s with git URL %s due to: %s", name, gitURL, err)
			errs = append(errs, errors.Wrapf(err, "failed to create git provider for Environment %s with git URL %s", name, gitURL))
			continue
		}
		if o.Results.GitProviders == nil {
			o.Results.GitProviders = map[string]gits.GitProvider{}
		}
		o.Results.GitProviders[name] = gitProvider

		if prow {
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
				return fmt.Errorf("could not find a username for git server %s", u)
			}
			err = authConfigSvc.SaveConfig()
			if err != nil {
				return err
			}
			// register the webhook
			return o.CreateWebhookProw(gitURL, gitProvider)
		}

		err = o.ImportProject(gitURL, envDir, jenkinsfile.Name, branchPattern, o.EnvJobCredentials, false, gitProvider, authConfigSvc, true, o.BatchMode)
		if err != nil {
			log.Logger().Errorf("failed to import Environment %s with git URL %s due to: %s", name, gitURL, err)
			errs = append(errs, errors.Wrapf(err, "failed to import Environment %s with git URL %s", name, gitURL))
		}
	}
	return util.CombineErrors(errs...)
}
