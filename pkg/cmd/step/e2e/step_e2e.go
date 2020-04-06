package e2e

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// StepE2EOptions contains the command line flags
type StepE2EOptions struct {
	step.StepOptions
}

// NewCmdStepE2E Steps a command object for the "e2e" command
func NewCmdStepE2E(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepE2EOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "e2e [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepE2ELabel(commonOpts))
	cmd.AddCommand(NewCmdStepE2EGC(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepE2EOptions) Run() error {
	return o.Cmd.Help()
}
