package scheduler

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

// StepSchedulerOptions contains the command line flags
type StepSchedulerOptions struct {
	step.StepOptions
}

var (
	stepSchedulerLong = templates.LongDesc(`
		This pipeline step command allows you to work with the scheduler. Sub commands include:

		* jx step scheduler config apply
		* jx step scheduler config generate
		* jx step scheduler config create pr
`)
)

// NewCmdStepScheduler Steps a command object for the "step" command
func NewCmdStepScheduler(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSchedulerOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "scheduler [command]",
		Long:  stepSchedulerLong,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepSchedulerConfig(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepSchedulerOptions) Run() error {
	return o.Cmd.Help()
}
