package env

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// StepEnvOptions contains the command line flags
type StepEnvOptions struct {
	step.StepOptions
}

// NewCmdStepEnv Steps a command object for the "step" command
func NewCmdStepEnv(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepEnvOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "env",
		Short: "env [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepEnvApply(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepEnvOptions) Run() error {
	return o.Cmd.Help()
}
