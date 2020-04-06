package scheduler

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/pipelinescheduler"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepSchedulerConfigMigrateOptions contains the command line flags
type StepSchedulerConfigMigrateOptions struct {
	step.StepOptions
	Agent                   string
	ProwConfigFileLocation  string
	ProwPluginsFileLocation string
	SkipVerification        bool
	DryRun                  bool

	// Used for testing
	CloneDir string
}

var (
	stepSchedulerConfigMigrateLong = templates.LongDesc(`
        This command will generate pipeline scheduler resources from either the prow config maps or prow config files.
        For gitops users they will be added to the dev environment git repository.
        For non gitops users they will be applied directly to the cluster if --dryRun=false.
`)
	stepSchedulerConfigMigrateExample = templates.Examples(`
	# Test the migration but do not apply
	jx step scheduler config migrate

    # Generate the pipeline schedulers and apply them either via gitops or directly to the cluster
    jx step scheduler config migrate --dryRun=false

    # Generate the pipeline schedulers from files instead of the existing configmaps in the cluster
    jx step scheduler config migrate --prow-config-file=config.yaml --prow-plugins-file=plugins.yaml

    # Disable validation checks when migrating to pipeline schedulers
    jx step scheduler config migrate --skipVerification=true
    
`)
)

// NewCmdStepSchedulerConfigMigrate Steps a command object for the "step" command
func NewCmdStepSchedulerConfigMigrate(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSchedulerConfigMigrateOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "migrate",
		Short:   "scheduler config migrate",
		Long:    stepSchedulerConfigMigrateLong,
		Example: stepSchedulerConfigMigrateExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Agent, "agent", "", "prow", "The scheduler agent to use e.g. Prow")
	cmd.Flags().StringVarP(&options.ProwConfigFileLocation, "prow-config-file", "", "", "The location of the config file to use")
	cmd.Flags().StringVarP(&options.ProwPluginsFileLocation, "prow-plugins-file", "", "", "The location of the plugins file to use")
	cmd.Flags().BoolVarP(&options.SkipVerification, "skipVerification", "", false, "Skip verification of the new configuration")
	cmd.Flags().BoolVarP(&options.DryRun, "dryRun", "", true, "Do not apply the generated configuration")
	return cmd
}

// Run implements this command
func (o *StepSchedulerConfigMigrateOptions) Run() error {
	gitOps, devEnv := o.GetDevEnv()
	jxClient, ns, err := o.JXClient()
	if err != nil {
		return errors.WithStack(err)
	}
	teamSettings, err := o.TeamSettings()
	if err != nil {
		return errors.WithStack(err)
	}
	switch o.Agent {
	case "prow":
		kubeClient, err := o.KubeClient()
		if err != nil {
			return errors.WithStack(err)
		}
		sourceRepoGroups, sourceRepos, schedulers, err := pipelinescheduler.CreateSchedulersFromProwConfig(o.ProwConfigFileLocation, o.ProwPluginsFileLocation, o.SkipVerification, o.DryRun, gitOps, jxClient, kubeClient, ns, teamSettings.DefaultScheduler.Name, devEnv)
		if err != nil {
			return errors.Wrapf(err, "generating Prow config")
		}
		if !o.DryRun {
			if gitOps {
				opts := pipelinescheduler.GitOpsOptions{
					Verbose: o.Verbose,
					DevEnv:  devEnv,
				}
				opts.PullRequestCloneDir = ""
				if o.CloneDir != "" {
					opts.PullRequestCloneDir = o.CloneDir
				}

				gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(devEnv.Spec.Source.URL)
				if err != nil {
					return errors.Wrapf(err, "creating git provider for %s", devEnv.Spec.Source.URL)
				}
				opts.GitProvider = gitProvider
				opts.Gitter = o.Git()
				opts.Helmer = o.Helm()
				err = opts.AddSchedulersToEnvironmentRepo(sourceRepoGroups, sourceRepos, schedulers)
				if err != nil {
					return errors.Wrapf(err, "adding pipeline scheduler config to environment repo")
				}
			} else {
				err = pipelinescheduler.ApplySchedulersDirectly(jxClient, ns, sourceRepoGroups, sourceRepos, schedulers, devEnv)
				if err != nil {
					return errors.Wrapf(err, "applying pipeline scheduler config")
				}
			}
		} else {
			log.Logger().Info("Running in dry run mode, pipeline scheduler config will be discarded")

		}
	default:
		return errors.Errorf("%s is an unsupported agent. Available agents are: prow", o.Agent)
	}
	return nil
}
