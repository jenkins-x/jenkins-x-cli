package scheduler

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/pipelinescheduler"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepSchedulerConfigApplyOptions contains the command line flags
type StepSchedulerConfigApplyOptions struct {
	step.StepOptions
	Agent         string
	ApplyDirectly bool

	// Used for testing
	CloneDir string
}

var (
	stepSchedulerConfigApplyLong = templates.LongDesc(`
        This command will transform your pipeline schedulers in to prow config. 
        If you are using gitops the prow config will be added to your environment repository. 
        For non-gitops environments the prow config maps will applied to your dev environment.
`)
	stepSchedulerConfigApplyExample = templates.Examples(`
	
	jx step scheduler config apply
`)
)

// NewCmdStepSchedulerConfigApply Steps a command object for the "step" command
func NewCmdStepSchedulerConfigApply(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSchedulerConfigApplyOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "scheduler config apply",
		Long:    stepSchedulerConfigApplyLong,
		Example: stepSchedulerConfigApplyExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Agent, "agent", "", "prow", "The scheduler agent to use e.g. Prow")
	cmd.Flags().BoolVarP(&options.ApplyDirectly, "direct", "", false, "Skip generating a PR and apply the pipeline config directly to the cluster when using gitops mode.")
	return cmd
}

// Run implements this command
func (o *StepSchedulerConfigApplyOptions) Run() error {
	gitOps, devEnv := o.GetDevEnv()
	switch o.Agent {
	case "prow":
		jxClient, ns, err := o.JXClientAndDevNamespace()
		if err != nil {
			return errors.WithStack(err)
		}
		teamSettings, err := o.TeamSettings()
		if err != nil {
			return err
		}
		if teamSettings == nil {
			return fmt.Errorf("no TeamSettings for namespace %s", ns)
		}
		cfg, plugs, err := pipelinescheduler.GenerateProw(gitOps, true, jxClient, ns, teamSettings.DefaultScheduler.Name, devEnv, nil)
		if err != nil {
			return errors.Wrapf(err, "generating Prow config")
		}
		kubeClient, ns, err := o.KubeClientAndNamespace()
		if err != nil {
			return errors.WithStack(err)
		}

		if gitOps && !o.ApplyDirectly {
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
			err = opts.AddToEnvironmentRepo(cfg, plugs, kubeClient, ns)
			if err != nil {
				return errors.Wrapf(err, "adding Prow config to environment repo")
			}
		} else {
			err = pipelinescheduler.ApplyDirectly(kubeClient, ns, cfg, plugs)
			if err != nil {
				return errors.Wrapf(err, "applying Prow config")
			}
		}
	default:
		return errors.Errorf("%s is an unsupported agent. Available agents are: prow", o.Agent)
	}
	return nil
}
