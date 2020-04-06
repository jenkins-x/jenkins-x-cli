package scheduler

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

// StepSchedulerConfigOptions contains the command line flags
type StepSchedulerConfigOptions struct {
	step.StepOptions
}

var (
	stepSchedulerConfigLong = templates.LongDesc(`
		This pipeline step command allows you to work with the scheduler configuration. Sub commands include:

		* jx step scheduler config apply
`)
)

// NewCmdStepSchedulerConfig Steps a command object for the "step" command
func NewCmdStepSchedulerConfig(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSchedulerConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "config",
		Short: "scheduler config [command]",
		Long:  stepSchedulerConfigLong,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepSchedulerConfigApply(commonOpts))
	cmd.AddCommand(NewCmdStepSchedulerConfigMigrate(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepSchedulerConfigOptions) Run() error {
	return o.Cmd.Help()
}
