package syntax

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// StepSyntaxOptions contains the command line flags
type StepSyntaxOptions struct {
	step.StepOptions
}

// NewCmdStepSyntax Steps a command object for the "step" command
func NewCmdStepSyntax(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSyntaxOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "syntax",
		Short: "syntax [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepSyntaxValidate(commonOpts))
	cmd.AddCommand(NewCmdStepSyntaxSchema(commonOpts))
	cmd.AddCommand(NewCmdStepSyntaxEffective(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepSyntaxOptions) Run() error {
	return o.Cmd.Help()
}
